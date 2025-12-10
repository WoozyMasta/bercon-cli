package bercon

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Default timeouts and buffer sizes.
const (
	// DefaultKeepaliveTimeout is the default keepalive interval in seconds.
	// It MUST stay below ~45s, otherwise BattlEye tends to drop idle sessions.
	DefaultKeepaliveTimeout = 30

	// DefaultDeadlineTimeout is the default request/response deadline in seconds.
	DefaultDeadlineTimeout = 5

	// DefaultMicroSleepTimeout is the default sleep (milliseconds) used in
	// the tight loop that waits for a free sequence number under load.
	// Higher values reduce CPU at the cost of a tiny extra latency per
	// contended Send(); lower values improve responsiveness but may spin more.
	// Recommended range: 0–2ms. See SetMicroSleepTimeout for details.
	DefaultMicroSleepTimeout = 1

	// DefaultBufferSize is the max body size accepted in a single UDP read.
	DefaultBufferSize = 1024

	// DefaultBufferHeaderSize accounts for fixed packet overhead.
	DefaultBufferHeaderSize = 16

	// MaxCommandBodySize is the protocol limit for a single client command body.
	// The client never sends multipart commands.
	MaxCommandBodySize = 1391
)

// Timeouts defines various timeout configurations for the connection.
type Timeouts struct {
	keepalive  time.Duration // interval for sending keepalive packets
	deadline   time.Duration // maximum time to wait for a response
	microSleep time.Duration // sleep duration during busy-wait loops
}

// PacketEvent is a struct for broadcasting incoming packets (like login or messages)
// so that client code can handle them (log them, etc.).
type PacketEvent struct {
	Time time.Time
	Data []byte
	Seq  byte
}

// Connection represents a connection to the BattlEye server.
type Connection struct {

	// lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	close  sync.Once

	// owned by manager loop
	conn     *net.UDPConn
	inflight map[byte]*inflight
	reqCh    chan sendReq // requests from Send()
	pktCh    chan *packet // parsed packets from reader
	ackCh    chan byte    // message seq to ack
	msgCh    chan *packet // internal channel for message dispatch

	// outward events
	Messages chan PacketEvent

	// immutable after Open
	address  string
	password string

	// config (atomic enough for our use)
	timeouts   Timeouts
	wg         sync.WaitGroup
	alive      uint32 // 1 if active
	bufferSize uint16

	sequence  byte
	keepalive bool
}

// internal plumbing for Send()
type sendReq struct {
	respCh  chan sendResp
	command string
}

type sendResp struct {
	err  error
	data []byte
}

// single in-flight response aggregator (for multipart)
type inflight struct {
	ts    time.Time
	done  chan sendResp
	data  []byte
	pages byte
	page  byte
}

// Open initializes and returns a new Connection to the specified BattlEye server using the provided address and password.
func Open(addr, pass string) (*Connection, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	rawConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	c := &Connection{
		address:    addr,
		password:   pass,
		conn:       rawConn,
		bufferSize: DefaultBufferSize + DefaultBufferHeaderSize,
		timeouts: Timeouts{
			keepalive:  DefaultKeepaliveTimeout * time.Second,
			deadline:   DefaultDeadlineTimeout * time.Second,
			microSleep: DefaultMicroSleepTimeout * time.Millisecond,
		},
		Messages: make(chan PacketEvent, 32),

		reqCh:    make(chan sendReq, 4),
		pktCh:    make(chan *packet, 64),
		ackCh:    make(chan byte, 64),
		msgCh:    make(chan *packet, 64),
		inflight: make(map[byte]*inflight, 16),
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())

	// login synchronously before loops
	if err := c.loginOnce(); err != nil {
		_ = rawConn.Close()
		return nil, err
	}

	atomic.StoreUint32(&c.alive, 1)

	// start reader, manager and dispatcher
	c.wg.Add(3)
	go c.readerLoop()
	go c.managerLoop()
	go c.dispatchLoop()

	return c, nil
}

// SetBufferSize updates the buffer size for receiving packets from the server.
func (c *Connection) SetBufferSize(size uint16) {
	cumulative := uint16(MaxCommandBodySize + DefaultBufferHeaderSize)
	if size > cumulative {
		size = cumulative
	}

	c.bufferSize = size
}

// Keepalive returns the current keepalive interval (how often a
// keepalive packet is sent to maintain the session).
func (c *Connection) Keepalive() time.Duration {
	return c.timeouts.keepalive
}

// SetKeepalive sets how often keepalive packets are sent.
// Values <= 0 use DefaultKeepaliveTimeout. Values >= ~45s are clamped to default
// because BE tends to drop idle sessions above that.
func (c *Connection) SetKeepalive(d time.Duration) {
	switch {
	case d <= 0:
		c.timeouts.keepalive = DefaultKeepaliveTimeout * time.Second

	case d >= 45*time.Second:
		c.timeouts.keepalive = DefaultKeepaliveTimeout * time.Second

	default:
		c.timeouts.keepalive = d
	}
}

// SetKeepaliveTimeout configures how often (in seconds) keepalive packets are sent to maintain the connection.
// If seconds >= 45, it resets to the default because the server typically disconnects if the interval is too long.
func (c *Connection) SetKeepaliveTimeout(seconds int) {
	if seconds >= 45 || seconds <= 0 {
		c.timeouts.keepalive = DefaultKeepaliveTimeout * time.Second
		return
	}

	c.timeouts.keepalive = time.Duration(seconds) * time.Second
}

// Deadline returns the current request/response deadline. This is the
// maximum time a Send() call will wait for a server response before failing.
func (c *Connection) Deadline() time.Duration {
	return c.timeouts.deadline
}

// SetDeadline sets the maximum time to wait for a server response.
func (c *Connection) SetDeadline(d time.Duration) {
	if d <= 0 {
		c.timeouts.deadline = DefaultDeadlineTimeout * time.Second
		return
	}

	c.timeouts.deadline = d
}

// SetDeadlineTimeout sets the max time (in seconds) to wait for a server response.
func (c *Connection) SetDeadlineTimeout(seconds int) {
	c.timeouts.deadline = time.Duration(seconds) * time.Second
}

// MicroSleep returns the current micro-sleep interval used in the tight loop
// when searching for a free sequence number under contention. Zero means
// sleeping is disabled (busy-spin).
func (c *Connection) MicroSleep() time.Duration {
	return c.timeouts.microSleep
}

// SetMicroSleep sets the micro-sleep interval used while waiting for a free
// sequence slot under contention. 0 disables sleeping (max responsiveness).
func (c *Connection) SetMicroSleep(d time.Duration) {
	if d <= 0 {
		c.timeouts.microSleep = 0
		return
	}

	c.timeouts.microSleep = d
}

// SetMicroSleepTimeout adjusts the micro-sleep interval (in ms) used in the
// loop that searches for a free sequence number when all 0..255 slots are
// temporarily busy. A value of 0 means no sleeping (max responsiveness,
// more CPU when heavily contended). Typical sweet spot: 1–2ms.
func (c *Connection) SetMicroSleepTimeout(milliseconds int) {
	if milliseconds <= 0 {
		c.timeouts.microSleep = 0
		return
	}

	c.timeouts.microSleep = time.Duration(milliseconds) * time.Millisecond
}

// IsAlive checks if the connection is active and not closed.
func (c *Connection) IsAlive() bool {
	return atomic.LoadUint32(&c.alive) == 1
}

// Close gracefully closes the connection, releases resources, and ensures no further operations are performed.
func (c *Connection) Close() error {
	if !c.IsAlive() {
		return nil
	}

	var err error
	c.close.Do(func() {
		atomic.StoreUint32(&c.alive, 0)
		c.cancel()

		// Force close socket for unlock readerLoop
		if c.conn != nil {
			err = c.conn.Close()
		}

		c.wg.Wait()
		close(c.Messages)
	})

	return err
}

// StartKeepAlive begins a routine that sends periodic keepalive packets.
func (c *Connection) StartKeepAlive() {
	c.keepalive = true
}

// Send dispatches a command to the BattlEye server and waits for a response.
func (c *Connection) Send(command string) ([]byte, error) {
	if !c.IsAlive() {
		return nil, ErrConnectionDown
	}

	// global timer for operation
	timer := time.NewTimer(c.timeouts.deadline)
	defer timer.Stop()

	respCh := make(chan sendResp, 1)
	req := sendReq{
		command: command,
		respCh:  respCh,
	}

	// push request
	select {
	case c.reqCh <- req: // succes add to queue

	case <-c.ctx.Done():
		return nil, ErrConnectionClosed

	case <-timer.C: // if queue overflow or managerLoop stuck
		return nil, ErrTimeout
	}

	// wait for response or deadline
	select {
	case resp := <-respCh:
		return resp.data, resp.err

	case <-c.ctx.Done():
		return nil, ErrConnectionClosed

	case <-timer.C:
		return nil, ErrTimeout
	}
}
