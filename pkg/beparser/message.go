package beparser

import "strings"

// All other responses
type Messages struct {
	Msg []string `json:"msg"`
}

func NewMessage() *Messages {
	return &Messages{}
}

func (m *Messages) Parse(data []byte) {
	lines := strings.Split(string(data), "\n")

	if len(lines) < 2 && len(lines[0]) == 0 {
		*m = Messages{Msg: []string{"OK"}}
	} else {
		*m = Messages{Msg: lines}
	}
}
