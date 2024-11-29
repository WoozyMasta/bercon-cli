package bercon

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
)

type packetKind byte

const (
	loginPacket packetKind = iota
	commandPacket
	messagePacket
	badPacket     packetKind = 0xFF
	firstByte     byte       = 0x42
	secondByte    byte       = 0x45
	lastByte      byte       = 0xFF
	loginSuccess  byte       = 0x01
	multipart     byte       = 0x00
	minPacketSize int        = 8 // 7 header + 1 type + ... payload
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
	header header     // use in all packets
	kind   packetKind // define type of packet
	seq    byte       // packet sequence number for command and message packet only
	pages  byte       // pages count for command packet only
	page   byte       // page number for command packet only
	data   []byte     // data
}

// make packet
func (p *packet) make(data []byte, kind packetKind, seq byte) {
	p.kind = kind
	p.data = data

	if kind != loginPacket {
		p.seq = seq
	}

	p.setHeader()
}

// get CRC for packet body
func (p *packet) getCRC() uint32 {
	var crcData []byte

	switch p.kind {
	case loginPacket:
		crcData = append([]byte{p.header.end, byte(p.kind)}, p.data...)

	case commandPacket:
		if p.pages == 0 {
			crcData = append([]byte{p.header.end, byte(p.kind), p.seq}, p.data...)
		} else {
			crcData = append([]byte{p.header.end, byte(p.kind), p.seq, multipart, p.pages, p.page}, p.data...)
		}

	case messagePacket:
		crcData = append([]byte{p.header.end, byte(p.kind), p.seq}, p.data...)
	}

	return crc32.ChecksumIEEE(crcData)
}

// check CRC packet header
func (p *packet) checkCRC() error {
	if binary.LittleEndian.Uint32(p.header.crc[:]) != p.getCRC() {
		return ErrPacketCRC
	}

	return nil
}

// set header for packet
func (p *packet) setHeader() {
	p.header.be = [2]byte{firstByte, secondByte}
	p.header.end = lastByte
	binary.LittleEndian.PutUint32(p.header.crc[:], p.getCRC())
}

// parse packet to bytes
func (p *packet) toBytes() ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := binary.LittleEndian

	if err := binary.Write(buf, enc, p.header.be); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, enc, p.header.crc); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, enc, p.header.end); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, enc, p.kind); err != nil {
		return nil, err
	}

	switch p.kind {
	case loginPacket:
		break

	case commandPacket:
		if err := binary.Write(buf, enc, p.seq); err != nil {
			return nil, err
		}
		if p.pages != 0 { // multipart packet processing
			if err := binary.Write(buf, enc, multipart); err != nil {
				return nil, err
			}
			if err := binary.Write(buf, enc, p.pages); err != nil {
				return nil, err
			}
			if err := binary.Write(buf, enc, p.page); err != nil {
				return nil, err
			}
		}

	case messagePacket:
		if err := binary.Write(buf, enc, p.seq); err != nil {
			return nil, err
		}

	default:
		return nil, ErrPacketUnknown
	}

	if err := binary.Write(buf, enc, p.data); err != nil {
		return nil, err
	}

	data := buf.Bytes()
	if err := checkPacket(data); err != nil {
		return nil, err
	}

	return data, nil
}

// check packet valid
func checkPacket(data []byte) error {
	if len(data) < minPacketSize { // 7 header + 1 type
		return ErrPacketSize
	}

	if data[0] != firstByte || data[1] != secondByte || data[6] != lastByte {
		return ErrPacketHeader
	}

	return nil
}

// make packet from bytes
func fromBytes(data []byte) (*packet, error) {
	if err := checkPacket(data); err != nil {
		return nil, err
	}

	p := new(packet)
	buf := bytes.NewReader(data)
	enc := binary.LittleEndian

	if err := binary.Read(buf, enc, &p.header.be); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, enc, &p.header.crc); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, enc, &p.header.end); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, enc, &p.kind); err != nil {
		return nil, err
	}

	switch p.kind {
	case loginPacket:
		break

	case commandPacket:
		if err := binary.Read(buf, enc, &p.seq); err != nil {
			return nil, err
		}
		if len(data) >= minPacketSize+2 && data[minPacketSize+1] == multipart { // multipart packet processing
			if _, err := buf.ReadByte(); err != nil { // read unused delimiter
				return nil, err
			}
			if err := binary.Read(buf, enc, &p.pages); err != nil {
				return nil, err
			}
			if err := binary.Read(buf, enc, &p.page); err != nil {
				return nil, err
			}
		}

	case messagePacket:
		if err := binary.Read(buf, enc, &p.seq); err != nil {
			return nil, err
		}

	default:
		return nil, ErrPacketUnknown
	}

	p.data = make([]byte, buf.Len())
	if err := binary.Read(buf, enc, &p.data); err != nil {
		return nil, err
	}

	if err := p.checkCRC(); err != nil {
		return nil, err
	}

	return p, nil
}
