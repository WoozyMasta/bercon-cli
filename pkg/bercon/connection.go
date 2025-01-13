package bercon

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Default timeouts and buffer sizes.
const (
	DefaultKeepaliveTimeout  = 30   // Default keepalive in seconds, must not exceed 45
	DefaultDeadlineTimeout   = 5    // Default deadline timeout in seconds
	DefaultMicroSleepTimeout = 10   // Default micro-sleep timeout in milliseconds
	DefaultBufferSize        = 1024 // 1024 bytes max body data
	DefaultBufferHeaderSize  = 16   // 7 bytes header, 1 byte type, 1 byte seq, 1 byte terminator, 2 bytes pages, etc.
)

// Timeouts defines various timeout configurations for the connection.
type Timeouts struct {
	keepalive  time.Duration // interval for sending keepalive packets
	deadline   time.Duration // maximum time to wait for a response
	microSleep time.Duration // sleep duration during busy-wait loops
}

// Response represents a single response from the server, potentially part of a multipart response.
type Response struct {
	timestamp time.Time // timestamp when this response was received
	data      []byte    // data received in the response
	pages     byte      // total number of pages in the response (for multipart responses)
	page      byte      // current page number of this response segment
}

// PacketEvent is a struct for broadcasting incoming packets (like login or messages)
// so that client code can handle them (log them, etc.).
type PacketEvent struct {
	Data []byte
	Seq  byte
	Time time.Time
}

// Connection represents a connection to the BattlEye server.
type Connection struct {
	buffer     [256]*Response // buffer for storing responses indexed by sequence number
	conn       *net.UDPConn   // UDP connection to the server
	done       chan struct{}  // channel to signal the closure of keepalive and listener routines
	address    string         // server address in "IP:Port" format
	password   string         // password for authenticating with the server
	timeouts   Timeouts       // timeout configurations for this connection
	bufferSize uint16         // size of the buffer for receiving packets
	bufferMu   sync.Mutex     // mutex for synchronizing access to the response buffer
	connMu     sync.Mutex     // mutex for synchronizing access to the UDP connection
	alive      uint32         // atomic flag (1 if active, 0 if closed)
	sequence   byte           // current packet sequence number

	// Messages is a channel to which we will send PacketEvents for any non-command packets (e.g. loginPacket, messagePacket).
	// Client code can read from this channel to handle them externally (logging, saving to file, etc.).
	Messages chan PacketEvent
}

// Open initializes and returns a new Connection to the specified BattlEye server using the provided address and password.
func Open(addr, pass string) (*Connection, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	timeouts := Timeouts{
		keepalive:  DefaultKeepaliveTimeout * time.Second,
		deadline:   DefaultDeadlineTimeout * time.Second,
		microSleep: DefaultMicroSleepTimeout * time.Millisecond,
	}

	c := &Connection{
		conn:       conn,
		address:    addr,
		password:   pass,
		sequence:   0,
		bufferSize: DefaultBufferSize + DefaultBufferHeaderSize,
		timeouts:   timeouts,
		Messages:   make(chan PacketEvent, 10),
	}

	if err := c.login(); err != nil {
		return nil, err
	}

	atomic.StoreUint32(&c.alive, 1)
	c.startListening()

	return c, nil
}

// SetBufferSize updates the buffer size for receiving packets from the server.
func (c *Connection) SetBufferSize(size uint16) {
	c.bufferSize = size
}

// SetKeepaliveTimeout configures how often (in seconds) keepalive packets are sent to maintain the connection.
// If seconds >= 45, it resets to the default because the server typically disconnects if the interval is too long.
func (c *Connection) SetKeepaliveTimeout(seconds int) {
	if seconds >= 45 {
		c.timeouts.keepalive = DefaultKeepaliveTimeout * time.Second
		return
	}
	c.timeouts.keepalive = time.Duration(seconds) * time.Second
}

// SetDeadlineTimeout sets the max time (in seconds) to wait for a server response.
func (c *Connection) SetDeadlineTimeout(seconds int) {
	c.timeouts.deadline = time.Duration(seconds) * time.Second
}

// SetMicroSleepTimeout adjusts the micro-sleep interval (in milliseconds) used in busy-wait loops.
func (c *Connection) SetMicroSleepTimeout(milliseconds int) {
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
	atomic.StoreUint32(&c.alive, 0)

	if c.done != nil {
		close(c.done)
		c.done = nil
	}

	if c.conn != nil {
		err := c.conn.Close()
		if err != nil {
			return err
		}
		c.conn = nil
	}

	close(c.Messages)

	return nil
}

// StartKeepAlive begins a routine that sends periodic keepalive packets.
func (c *Connection) StartKeepAlive() {
	if c.done != nil {
		// Keepalive is already running.
		return
	}
	c.done = make(chan struct{})

	go func() {
		ticker := time.NewTicker(c.timeouts.keepalive)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// send keepalive
				_, err := c.Send("")
				if err != nil {
					c.Close()
					return
				}
			case <-c.done:
				return
			}
		}
	}()
}

// Send dispatches a command to the BattlEye server and waits for a response.
func (c *Connection) Send(command string) ([]byte, error) {
	if !c.IsAlive() {
		return nil, ErrConnectionDown
	}

	c.connMu.Lock()
	defer c.connMu.Unlock()

	seq := c.sequence

	// check if buffer is occupied by current seq
	if c.buffer[seq] != nil {
		timeout := time.After(c.timeouts.deadline)
		for {
			select {
			case <-timeout:
				return nil, ErrBufferFull
			default:
				if c.buffer[seq] == nil {
					break
				}
				time.Sleep(c.timeouts.microSleep)
			}
		}
	}

	// increment seq
	c.sequence++

	// send the packet
	if err := c.writePacket(commandPacket, []byte(command), seq); err != nil {
		return nil, err
	}

	// keepalive packets are empty commands
	// waiting for an answer
	data, err := c.getResponse(seq)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// getResponse waits for and retrieves the server response for the specified seq.
func (c *Connection) getResponse(seq byte) ([]byte, error) {
	timeout := time.After(c.timeouts.deadline)

	for {
		select {
		case <-timeout:
			return nil, ErrTimeout

		case <-c.done:
			return nil, ErrConnectionClosed

		default:
			c.bufferMu.Lock()
			if c.buffer[seq] != nil {
				resp := c.buffer[seq]
				// if all parts collected or it's a single-part
				if resp.pages == 0 || resp.pages == resp.page+1 {
					data := resp.data
					c.buffer[seq] = nil
					c.bufferMu.Unlock()

					return data, nil
				}
			}
			c.bufferMu.Unlock()

			time.Sleep(c.timeouts.microSleep)
		}
	}
}

// login authenticates with the server using the provided password.
func (c *Connection) login() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if err := c.writePacket(loginPacket, []byte(c.password), 0); err != nil {
		return err
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(c.timeouts.deadline)); err != nil {
		return err
	}

	packet, err := c.readPacket()
	if err != nil || packet.kind != loginPacket {
		return ErrNotResponse
	}

	if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
		return err
	}

	if len(packet.data) == 0 || packet.data[0] != loginSuccess {
		return ErrLoginFailed
	}

	return nil
}

// writePacket constructs and sends a packet of the specified type, data and sequence to the server.
func (c *Connection) writePacket(kind packetKind, data []byte, seq byte) error {
	if len(data) > int(c.bufferSize-9) {
		// 9 == 7-bytes header + 1-byte type + 1-byte sequence
		return ErrBadSize
	}

	pkt := new(packet)
	pkt.make(data, kind, seq)
	raw, err := pkt.toBytes()
	if err != nil {
		return err
	}

	_, err = c.conn.Write(raw)
	return err
}

// startListening starts a goroutine that handles incoming packets from the server.
func (c *Connection) startListening() {
	go func() {
		for {
			select {
			case <-c.done:
				return
			default:
				if c.conn == nil || !c.IsAlive() {
					c.Close()
					return
				}

				pkt, err := c.readPacket()
				if err != nil {
					if !c.IsAlive() {
						return
					}
					c.Close()
					return
				}

				switch pkt.kind {
				case loginPacket:
					// Broadcast login packet through channel
					c.Messages <- PacketEvent{
						Data: pkt.data,
						Seq:  pkt.seq,
						Time: time.Now(),
					}

				case messagePacket:
					// Forward incoming message via channel
					c.Messages <- PacketEvent{
						Data: pkt.data,
						Seq:  pkt.seq,
						Time: time.Now(),
					}

					// Acknowledge the message
					_ = c.writePacket(messagePacket, nil, pkt.seq)

				case commandPacket:
					_ = c.storeResponse(pkt)
				}
			}
		}
	}()
}

// readPacket reads a single packet from the server and returns it.
func (c *Connection) readPacket() (*packet, error) {
	if c.conn == nil {
		return nil, ErrConnectionClosed
	}

	buf := make([]byte, c.bufferSize)
	n, err := c.conn.Read(buf)
	if err != nil {
		if netErr, ok := err.(*net.OpError); ok && c.done == nil && netErr.Err.Error() == "use of closed network connection" {
			return nil, ErrConnectionClosed
		}
		return nil, err
	}

	pkt, err := fromBytes(buf[:n])
	if err != nil {
		return nil, err
	}

	return pkt, nil
}

// storeResponse handles the received command packet response in the buffer, collecting multipart segments if necessary.
func (c *Connection) storeResponse(pkt *packet) error {
	c.bufferMu.Lock()
	defer c.bufferMu.Unlock()

	seq := pkt.seq

	if c.buffer[seq] == nil {
		c.buffer[seq] = &Response{
			data:      pkt.data,
			pages:     pkt.pages,
			page:      pkt.page,
			timestamp: time.Now(),
		}

	} else if c.buffer[seq].pages > 0 {
		// appending new pages
		if c.buffer[seq].page+1 != pkt.page {
			return ErrBadSequence
		}
		c.buffer[seq].data = append(c.buffer[seq].data, pkt.data...)
		c.buffer[seq].page = pkt.page

	} else {
		return ErrBadPart
	}

	return nil
}
