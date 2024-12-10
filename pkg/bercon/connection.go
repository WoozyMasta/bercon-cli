package bercon

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	DefaultKeepaliveTimeout  = 30   // default keep alive in seconds, must be not more 45
	DefaultDeadlineTimeout   = 5    // default deadline timeout in seconds
	DefaultMicroSleepTimeout = 10   // default micro-sleep timeout in microseconds
	DefaultBufferSize        = 1024 // 1024b max body data
	DefaultBufferHeaderSize  = 16   // 7byte header, 1b type, 1b seq, 1b terminator, 2b pages and + just in case
)

// defines various timeout configurations for the connection.
type Timeouts struct {
	keepalive  time.Duration // interval for sending keepalive packets
	deadline   time.Duration // maximum time to wait for a response
	microSleep time.Duration // sleep duration during busy-wait loops
}

// represents a single response from the server, potentially a part of a multipart response
type Response struct {
	timestamp time.Time // timestamp of when this response was received
	data      []byte    // data received in the response
	pages     byte      // total number of pages in the response (for multipart responses)
	page      byte      // current page number of this response segment
}

// represents a connection to the BattlEye server
type Connection struct {
	buffer     [256]*Response // buffer for storing responses indexed by sequence number
	conn       *net.UDPConn   // UDP connection to the server
	done       chan struct{}  // channel to signal the closure of keepalive and listener routines
	address    string         // server address in the form "IP:Port"
	password   string         // password for authenticating with the server
	timeouts   Timeouts       // timeout configurations for this connection
	bufferSize int            // size of the buffer for receiving packets
	bufferMu   sync.Mutex     // mutex for synchronizing access to the response buffer
	connMu     sync.Mutex     // mutex for synchronizing access to the UDP connection
	alive      uint32         // atomic flag indicating if the connection is active (1) or closed (0)
	sequence   byte           // current packet sequence number
}

// initializes a new connection to the specified BattlEye server using the provided address and password
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

	connection := &Connection{
		conn:       conn,
		address:    addr,
		password:   pass,
		sequence:   0,
		bufferSize: DefaultBufferSize + DefaultBufferHeaderSize,
		timeouts:   timeouts,
	}

	if err = connection.login(); err != nil {
		return nil, err
	}

	atomic.StoreUint32(&connection.alive, 1)
	connection.startListening()

	return connection, nil
}

// sets the buffer size for receiving packets from the server
func (c *Connection) SetBufferSize(size int) {
	c.bufferSize = size
}

// configures the interval (in seconds) for sending keepalive packets to maintain the connection
func (c *Connection) SetKeepaliveTimeout(seconds int) {
	if seconds >= 45 {
		log.Errorf("Keepalive timeout try set to %d, its more then limit of 45 and set to default %d seconds", seconds, DefaultKeepaliveTimeout)
		c.timeouts.keepalive = DefaultKeepaliveTimeout * time.Second
		return
	}

	c.timeouts.keepalive = time.Duration(seconds) * time.Second
}

// sets the timeout (in seconds) for waiting for responses from the server
func (c *Connection) SetDeadlineTimeout(seconds int) {
	c.timeouts.deadline = time.Duration(seconds) * time.Second
}

// adjusts the micro-sleep interval (in milliseconds) used for busy-wait loops
func (c *Connection) SetMicroSleepTimeout(millisecond int) {
	c.timeouts.microSleep = time.Duration(millisecond) * time.Millisecond
}

// checks if the connection is active and not closed
func (c *Connection) IsAlive() bool {
	return atomic.LoadUint32(&c.alive) == 1 // Atomic reading
}

// gracefully closes the connection, releases resources, and ensures no further operations are performed
func (c *Connection) Close() error {
	if !c.IsAlive() {
		log.Debug("Connection already closed")
		return nil
	}

	atomic.StoreUint32(&c.alive, 0)

	if c.done != nil {
		close(c.done)
		c.done = nil
	}

	if c.conn != nil {
		log.Info("BattlEye RCON closed")

		err := c.conn.Close()
		if err != nil {
			return err
		}
		c.conn = nil
	}

	return nil
}

// begins a routine that sends periodic keepalive packets to maintain the connection
func (c *Connection) StartKeepAlive() {
	if c.done != nil {
		log.Warn("Keepalive already exists, not starting again")
		return
	}

	c.done = make(chan struct{})

	go func() {
		ticker := time.NewTicker(c.timeouts.keepalive) // keepalive timer
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C: // every keepaliveTimeout seconds
				_, err := c.Send("")
				if err != nil {
					log.Errorf("Keepalive failed (%e)", err)
					c.Close()
					return
				}

			case <-c.done: // done signal was received
				log.Info("Keepalive stopped")
				return
			}
		}
	}()
}

// sends a command to the BattlEye server and waits for a response, returning the data received
func (c *Connection) Send(command string) ([]byte, error) {
	if !c.IsAlive() {
		return nil, ErrConnectionDown
	}

	c.connMu.Lock()
	defer c.connMu.Unlock()

	seq := c.sequence

	// checking that the buffer is not full for the current sequence
	if c.buffer[seq] != nil {
		// wait if the buffer is already busy
		timeout := time.After(c.timeouts.deadline)
		for {
			select {
			case <-timeout:
				return nil, ErrBufferFull

			default:
				if c.buffer[seq] == nil {
					break
				}

				log.Tracef("Buffer is full, wait %fs in %fs loop", c.timeouts.microSleep.Seconds(), c.timeouts.deadline.Seconds())
				time.Sleep(c.timeouts.microSleep)
			}
		}
	}

	// increment sequence
	c.sequence++

	// packet sent
	if err := c.writePacket(commandPacket, []byte(command), seq); err != nil {
		return nil, err
	}

	if command == "" {
		log.Debugf("Keepalive packet #%d sent", seq)
	} else {
		log.Debugf("Command '%s' sent in packet #%d", command, seq)
	}

	// waiting for an answer
	data, err := c.getResponse(seq)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// waits for and retrieves the response for the specified packet sequence number
func (c *Connection) getResponse(seq byte) ([]byte, error) {
	timeout := time.After(c.timeouts.deadline)

	for {
		select {
		case <-timeout:
			return nil, ErrTimeout

		case <-c.done:
			if !c.IsAlive() {
				log.Debugf("Response reading stopped due to connection closure.")
				return nil, ErrConnectionClosed
			}

			return nil, ErrConnectionClosed

		default:
			c.bufferMu.Lock()

			if c.buffer[seq] != nil {
				response := c.buffer[seq]

				if response.pages == 0 || response.pages == response.page+1 {
					data := response.data
					c.buffer[seq] = nil
					c.bufferMu.Unlock()
					return data, nil
				}
			}

			c.bufferMu.Unlock()

			time.Sleep(c.timeouts.microSleep) // short pause before recheck
		}
	}
}

// authenticates with the server using the provided password
func (c *Connection) login() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if err := c.writePacket(loginPacket, []byte(c.password), 0); err != nil {
		return err
	}
	log.Debug("Login request send")

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

	if packet.data[0] != loginSuccess {
		return ErrLoginFailed
	}
	log.Infof("Login to %s success", c.address)

	return nil
}

// constructs and sends a packet of the specified type and sequence to the server
func (c *Connection) writePacket(kind packetKind, data []byte, seq byte) error {
	if len(data) > c.bufferSize-9 { // 9 == header 7-bytes, 1-byte type, 1-byte sequence number
		return ErrBadSize
	}

	packet := new(packet)
	packet.make([]byte(data), kind, seq)
	data, err := packet.toBytes()
	if err != nil {
		return err
	}

	_, err = c.conn.Write(data)
	if err != nil {
		return err
	}

	return nil
}

// starts a goroutine to handle incoming packets from the server
func (c *Connection) startListening() {
	log.Debugf("Listening start")

	go func() {
		for {
			select {
			case <-c.done:
				log.Debugf("Listening stopped")
				return

			default:
				if c.conn == nil || !c.IsAlive() {
					log.Warn("Connection is closed, stopping listener")
					c.Close()
					return
				}

				packet, err := c.readPacket()
				if err != nil {
					if !c.IsAlive() {
						log.Debugf("Read aborted due to connection closure")
						return
					}

					log.Warnf("Failed to get response: %v", err)
					c.Close()
					return
				}

				switch packet.kind {
				case loginPacket:
					log.Infof("Login session: %s", packet.data)

				case messagePacket:
					c.messageAcknowledge(packet)

				case commandPacket:
					if err := c.storeResponse(packet); err != nil {
						log.Warnf("Failed to store response: %v", err)
					}
				}
			}
		}
	}()
}

// reads a single packet from the server and returns it
func (c *Connection) readPacket() (*packet, error) {
	var response *packet

	if c.conn == nil { // closed connection
		return response, ErrConnectionClosed
	}

	buf := make([]byte, c.bufferSize)
	count, err := c.conn.Read(buf)
	if err != nil {
		if netErr, ok := err.(*net.OpError); ok && c.done == nil && netErr.Err.Error() == "use of closed network connection" {
			return response, ErrConnectionClosed
		}
		return response, err
	}

	response, err = fromBytes(buf[:count])
	if err != nil {
		return response, err
	}

	return response, nil
}

// stores the received response packet in the buffer, assembling multipart responses if necessary
func (c *Connection) storeResponse(response *packet) error {
	c.bufferMu.Lock()
	defer c.bufferMu.Unlock()

	seq := response.seq

	if c.buffer[seq] == nil { // if this is the first segment of the packet, create a new response
		c.buffer[seq] = &Response{
			data:      response.data,
			pages:     response.pages,
			page:      response.page,
			timestamp: time.Now(),
		}

	} else if c.buffer[seq].pages > 0 { // populate data in exists response
		if c.buffer[seq].page+1 != response.page {
			return ErrBadSequence
		}
		c.buffer[seq].data = append(c.buffer[seq].data, response.data...)
		c.buffer[seq].page = response.page
	} else {

		log.Warn("Didn't expect such behavior, don't know how it happened")
	}

	// Проверяем, завершён ли пакет
	if c.buffer[seq].page+1 == c.buffer[seq].pages {
		log.Tracef("All %d parts of the multipart response #%d have been received", c.buffer[seq].pages, seq)
	}

	return nil
}

// sends an acknowledgment for a received message packet back to the server
func (c *Connection) messageAcknowledge(packet *packet) {
	log.Debugf("Message received: %s", packet.data)

	if err := c.writePacket(messagePacket, nil, packet.seq); err != nil {
		log.Warnf("Failed acknowledge to message: %v", err)
	}

	log.Traceln("Acknowledge message")
}
