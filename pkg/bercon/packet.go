package bercon

import (
	"encoding/binary"
	"hash/crc32"
)

type packetKind byte

const (
	loginPacket packetKind = iota
	commandPacket
	messagePacket
	badPacket      packetKind = 0xFF
	firstByte      byte       = 0x42
	secondByte     byte       = 0x45
	lastByte       byte       = 0xFF
	loginSuccess   byte       = 0x01
	multipart      byte       = 0x00
	minPacketSize  int        = 8 // 7 header + 1 type + ... payload
	packetOverhead            = 9
)

/*
BattleEye RCON packet header
  - 0x42 0x45 | 0x00 0x00 0x00 0x00 | 0xFF -> BE CRC END
*/
type header struct {
	be  [2]byte // BE - magic bytes
	crc [4]byte // CRC sum of all data from 6 byte to end
	end byte    // Header end
}

/*
BattleEye RCON packet
  - header | 0x00 | ... -> login (type | data (status/pass))
  - header | 0x01 | 0x42 ... -> cmd single (type | seq, data (msg))
  - header | 0x01 | 0x43 0x00 0x06 0x00 ... -> cmd multi (type | seq, 0x00, pages, page#, data (msg))
  - header | 0x02 | 0x44 ... -> message (type | seq, data (msg))
*/
type packet struct {
	data   []byte     // data
	header header     // use in all packets
	kind   packetKind // define type of packet
	seq    byte       // packet sequence number (for command/message)
	pages  byte       // total pages (command multipart)
	page   byte       // page index (command multipart)
}

// make initializes fields; header/CRC are computed in toBytes().
func (p *packet) make(data []byte, kind packetKind, seq byte) {
	p.kind = kind
	p.data = data

	if kind != loginPacket {
		p.seq = seq
	}
}

// check CRC packet header
func (p *packet) checkCRC(raw []byte) error {
	got := binary.LittleEndian.Uint32(p.header.crc[:])
	want := crc32.ChecksumIEEE(raw[6:])
	if got != want {
		return ErrPacketCRC
	}

	return nil
}

// toBytes builds the packet with a single allocation and computes CRC over out[6:].
func (p *packet) toBytes() ([]byte, error) {
	// Compute length: 7(header) + 1(kind) + seq? + multipart hdr? + len(data)
	// header layout: [0]='B', [1]='E', [2..5]=CRC, [6]=0xFF
	extra := 0
	if p.kind != loginPacket {
		extra++ // seq
	}

	if p.kind == commandPacket && p.pages != 0 {
		extra += 3 // multipart header: 0x00, pages, page
	}

	total := 7 + 1 + extra + len(p.data)
	out := make([]byte, total)

	// magic header
	out[0] = firstByte
	out[1] = secondByte
	out[6] = lastByte

	// kind
	i := 7
	out[i] = byte(p.kind)
	i++

	// seq / multipart header
	if p.kind != loginPacket {
		out[i] = p.seq
		i++
	}

	if p.kind == commandPacket && p.pages != 0 {
		out[i+0] = multipart
		out[i+1] = p.pages
		out[i+2] = p.page
		i += 3
	}

	// payload
	copy(out[i:], p.data)

	// CRC over bytes from index 6 (0xFF) to the end
	sum := crc32.ChecksumIEEE(out[6:])
	binary.LittleEndian.PutUint32(out[2:6], sum)

	// quick header validation (cheap)
	if err := checkPacket(out); err != nil {
		return nil, err
	}

	return out, nil
}

// checkPacket validates fixed header bytes and minimum size.
func checkPacket(data []byte) error {
	if len(data) < minPacketSize { // 7 header + 1 kind
		return ErrPacketSize
	}

	if data[0] != firstByte || data[1] != secondByte || data[6] != lastByte {
		return ErrPacketHeader
	}

	return nil
}

// fromBytes parses the raw bytes into packet struct with minimal allocations.
// It copies only the payload into p.data; header and small fields are read by index.
func fromBytes(raw []byte) (*packet, error) {
	if err := checkPacket(raw); err != nil {
		return nil, err
	}

	// parse header
	p := new(packet)
	copy(p.header.be[:], raw[0:2])
	copy(p.header.crc[:], raw[2:6])
	p.header.end = raw[6]

	// verify CRC over raw[6:]
	if err := p.checkCRC(raw); err != nil {
		return nil, err
	}

	// parse kind and body
	i := 7
	p.kind = packetKind(raw[i])
	i++

	switch p.kind {
	case loginPacket:
		// payload is the rest

	case commandPacket:
		// seq
		if i >= len(raw) {
			return nil, ErrPacketSize
		}

		p.seq = raw[i]
		i++

		// optional multipart header
		// after kind(1)+seq(1), next byte may be 0x00 => multipart
		if i < len(raw) && raw[i] == multipart {
			if i+3 > len(raw) {
				return nil, ErrPacketSize
			}

			// consume delimiter + pages + page
			i++
			p.pages = raw[i]
			i++
			p.page = raw[i]
			i++
		}

	case messagePacket:
		// seq
		if i >= len(raw) {
			return nil, ErrPacketSize
		}
		p.seq = raw[i]
		i++

	default:
		return nil, ErrPacketUnknown
	}

	// copy payload (avoid retaining the large read buffer)
	if i > len(raw) {
		return nil, ErrPacketSize
	}

	n := len(raw) - i
	if n > 0 {
		p.data = make([]byte, n)
		copy(p.data, raw[i:])
	} else {
		p.data = nil
	}

	return p, nil
}
