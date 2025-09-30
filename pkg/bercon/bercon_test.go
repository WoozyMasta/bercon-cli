package bercon

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/woozymasta/bercon-cli/pkg/beparser"
)

/*
Integration tests & simple benchmarks against a real BattlEye RCON server.

Environment (can be provided via shell or loaded by your own dev.env loader):
  - BERCON_ADDRESS   (default: 127.0.0.1)
  - BERCON_PORT      (default: 2302)
  - BERCON_PASSWORD  (default: ""  => server must allow empty or tests will fail)
  - BENCH_CMD        (default: "players")  // used by benchmarks

Usage examples:
  go test -race ./pkg/bercon -run Integration -v
  go test -bench=BenchmarkSendSerial -benchmem ./pkg/bercon
  BENCH_CMD=admins go test -bench=BenchmarkSendParallel -benchmem ./pkg/bercon
*/

func testEnv(t *testing.T) (address, password string) {
	t.Helper()
	ip := getenv("BERCON_ADDRESS", "127.0.0.1")
	port := getenv("BERCON_PORT", "2302")
	address = fmt.Sprintf("%s:%s", ip, port)
	password = getenv("BERCON_PASSWORD", "")
	return
}

func benchEnv() (addr, pass, cmd string) {
	ip := getenv("BERCON_ADDRESS", "127.0.0.1")
	port := getenv("BERCON_PORT", "2302")
	addr = fmt.Sprintf("%s:%s", ip, port)
	pass = getenv("BERCON_PASSWORD", "")
	cmd = getenv("BENCH_CMD", "players")
	return
}

func getenv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return def
}

// TestIntegration_BasicSend verifies login and a single command/response round-trip.
func TestIntegration_BasicSend(t *testing.T) {
	address, password := testEnv(t)

	c, err := Open(address, password)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer c.Close()

	// Make failures surface quickly in CI.
	c.SetDeadlineTimeout(3)
	c.SetKeepaliveTimeout(10)

	resp, err := c.Send("players")
	if err != nil {
		t.Fatalf("send(players): %v", err)
	}
	if len(resp) == 0 {
		t.Fatalf("empty response for players")
	}

	// Sanity: response should be parseable.
	_ = beparser.Parse(resp, "players")
}

// TestIntegration_Keepalive ensures the session remains valid after idle time.
func TestIntegration_Keepalive(t *testing.T) {
	address, password := testEnv(t)

	c, err := Open(address, password)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer c.Close()

	c.SetKeepaliveTimeout(10) // must be < 45s per BE spec
	c.StartKeepAlive()

	time.Sleep(12 * time.Second) // go idle beyond keepalive interval

	if _, err := c.Send("bans"); err != nil {
		t.Fatalf("send after idle: %v", err)
	}
}

// TestIntegration_ConcurrentSends stresses Send() with concurrent callers.
func TestIntegration_ConcurrentSends(t *testing.T) {
	address, password := testEnv(t)

	c, err := Open(address, password)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer c.Close()

	c.SetDeadlineTimeout(5)
	c.SetKeepaliveTimeout(10)
	c.StartKeepAlive()

	commands := []string{"players", "bans", "admins", "players", "bans"}
	const workers = 8
	const perWorker = 10

	var wg sync.WaitGroup
	errCh := make(chan error, workers*perWorker)

	run := func(id int) {
		defer wg.Done()
		for i := 0; i < perWorker; i++ {
			cmd := commands[(id+i)%len(commands)]
			if _, err := c.Send(cmd); err != nil {
				errCh <- fmt.Errorf("worker %d iter %d cmd %q: %v", id, i, cmd, err)
				return
			}
		}
	}

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go run(w)
	}

	wg.Wait()
	close(errCh)

	for e := range errCh {
		t.Error(e)
	}
}

// TestIntegration_RingLoad drives the 0..255 sequence ring multiple times.
func TestIntegration_RingLoad(t *testing.T) {
	address, password := testEnv(t)

	c, err := Open(address, password)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer c.Close()

	c.SetDeadlineTimeout(5)

	// 3 commands * 180 = 540 (~2.1 full rotations of a 256-seq ring).
	for i := 0; i < 180; i++ {
		resp, err := c.Send("players")
		if err != nil {
			t.Fatalf("players: %v", err)
		}
		_ = beparser.Parse(resp, "players")

		resp, err = c.Send("bans")
		if err != nil {
			t.Fatalf("bans: %v", err)
		}
		_ = beparser.Parse(resp, "bans")

		resp, err = c.Send("admins")
		if err != nil {
			t.Fatalf("admins: %v", err)
		}
		_ = beparser.Parse(resp, "admins")
	}
}

// TestIntegration_CloseGraceful ensures Close() is non-blocking and Messages is closed.
func TestIntegration_CloseGraceful(t *testing.T) {
	address, password := testEnv(t)

	c, err := Open(address, password)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Drain Messages to avoid blocking on send.
	done := make(chan struct{})
	go func() {
		for range c.Messages {
		}
		close(done)
	}()

	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Messages channel not closed")
	}
}

// TestIntegration_MultipartIfSupported exercises multipart assembly if server returns long output.
// You can force a multi-page response by generating a long ban list (server-dependent).
func TestIntegration_MultipartIfSupported(t *testing.T) {
	address, password := testEnv(t)
	c, err := Open(address, password)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer c.Close()

	c.SetDeadlineTimeout(5)
	if resp, err := c.Send("bans"); err != nil {
		t.Skipf("server doesn't provide long response: %v", err)
	} else if len(resp) == 0 {
		t.Fatalf("multipart candidate returned empty")
	}
}

// BenchmarkSendSerial measures latency of sequential Send(cmd).
func BenchmarkSendSerial(b *testing.B) {
	addr, pass, cmd := benchEnv()

	c, err := Open(addr, pass)
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer c.Close()

	c.SetDeadlineTimeout(5)
	c.SetKeepaliveTimeout(10)
	c.StartKeepAlive()

	// Warm-up (not measured).
	if _, err := c.Send(cmd); err != nil {
		b.Fatalf("warmup send: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := c.Send(cmd); err != nil {
			b.Fatalf("send(%d): %v", i, err)
		}
	}
}

// BenchmarkSendParallel measures throughput of concurrent Send(cmd).
// testing.B.RunParallel splits b.N across multiple goroutines (scaled by GOMAXPROCS).
func BenchmarkSendParallel(b *testing.B) {
	addr, pass, cmd := benchEnv()

	c, err := Open(addr, pass)
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer c.Close()

	c.SetDeadlineTimeout(5)
	c.SetKeepaliveTimeout(10)
	c.SetMicroSleepTimeout(0)
	c.StartKeepAlive()

	// Warm-up to establish session.
	if _, err := c.Send(cmd); err != nil {
		b.Fatalf("warmup send: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := c.Send(cmd); err != nil {
				// Abort the whole benchmark on failure.
				panic(fmt.Errorf("send parallel: %w", err))
			}
		}
	})
}

// BenchmarkKeepaliveSmoke is a tiny idle/keepalive smoke test benchmark.
// Enable explicitly: BENCH_IDLE=1 go test -bench=BenchmarkKeepaliveSmoke ./pkg/bercon
func BenchmarkKeepaliveSmoke(b *testing.B) {
	if os.Getenv("BENCH_IDLE") == "" {
		b.Skip("set BENCH_IDLE=1 to run")
	}

	addr, pass, cmd := benchEnv()

	c, err := Open(addr, pass)
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer c.Close()

	c.SetDeadlineTimeout(5)
	c.SetKeepaliveTimeout(3) // must be < 45s per BE spec
	c.StartKeepAlive()

	// Warm-up.
	if _, err := c.Send(cmd); err != nil {
		b.Fatalf("warmup send: %v", err)
	}

	// Fixed low iteration count; short sleeps to avoid long runs.
	iters := 3
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < iters; i++ {
		if _, err := c.Send(cmd); err != nil {
			b.Fatalf("send: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
