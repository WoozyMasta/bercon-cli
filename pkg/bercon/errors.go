package bercon

import (
	"errors"
)

var (
	ErrTimeout          = errors.New("deadline timeout reached")                      // deadline timeout reached
	ErrBufferFull       = errors.New("send command queue is full, try again later")   // send command queue is full, try again later
	ErrConnectionClosed = errors.New("connection closed unexpected")                  // connection closed unexpected
	ErrConnectionDown   = errors.New("connection to server is down, need reconnect")  // connection to server is down, need reconnect
	ErrReconnectFailed  = errors.New("failed to reconnect after several attempts")    // failed to reconnect after several attempts
	ErrPacketSize       = errors.New("packet size to small")                          // packet size to small
	ErrPacketHeader     = errors.New("packet header mismatched")                      // packet header mismatched
	ErrPacketCRC        = errors.New("CRC data not match")                            // CRC data not match
	ErrPacketUnknown    = errors.New("received unknown packet type")                  // received unknown packet type
	ErrNotResponse      = errors.New("server not response")                           // server not response
	ErrLoginFailed      = errors.New("login failed")                                  // login failed
	ErrNoLoginResponse  = errors.New("wait for login but get unexpected response")    // wait for login but get unexpected response
	ErrBadResponse      = errors.New("unexpected response data")                      // unexpected response data
	ErrBadSequence      = errors.New("returned not expected page number of sequence") // returned not expected page number of sequence
	ErrBadSize          = errors.New("size of buffer is greater than the allowed")    // size of buffer is greater than the allowed
	ErrBadPart          = errors.New("unexpected packet part returned")               // unexpected packet part returned
)
