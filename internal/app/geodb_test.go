package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// delayedReader returns bounded chunks to exercise progress updates during a slow transfer.
type delayedReader struct {
	contents  []byte        // contents is the test response body.
	chunkSize int           // chunkSize bounds each simulated network read.
	delay     time.Duration // delay spaces reads to expose intermediate progress frames.
	offset    int           // offset is the next unread byte.
}

// Read returns one delayed chunk or EOF when all data has been consumed.
func (r *delayedReader) Read(buffer []byte) (int, error) {
	if r.offset == len(r.contents) {
		return 0, io.EOF
	}

	time.Sleep(r.delay)
	end := min(r.offset+r.chunkSize, len(r.contents))
	read := copy(buffer, r.contents[r.offset:end])
	r.offset += read

	return read, nil
}

// TestUpdateGeoDBUsesETagFallback verifies auth, daily throttling, and safe replacement.
func TestUpdateGeoDBUsesETagFallback(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 0, 0, 0, time.UTC)
	databasePath := filepath.Join(t.TempDir(), "GeoLite2.mmdb")
	oldDatabase := []byte("existing database")
	if err := os.WriteFile(databasePath, oldDatabase, 0o600); err != nil {
		t.Fatal(err)
	}

	statePath := databasePath + ".update.json"
	state := geoDBState{CheckedAt: now.Add(-48 * time.Hour), ETag: "\"v1\""}
	if err := writeGeoDBState(statePath, state); err != nil {
		t.Fatal(err)
	}

	var headRequests, getRequests int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "account" || password != "license" {
			t.Errorf("request auth = %q:%q, present=%t", username, password, ok)
		}

		switch r.Method {
		case http.MethodHead:
			headRequests++
			w.Header().Set("ETag", "\"v1\"")
			w.WriteHeader(http.StatusOK)

		case http.MethodGet:
			getRequests++
			_, _ = w.Write([]byte("not an MMDB file"))

		default:
			t.Errorf("unexpected HTTP method %q", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	defer server.Close()
	state.SourceKey = geoDBSourceKey(server.URL)
	if err := writeGeoDBState(statePath, state); err != nil {
		t.Fatal(err)
	}

	client := server.Client()
	client.Timeout = geoDBDownloadTimeout
	if err := updateGeoDBWithClientAndProgress(
		databasePath, server.URL, "account:license", now, client, geoDBProgressOptions{},
	); err != nil {
		t.Fatal(err)
	}
	if headRequests != 1 || getRequests != 0 {
		t.Fatalf("requests: HEAD=%d GET=%d, want HEAD=1 GET=0", headRequests, getRequests)
	}
	updatedState, err := readGeoDBState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !updatedState.CheckedAt.Equal(now) || updatedState.ETag != "\"v1\"" {
		t.Fatalf("saved update state = %#v", updatedState)
	}

	// A second invocation within a day must return before creating any HTTP request.
	if err := updateGeoDBWithClientAndProgress(
		databasePath, server.URL, "account:license", now.Add(time.Hour), client, geoDBProgressOptions{},
	); err != nil {
		t.Fatal(err)
	}
	if headRequests != 1 || getRequests != 0 {
		t.Fatalf("daily throttle made extra requests: HEAD=%d GET=%d", headRequests, getRequests)
	}

	// A changed ETag triggers a download, but invalid bytes must not replace the working file.
	state.CheckedAt = now.Add(-48 * time.Hour)
	invalidArchive := makeGeoDBArchive(t, []byte("not an MMDB file"))
	updatedServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			headRequests++
			w.Header().Set("ETag", "\"v2\"")
			return
		}
		getRequests++
		_, _ = w.Write(invalidArchive)
	}))
	defer updatedServer.Close()

	state.SourceKey = geoDBSourceKey(updatedServer.URL)
	if err := writeGeoDBState(statePath, state); err != nil {
		t.Fatal(err)
	}
	if err := updateGeoDBWithClientAndProgress(
		databasePath,
		updatedServer.URL,
		"account:license",
		now,
		updatedServer.Client(),
		geoDBProgressOptions{},
	); err == nil {
		t.Fatal("invalid downloaded database was accepted")
	}

	got, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(oldDatabase) {
		t.Fatalf("existing database changed to %q", got)
	}
}

// TestGeoDBProgressReaderTracksTransferredBytes covers progress updates from streamed input.
func TestGeoDBProgressReaderTracksTransferredBytes(t *testing.T) {
	contents := []byte("database response body")
	var output bytes.Buffer
	reader := &geoDBProgressReader{
		Reader: &delayedReader{contents: contents, chunkSize: 5, delay: 80 * time.Millisecond},
		progress: geoDBProgressOptions{
			output:         &output,
			terminal:       true,
			startedMessage: "Downloading GeoIP database",
		},
		total: int64(len(contents)),
	}

	written, err := io.Copy(io.Discard, reader)
	if err != nil {
		t.Fatalf("copy streamed response: %v", err)
	}
	if written != int64(len(contents)) {
		t.Fatalf("copied %d bytes, want %d", written, len(contents))
	}
	reader.finish(true)

	got := output.String()
	for _, expected := range []string{"Downloading GeoIP database", "done!", "%", "B"} {
		if !strings.Contains(got, expected) {
			t.Errorf("progress output %q does not contain %q", got, expected)
		}
	}
}

// TestGeoDBProgressReaderSkipsEmptyInput ensures the progress bar needs received bytes.
func TestGeoDBProgressReaderSkipsEmptyInput(t *testing.T) {
	var output bytes.Buffer
	reader := &geoDBProgressReader{
		Reader: strings.NewReader(""),
		progress: geoDBProgressOptions{
			output:           &output,
			startedMessage:   "Download started",
			completedMessage: "Download completed in %s",
		},
		total: 0,
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatalf("copy empty response: %v", err)
	}
	reader.finish(true)

	if got := output.String(); got != "" {
		t.Fatalf("empty response produced progress output %q", got)
	}
}

// TestGeoDBProgressReaderReportsPlainStatusWithoutTTY covers redirected stderr output.
func TestGeoDBProgressReaderReportsPlainStatusWithoutTTY(t *testing.T) {
	for _, test := range []struct {
		name            string
		success         bool
		completionStart string
	}{
		{name: "completed", success: true, completionStart: "Download completed in "},
		{name: "failed", completionStart: "Download failed after "},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			reader := &geoDBProgressReader{
				Reader: &delayedReader{
					contents:  []byte("database response"),
					chunkSize: 4,
					delay:     20 * time.Millisecond,
				},
				progress: geoDBProgressOptions{
					output:           &output,
					startedMessage:   "Download started",
					completedMessage: "Download completed in %s",
					failedMessage:    "Download failed after %s",
				},
			}
			if _, err := io.Copy(io.Discard, reader); err != nil {
				t.Fatalf("copy streamed response: %v", err)
			}
			reader.finish(test.success)

			lines := strings.Split(strings.TrimSpace(output.String()), "\n")
			if len(lines) != 2 || lines[0] != "Download started" ||
				!strings.HasPrefix(lines[1], test.completionStart) {
				t.Fatalf("plain progress output = %q, want start and result lines", output.String())
			}
			duration := strings.TrimPrefix(lines[1], test.completionStart)
			if _, err := time.ParseDuration(duration); err != nil {
				t.Fatalf("completion duration = %q, want a Go duration", duration)
			}
		})
	}
}

// makeGeoDBArchive builds a small MaxMind-like archive for extraction error tests.
func makeGeoDBArchive(t *testing.T, contents []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)
	if err := archive.WriteHeader(&tar.Header{
		Name:     "GeoLite2-City/GeoLite2-City.mmdb",
		Mode:     0o600,
		Size:     int64(len(contents)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// TestGeoDBOptionsRejectUnsafeValues keeps credentials out of URLs and enforces HTTPS.
func TestGeoDBOptionsRejectUnsafeValues(t *testing.T) {
	for _, source := range []string{
		"http://example.test/db.mmdb",
		"https://user:password@example.test/db.mmdb",
		"https://example.test/db.mmdb#fragment",
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := parseGeoDBURL(source); err == nil {
				t.Fatalf("parseGeoDBURL(%q) unexpectedly succeeded", source)
			}
		})
	}

	for _, test := range []struct {
		value string
		want  []string
	}{
		{value: "", want: []string{"", ""}},
		{value: "account:license", want: []string{"account", "license"}},
		{value: "account:"},
		{value: "missing-separator"},
	} {
		gotUser, gotPass, err := parseGeoDBAuth(test.value)
		if test.want == nil {
			if err == nil {
				t.Errorf("parseGeoDBAuth(%q) unexpectedly succeeded", test.value)
			}
			continue
		}
		if err != nil || gotUser != test.want[0] || gotPass != test.want[1] {
			t.Errorf("parseGeoDBAuth(%q) = %q:%q, %v", test.value, gotUser, gotPass, err)
		}
	}
}

// TestSameGeoDBVersionPrefersLastModifiedAndFallsBackToETag covers both validators.
func TestSameGeoDBVersionPrefersLastModifiedAndFallsBackToETag(t *testing.T) {
	state := geoDBState{LastModified: "Fri, 02 Jan 2026 02:00:00 GMT", ETag: "\"v1\""}
	tests := []struct {
		name     string
		modified string
		etag     string
		want     bool
	}{
		{name: "unchanged Last-Modified", modified: state.LastModified, etag: "\"v2\"", want: true},
		{name: "changed Last-Modified takes precedence", modified: "Sat, 03 Jan 2026 02:00:00 GMT", etag: state.ETag},
		{name: "ETag fallback", etag: state.ETag, want: true},
		{name: "changed ETag fallback", etag: "\"v2\""},
		{name: "missing validators"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sameGeoDBVersion(state, test.modified, test.etag); got != test.want {
				t.Fatalf("sameGeoDBVersion() = %t, want %t", got, test.want)
			}
		})
	}
}

// TestGeoDBStateJSONKeepsOnlyUpdateMetadata documents the stable sidecar format.
func TestGeoDBStateJSONKeepsOnlyUpdateMetadata(t *testing.T) {
	state := geoDBState{
		CheckedAt:    time.Date(2026, time.January, 2, 3, 0, 0, 0, time.UTC),
		LastModified: "Fri, 02 Jan 2026 02:00:00 GMT",
		ETag:         "\"release-1\"",
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "license") {
		t.Fatalf("state unexpectedly contains credentials: %s", data)
	}
}
