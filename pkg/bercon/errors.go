package bercon

import (
	"errors"
)

var (
	// ErrTimeout is returned when a request exceeded its deadline.
	ErrTimeout = errors.New("deadline timeout reached")

	// ErrLoginTimeout is returned when login request exceeded its deadline.
	ErrLoginTimeout = errors.New("login deadline timeout reached")

	// ErrBufferFull is returned when the send command queue is full.
	ErrBufferFull = errors.New("send command queue is full, try again later")

	// ErrConnectionClosed is returned when the connection was closed unexpectedly.
	ErrConnectionClosed = errors.New("connection closed unexpected")

	// ErrConnectionDown indicates that the connection to the server is down
	// and a reconnect is required.
	ErrConnectionDown = errors.New("connection to server is down, need reconnect")

	// ErrReconnectFailed is returned when automatic reconnect attempts failed.
	ErrReconnectFailed = errors.New("failed to reconnect after several attempts")

	// ErrPacketSize is returned when the packet size is too small.
	ErrPacketSize = errors.New("packet size to small")

	// ErrPacketHeader is returned when the packet header is invalid.
	ErrPacketHeader = errors.New("packet header mismatched")

	// ErrPacketCRC is returned when the CRC check failed.
	ErrPacketCRC = errors.New("CRC mismatch")

	// ErrPacketUnknown is returned when an unknown packet type is received.
	ErrPacketUnknown = errors.New("received unknown packet type")

	// ErrNotResponse is returned when the server did not send a response.
	ErrNotResponse = errors.New("server not response")

	// ErrLoginFailed is returned when login to the BattlEye server fails.
	ErrLoginFailed = errors.New("login failed")

	// ErrNoLoginResponse is returned when login response was not received
	// or was unexpected.
	ErrNoLoginResponse = errors.New("wait for login but get unexpected response")

	// ErrBadResponse is returned when the response data is not valid.
	ErrBadResponse = errors.New("unexpected response data")

	// ErrBadSequence is returned when a response contains an unexpected
	// sequence or page number.
	ErrBadSequence = errors.New("returned not expected page number of sequence")

	// ErrBadSize is returned when the buffer size exceeds the allowed limit.
	ErrBadSize = errors.New("size of buffer is greater than the allowed")

	// ErrBadPart is returned when a multipart packet contains an unexpected part.
	ErrBadPart = errors.New("unexpected packet part returned")

	// ErrCommandTooLong is returned when the client command exceeds
	// the maximum allowed length by the protocol.
	ErrCommandTooLong = errors.New("command too long")

	// ErrReconnectWindow is returned when the maximum reconnect wait window
	// has been exceeded.
	ErrReconnectWindow = errors.New("reconnect window exceeded")
)
