// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/oschwald/geoip2-golang/v2"
)

const (
	// geoDBCheckInterval limits update checks for each database path.
	geoDBCheckInterval = 24 * time.Hour
	// geoDBDownloadTimeout bounds the full update attempt, including redirects.
	geoDBDownloadTimeout = 2 * time.Minute
	// geoDBMaxSize protects disk space when a configured URL serves unexpected data.
	geoDBMaxSize = 256 << 20
)

// geoDBState records the last check and remote validators beside the database.
type geoDBState struct {
	// CheckedAt prevents startup from contacting the source more than daily.
	CheckedAt time.Time `json:"checked_at"`

	// LastModified is the source build time from the last accepted database.
	LastModified string `json:"last_modified,omitempty"`

	// ETag is used when the source does not provide Last-Modified.
	ETag string `json:"etag,omitempty"`

	// SourceKey binds validators and the check inter val to the configured endpoint.
	SourceKey string `json:"source_key"`
}

// geoDBProgressOptions configures terminal or plain-text download status output.
type geoDBProgressOptions struct {
	output           io.Writer // output receives progress details.
	startedMessage   string    // startedMessage is printed when the first response byte arrives.
	completedMessage string    // completedMessage formats the successful transfer duration.
	failedMessage    string    // failedMessage formats the failed transfer duration.
	terminal         bool      // terminal selects the animated progress bar.
}

// geoDBProgressReader tracks bytes received from a database download response.
type geoDBProgressReader struct {
	startedAt time.Time            // startedAt is set when the first response byte arrives.
	io.Reader                      // Reader supplies bytes from the HTTP response body.
	writer    progress.Writer      // writer renders updates after the first response byte.
	tracker   *progress.Tracker    // tracker records received response bytes.
	renderEnd chan struct{}        // renderEnd closes after the final progress frame.
	progress  geoDBProgressOptions // progress controls progress output.
	total     int64                // total is the expected response size, or zero when unknown.
}

// updateGeoDB checks and atomically installs a newer MMDB, preserving a usable file on errors.
func updateGeoDB(path, source, auth string, now time.Time, progress geoDBProgressOptions) error {
	return updateGeoDBWithClientAndProgress(
		path,
		source,
		auth,
		now,
		geoDBHTTPClient(),
		progress,
	)
}

// updateGeoDBWithClientAndProgress optionally reports bytes read from a real database download.
func updateGeoDBWithClientAndProgress(
	path, source, auth string,
	now time.Time,
	client *http.Client,
	progress geoDBProgressOptions,
) error {
	endpoint, err := parseGeoDBURL(source)
	if err != nil {
		return err
	}

	username, password, err := parseGeoDBAuth(auth)
	if err != nil {
		return err
	}

	statePath := path + ".update.json"
	state, stateErr := readGeoDBState(statePath)
	if stateErr != nil && !errors.Is(stateErr, os.ErrNotExist) {
		return stateErr
	}

	sourceKey := geoDBSourceKey(endpoint.String())
	if state.SourceKey != sourceKey {
		// Validators belong to one endpoint; never compare them after a URL change.
		state = geoDBState{SourceKey: sourceKey}
	}

	_, databaseErr := os.Stat(path)
	checkAge := now.Sub(state.CheckedAt)
	// The sidecar records successful or failed attempts
	// so a service outage does not cause every CLI invocation to repeat the same network request.
	if databaseErr == nil &&
		stateErr == nil &&
		state.SourceKey == sourceKey &&
		checkAge >= 0 &&
		checkAge < geoDBCheckInterval {
		return nil
	}

	request, err := http.NewRequest(http.MethodHead, endpoint.String(), nil)
	if err != nil {
		return appWrap("geo.error.create_check", "create update check", err)
	}

	setGeoDBAuth(request, username, password)
	response, err := client.Do(request)
	if err != nil {
		return markGeoDBCheck(statePath, state, now,
			appWrap("geo.error.check_remote", "check remote database", geoDBTransportCause(err)))
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return markGeoDBCheck(
			statePath, state, now,
			appError("geo.error.check_status", "check remote database: HTTP %s", response.Status))
	}

	remoteModified := response.Header.Get("Last-Modified")
	remoteETag := response.Header.Get("ETag")
	if _, err := os.Stat(path); err == nil && sameGeoDBVersion(state, remoteModified, remoteETag) {
		// Advance the local throttle after a confirmed unchanged response;
		// the DB file itself remains untouched so its timestamp continues to describe its contents.
		state.CheckedAt = now
		state.LastModified = remoteModified
		state.ETag = remoteETag
		state.SourceKey = sourceKey
		return writeGeoDBState(statePath, state)
	}

	if err := downloadGeoDB(
		client,
		endpoint.String(),
		username,
		password,
		path,
		progress,
	); err != nil {
		return markGeoDBCheck(statePath, state, now, err)
	}

	state.CheckedAt = now
	state.LastModified = remoteModified
	state.ETag = remoteETag
	state.SourceKey = sourceKey

	return writeGeoDBState(statePath, state)
}

// geoDBTransportCause omits the request URL from diagnostics because it may contain tokens.
func geoDBTransportCause(err error) error {
	if requestErr, ok := errors.AsType[*url.Error](err); ok {
		return requestErr.Err
	}

	return err
}

// parseGeoDBURL accepts public HTTPS endpoints and excludes credentials from URL strings.
func parseGeoDBURL(source string) (*url.URL, error) {
	endpoint, err := url.Parse(source)
	if err != nil ||
		endpoint.Scheme != "https" ||
		endpoint.Host == "" ||
		endpoint.User != nil ||
		endpoint.Fragment != "" {
		return nil, appError(
			"geo.error.url",
			"GeoIP update URL must be HTTPS and contain no credentials or fragment",
		)
	}

	return endpoint, nil
}

// parseGeoDBAuth splits the MaxMind account ID and license key for HTTP Basic Auth.
func parseGeoDBAuth(auth string) (string, string, error) {
	if auth == "" {
		return "", "", nil
	}

	username, password, ok := strings.Cut(auth, ":")
	if !ok || username == "" || password == "" {
		return "", "", appError("geo.error.auth", "GeoIP URL auth must use account-id:license-key")
	}

	return username, password, nil
}

// geoDBHTTPClient enforces HTTPS across redirects and bounds each update attempt.
func geoDBHTTPClient() *http.Client {
	return &http.Client{
		Timeout: geoDBDownloadTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if request.URL.Scheme != "https" {
				return appError("geo.error.redirect_scheme", "GeoIP update redirect must use HTTPS")
			}

			if len(via) >= 10 {
				return appError("geo.error.redirect_limit", "too many GeoIP update redirects")
			}

			return nil
		},
	}
}

// setGeoDBAuth adds credentials only to the initial HTTPS request.
func setGeoDBAuth(request *http.Request, username, password string) {
	if username != "" {
		request.SetBasicAuth(username, password)
	}
}

// sameGeoDBVersion uses Last-Modified when available and falls back to ETag.
func sameGeoDBVersion(state geoDBState, modified, etag string) bool {
	if modified != "" {
		return state.LastModified == modified
	}

	return etag != "" && state.ETag == etag
}

// geoDBSourceKey hashes the URL so endpoint changes invalidate cached validators
// without persisting query-string credentials or other sensitive URL parameters in the sidecar.
func geoDBSourceKey(source string) string {
	digest := sha256.Sum256([]byte(source))
	return hex.EncodeToString(digest[:])
}

// downloadGeoDB validates a temporary database before replacing the existing file.
func downloadGeoDB(
	client *http.Client,
	source, username, password, path string,
	progress geoDBProgressOptions,
) error {
	request, err := http.NewRequest(http.MethodGet, source, nil)
	if err != nil {
		return appWrap("geo.error.create_download", "create database download", err)
	}

	setGeoDBAuth(request, username, password)
	response, err := client.Do(request)
	if err != nil {
		return appWrap("geo.error.download", "download database", geoDBTransportCause(err))
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return appError("geo.error.download_status", "download database: HTTP %s", response.Status)
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return appWrap("geo.error.create_directory", "create database directory", err)
	}

	temporary, err := os.CreateTemp(directory, ".geo-db-*.mmdb")
	if err != nil {
		return appWrap("geo.error.create_temporary", "create temporary database", err)
	}

	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	var body io.Reader = response.Body
	var progressReader *geoDBProgressReader
	if progress.output != nil {
		// The progress package uses zero for an unknown total and renders an indeterminate bar.
		total := max(response.ContentLength, 0)

		progressReader = &geoDBProgressReader{
			Reader:   response.Body,
			progress: progress,
			total:    total,
		}
		body = progressReader
	}

	written, copyErr := io.Copy(temporary, io.LimitReader(body, geoDBMaxSize+1))
	if progressReader != nil {
		progressReader.finish(copyErr == nil && written > 0 && written <= geoDBMaxSize)
	}
	if copyErr != nil {
		_ = temporary.Close()
		return appWrap("geo.error.write_temporary", "write temporary database", copyErr)
	}

	if written == 0 || written > geoDBMaxSize {
		_ = temporary.Close()
		return appError(
			"geo.error.download_size",
			"downloaded database size %d is outside the allowed range",
			written,
		)
	}

	if err := temporary.Close(); err != nil {
		return appWrap("geo.error.close_temporary", "close temporary database", err)
	}

	if err := installDownloadedGeoDB(temporaryPath, directory, path); err != nil {
		return err
	}

	return nil
}

// Read advances the progress bar by the number of bytes actually received.
func (r *geoDBProgressReader) Read(buffer []byte) (int, error) {
	read, err := r.Reader.Read(buffer)
	if read == 0 {
		return read, err
	}

	// Delay renderer startup until bytes arrive so empty responses stay silent.
	if r.startedAt.IsZero() {
		r.startedAt = time.Now()
		if !r.progress.terminal {
			_, _ = fmt.Fprintln(r.progress.output, r.progress.startedMessage)
			return read, err
		}

		r.startProgressBar()
	}

	if r.progress.terminal {
		r.tracker.Increment(int64(read))
	}
	return read, err
}

// startProgressBar creates the renderer after the first network bytes arrive.
func (r *geoDBProgressReader) startProgressBar() {
	style := progress.StyleDefault
	style.Visibility.Percentage = true
	style.Visibility.Speed = true
	style.Visibility.Value = true

	r.writer = progress.NewWriter()
	r.writer.SetOutputWriter(r.progress.output)
	r.writer.SetStyle(style)
	r.writer.SetAutoStop(true)
	r.tracker = &progress.Tracker{
		Message:    r.progress.startedMessage,
		Total:      r.total,
		Units:      progress.UnitsBytes,
		DeferStart: true,
	}
	r.writer.AppendTracker(r.tracker)
	r.renderEnd = make(chan struct{})
	go func() {
		defer close(r.renderEnd)
		r.writer.Render()
	}()
}

// finish completes the rendered status and waits for its final terminal update.
func (r *geoDBProgressReader) finish(success bool) {
	if r.startedAt.IsZero() {
		return
	}

	if !r.progress.terminal {
		message := r.progress.failedMessage
		if success {
			message = r.progress.completedMessage
		}
		_, _ = fmt.Fprintf(
			r.progress.output,
			message,
			time.Since(r.startedAt).Round(time.Millisecond).String(),
		)
		_, _ = fmt.Fprintln(r.progress.output)

		return
	}

	if success {
		r.tracker.MarkAsDone()
	} else {
		r.tracker.MarkAsErrored()
	}
	<-r.renderEnd
}

// installDownloadedGeoDB accepts a raw MMDB or MaxMind's gzip-compressed tar download.
func installDownloadedGeoDB(downloadPath, directory, destination string) error {
	reader, rawErr := geoip2.Open(downloadPath)
	if rawErr == nil {
		if err := reader.Close(); err != nil {
			return appWrap("geo.error.close_validated", "close validated database", err)
		}

		if err := os.Rename(downloadPath, destination); err != nil {
			return appWrap("geo.error.install_database", "install updated database", err)
		}

		return nil
	}

	// MaxMind's download permalink returns a gzip-compressed archive,
	// while public mirrors often expose the .mmdb file directly.
	// Extract only MMDB contents.
	if err := extractGeoDBArchive(downloadPath, directory, destination); err != nil {
		return errors.Join(appWrap("geo.error.validate_download", "validate downloaded database", rawErr), err)
	}
	return nil
}

// extractGeoDBArchive streams one MMDB member from a gzip tar without unpacking paths.
func extractGeoDBArchive(archivePath, directory, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return appWrap("geo.error.open_archive", "open compressed database", err)
	}
	defer func() { _ = file.Close() }()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return appWrap("geo.error.open_archive", "open compressed database", err)
	}
	defer func() { _ = gz.Close() }()

	// Bound decompressed archive data too;
	// a small gzip response must not expand into an unbounded stream
	// while tar.Next skips unrelated members.
	archive := tar.NewReader(io.LimitReader(gz, geoDBMaxSize+1))
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return appError(
				"geo.error.archive_member",
				"compressed database does not contain an MMDB file",
			)
		}
		if err != nil {
			return appWrap("geo.error.read_archive", "read compressed database", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(header.Name), ".mmdb") {
			continue
		}

		if header.Size <= 0 || header.Size > geoDBMaxSize {
			return appError(
				"geo.error.archive_size",
				"archived database size %d is outside the allowed range",
				header.Size,
			)
		}

		return writeExtractedGeoDB(archive, directory, destination, header.Size)
	}
}

// writeExtractedGeoDB validates streamed archive content before replacing the database.
func writeExtractedGeoDB(source io.Reader, directory, destination string, size int64) error {
	temporary, err := os.CreateTemp(directory, ".geo-db-*.mmdb")
	if err != nil {
		return appWrap("geo.error.create_extracted", "create extracted database", err)
	}

	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	written, copyErr := io.Copy(temporary, io.LimitReader(source, size+1))
	if copyErr != nil {
		_ = temporary.Close()
		return appWrap("geo.error.extract", "extract database", copyErr)
	}

	if written != size {
		_ = temporary.Close()
		return appError(
			"geo.error.extracted_size",
			"extracted database size %d does not match archive size %d",
			written,
			size,
		)
	}

	if err := temporary.Close(); err != nil {
		return appWrap("geo.error.close_extracted", "close extracted database", err)
	}

	reader, err := geoip2.Open(temporaryPath)
	if err != nil {
		return appWrap("geo.error.validate_extracted", "validate extracted database", err)
	}
	if err := reader.Close(); err != nil {
		return appWrap("geo.error.close_validated", "close validated database", err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return appWrap("geo.error.install_database", "install updated database", err)
	}

	return nil
}

// readGeoDBState loads per-file update metadata; absent state triggers an initial download.
func readGeoDBState(path string) (geoDBState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return geoDBState{}, err
	}

	var state geoDBState
	if err := json.Unmarshal(data, &state); err != nil {
		return geoDBState{}, appWrap("geo.error.read_state", "read GeoIP update state", err)
	}

	return state, nil
}

// writeGeoDBState atomically persists the request throttle and remote validators.
func writeGeoDBState(path string, state geoDBState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return appWrap("geo.error.encode_state", "encode GeoIP update state", err)
	}

	temporary, err := os.CreateTemp(filepath.Dir(path), ".geo-state-*.json")
	if err != nil {
		return appWrap("geo.error.create_state", "create GeoIP update state", err)
	}

	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return appWrap("geo.error.write_state", "write GeoIP update state", err)
	}
	if err := temporary.Close(); err != nil {
		return appWrap("geo.error.close_state", "close GeoIP update state", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return appWrap("geo.error.install_state", "install GeoIP update state", err)
	}

	return nil
}

// markGeoDBCheck throttles repeated failures while retaining the last known validators.
func markGeoDBCheck(path string, state geoDBState, now time.Time, cause error) error {
	state.CheckedAt = now
	if err := writeGeoDBState(path, state); err != nil {
		return errors.Join(cause, err)
	}

	return cause
}
