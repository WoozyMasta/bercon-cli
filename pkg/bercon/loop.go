package bercon

import (
	"errors"
	"net"
	"sync/atomic"
	"time"
)

// managerLoop serializes state changes and all writes to UDP socket.
func (c *Connection) managerLoop() {
	defer c.wg.Done()
	tk := time.NewTicker(c.timeouts.keepalive)
	defer tk.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return

		case req := <-c.reqCh:
			seq, ok := c.nextFreeSeq(c.timeouts.deadline)
			if !ok {
				req.respCh <- sendResp{data: nil, err: ErrBufferFull}
				continue
			}

			holder := &inflight{done: req.respCh, ts: time.Now()}
			c.inflight[seq] = holder

			if err := c.writePacket(commandPacket, []byte(req.command), seq); err != nil {
				delete(c.inflight, seq)
				req.respCh <- sendResp{data: nil, err: err}
				continue
			}

		case pkt := <-c.pktCh:
			switch pkt.kind {
			case loginPacket:
				select {
				case c.msgCh <- pkt:
				default:
				}

			case messagePacket:
				select {
				case c.msgCh <- pkt:
				default:
				}

				// ack will be sent by manager
				select {
				case c.ackCh <- pkt.seq:
				default:
				}

			case commandPacket:
				c.handleCommandPacket(pkt)
			}

		case seq := <-c.ackCh:
			_ = c.writePacket(messagePacket, nil, seq)

		case <-tk.C:
			if c.keepalive {
				// fire-and-forget empty command to keep login alive.
				if seq, ok := c.tryFindFreeSeq(); ok {
					_ = c.writePacket(commandPacket, nil, seq)
				}
			}
		}
	}
}

func (c *Connection) tryFindFreeSeq() (byte, bool) {
	for i := 0; i < 256; i++ {
		s := c.sequence
		if _, busy := c.inflight[s]; !busy {
			c.sequence++
			return s, true
		}
		c.sequence++
	}

	return 0, false
}

// nextFreeSeq finds next sequence id that is not currently inflight.
func (c *Connection) nextFreeSeq(deadline time.Duration) (byte, bool) {
	exp := time.Now().Add(deadline)
	for {
		seq := c.sequence
		if _, busy := c.inflight[seq]; !busy {
			c.sequence++
			return seq, true
		}

		if time.Now().After(exp) {
			return 0, false
		}

		time.Sleep(c.timeouts.microSleep)
	}
}

// assemble multipart or complete single-part and reply to waiter
func (c *Connection) handleCommandPacket(pkt *packet) {
	holder, ok := c.inflight[pkt.seq]
	if !ok {
		return // stale/keepalive response; drop
	}

	// single-part
	if holder.pages == 0 && pkt.pages == 0 {
		delete(c.inflight, pkt.seq)
		holder.done <- sendResp{data: pkt.data, err: nil}
		return
	}

	// multipart assemble
	if holder.pages == 0 {
		holder.pages = pkt.pages
		holder.page = pkt.page
		holder.data = append(holder.data, pkt.data...)
	} else {
		if holder.page+1 != pkt.page {
			delete(c.inflight, pkt.seq)
			holder.done <- sendResp{data: nil, err: ErrBadSequence}
			return
		}

		holder.page = pkt.page
		holder.data = append(holder.data, pkt.data...)
	}

	if holder.pages == holder.page+1 {
		delete(c.inflight, pkt.seq)
		holder.done <- sendResp{data: holder.data, err: nil}
	}
}

// dispatchLoop handles buffering and sending events to the user
func (c *Connection) dispatchLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return

		case pkt := <-c.msgCh:
			select {
			case c.Messages <- PacketEvent{Time: time.Now(), Data: pkt.data, Seq: pkt.seq}:

			case <-c.ctx.Done():
				return
			}
		}
	}
}

// readerLoop reads UDP, parses packets and forwards to manager.
func (c *Connection) readerLoop() {
	defer c.wg.Done()
	defer c.cancel()

	buf := make([]byte, c.bufferSize)

	for {
		if c.conn == nil {
			return
		}

		_ = c.conn.SetReadDeadline(time.Now().Add(c.timeouts.deadline))
		n, err := c.conn.Read(buf)
		if err != nil {
			// normalize close/timeouts
			if errors.Is(err, net.ErrClosed) || c.ctx.Err() != nil {
				return
			}

			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue // deadline to periodically check ctx
			}

			// fatal read error: stop connection
			c.cancel()
			return
		}

		// update last activity
		atomic.StoreInt64(&c.lastActivity, time.Now().UnixNano())

		pkt, err := fromBytes(buf[:n])
		if err != nil {
			continue // bad packet – ignore
		}

		select {
		case c.pktCh <- pkt:
		case <-c.ctx.Done():
			return
		}
	}
}

// loginOnce performs synchronous login handshake before loops start.
func (c *Connection) loginOnce() error {
	if c.conn == nil {
		return ErrConnectionClosed
	}

	p := new(packet)
	p.make([]byte(c.password), loginPacket, 0)
	raw, err := p.toBytes()
	if err != nil {
		return err
	}

	if _, err := c.conn.Write(raw); err != nil {
		return err
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(c.timeouts.deadline))

	buf := make([]byte, c.bufferSize)
	n, err := c.conn.Read(buf)
	if err != nil {
		return err
	}

	resp, err := fromBytes(buf[:n])
	if err != nil || resp.kind != loginPacket {
		return ErrNotResponse
	}

	if len(resp.data) == 0 || resp.data[0] != loginSuccess {
		return ErrLoginFailed
	}

	_ = c.conn.SetReadDeadline(time.Time{})

	return nil
}

// writePacket constructs and sends a packet of the specified type, data and sequence to the server.
// NOTE: must be called only from managerLoop (single writer).
func (c *Connection) writePacket(kind packetKind, data []byte, seq byte) error {
	// Protocol-level limit for command body (client never sends multipart)
	if kind == commandPacket && len(data) > MaxCommandBodySize {
		return ErrCommandTooLong
	}

	//  guard against configured buffer smaller than packet
	if len(data) > int(c.bufferSize-packetOverhead) {
		return ErrBadSize
	}

	if c.conn == nil {
		return ErrConnectionClosed
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
