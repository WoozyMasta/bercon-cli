package beparser

import "strings"

// Messages holds unstructured command output split into lines. It is
// returned by Parse for commands that do not have a dedicated parser.
type Messages struct {
	Msg []string `json:"msg"`
}

// NewMessage returns an empty Messages value.
func NewMessage() *Messages {
	return &Messages{}
}

// Parse splits arbitrary response data into lines. The BE server often
// replies with an empty line for successful actions; in that case "OK"
// is used as a single line placeholder.
func (m *Messages) Parse(data []byte) {
	*m = Messages{Msg: []string{}}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return
	}

	if len(lines) < 2 && len(lines[0]) == 0 {
		*m = Messages{Msg: []string{"OK"}}
	} else {
		*m = Messages{Msg: lines}
	}
}
