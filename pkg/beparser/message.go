package beparser

import "strings"

// All other responses
type Messages struct {
	Msg []string `json:"msg"`
}

// Create new Messages struct
func NewMessage() *Messages {
	return &Messages{}
}

// Split other not parsable response to lines in Messages struct
func (m *Messages) Parse(data []byte) {
	lines := strings.Split(string(data), "\n")

	if len(lines) < 2 && len(lines[0]) == 0 {
		*m = Messages{Msg: []string{"OK"}}
	} else {
		*m = Messages{Msg: lines}
	}
}
