package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sigurn/crc16"
)

type MsgType uint8

var crc16Table = crc16.MakeTable(crc16.CRC16_MODBUS)

const (
	MsgTypeUnknown      MsgType = 0x0
	MsgTypeAnnounce     MsgType = 0x03
	MsgTypeData         MsgType = 0x04
	MsgTypePing         MsgType = 0x16
	MsgTypeConfig       MsgType = 0x18
	MsgTypeQuery        MsgType = 0x19
	MsgTypeReboot       MsgType = 0x20
	MsgTypeBufferedData MsgType = 0x50
)

// https://github.com/knowthelist/Growatt-server/blob/main/growatt_server.pl
type Msg struct {
	ID         uint16
	ProtocolID uint16
	Length     uint16
	UnitID     uint8
	Type       MsgType
	Payload    []byte
	CRC        uint16
}

func decode(data []byte, result *Msg) error {
	dec := binary.BigEndian
	l := len(data)
	result.ID = dec.Uint16(data[0:2])
	result.ProtocolID = dec.Uint16(data[2:4])
	result.Length = dec.Uint16(data[4:6])
	result.UnitID = data[6]
	result.Type = MsgType(data[7])
	result.Payload = data[8 : l-2]
	result.CRC = dec.Uint16(data[l-2 : l])

	// check if CRC is valid
	calculatedCRC := crc16.Checksum(data[0:l-2], crc16Table)
	if calculatedCRC != result.CRC {
		return fmt.Errorf("CRC mismatch: exp=%x received=%x", calculatedCRC, result.CRC)
	}

	// now xor data with mask
	for i := range result.Payload {
		result.Payload[i] = result.Payload[i] ^ mask[i%len(mask)]
	}

	return nil
}

// This is super helpful: https://github.com/johanmeijer/grott/blob/master/examples/Record%20Layout/t060104sph.json#L1
type rawData struct {
	Serial         [10]byte
	Unknown1       [20]byte    // offset 10
	Inverter       [10]byte    // offset 30
	Unknown2       [20]byte    // offset 40
	Year           uint8       // offset 60
	Month          uint8       // offset 61
	Day            uint8       // offset 62
	Hour           uint8       // offset 63
	Minute         uint8       // offset 64
	Second         uint8       // offset 65
	Unknown3       [6]byte     // offset 66
	Status         uint8       // offset 72
	PVPI           uint32      // offset 73
	PV1            Measurement // offset 77
	PV2            Measurement // offset 85
	PVPO           uint32      // offset 93
	PVFreq         uint16      // offset 97
	GridVoltage    uint16      // offset 99
	Unknown4       [16]byte    // offset 101
	PAC            uint32      // offset 117
	FAC            uint16      // offset 121
	AC1            Measurement // offset 123
	Unknown5A      [14]byte    // offset 131
	UnknownFreq    uint16      // offset 145
	UnknownVoltage uint16      // offset 147
	UnknownSOC     uint16      // offset 149
	Unknown5B      [18]byte    // offset 151
	EACToday       uint32      // offset 169
	Unknown6       [4]byte     // offset 173
	EACTotal       uint32      // offset 177
	Unknown7       [162]byte   // offset 181
	PDischarge     uint32      // offset 343
	PCharge        uint32      // offset 347
	VBat           uint16      // offset 351
	SOC            uint16      // offset 353
	PACToUserR     uint32      // offset 355
	Unknown8       [48]byte    // offset 359
	TempBat        uint16      // offset 407
}

type Data struct {
	Serial    string
	Inverter  string
	Timestamp time.Time

	Status      uint8
	PVPI        uint32 // also described as pb power in
	PV1         Measurement
	PV2         Measurement
	PVPO        float64 // also described as pb power out
	PVFreq      float64
	GridVoltage float64

	PAC uint32
	FAC uint16
	AC1 Measurement

	EACTotal uint32
	EACToday uint32

	Temp uint32

	PCharge    float64
	PDischarge float64
	VBat       float64
	SOC        float64
	TempBat    float64

	PACToUserR float64
}

type Measurement struct {
	Voltage uint16
	Current uint16
	Power   uint32
}

func decodeDataMessage(m *Msg, d *Data) error {
	var (
		raw rawData
	)

	reader := bytes.NewReader(m.Payload)
	if err := binary.Read(reader, binary.BigEndian, &raw); err != nil {
		return err
	}

	left, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	d.Serial = string(raw.Serial[:])
	d.Inverter = string(raw.Inverter[:])
	d.Timestamp = time.Date(
		int(raw.Year)+2000,
		time.Month(raw.Month),
		int(raw.Day),
		int(raw.Hour),
		int(raw.Minute),
		int(raw.Second),
		0,
		time.UTC,
	)

	d.Status = raw.Status
	d.PVPI = raw.PVPI
	d.PV1 = raw.PV1
	d.PV2 = raw.PV2
	d.PVPO = float64(raw.PVPO) * 0.1
	d.PVFreq = float64(raw.PVFreq) * 0.01
	d.GridVoltage = float64(raw.GridVoltage) * 0.1
	d.AC1 = raw.AC1
	d.PAC = raw.PAC
	d.FAC = raw.FAC
	d.EACToday = raw.EACToday
	d.EACTotal = raw.EACTotal

	d.PCharge = float64(raw.PCharge) * 0.1
	d.PDischarge = float64(raw.PDischarge) * 0.1
	d.VBat = float64(raw.VBat) * 0.1
	d.SOC = float64(raw.SOC) * 0.01
	d.TempBat = float64(raw.TempBat) * 0.1

	d.PACToUserR = float64(raw.PACToUserR) * 0.1

	if os.Getenv("DEBUG") != "" {
		fmt.Printf("unknown1=%02x\n", raw.Unknown1[:])
		fmt.Printf("unknown2=%02x\n", raw.Unknown2[:])
		fmt.Printf("unknown3=%02x\n", raw.Unknown3[:])
		fmt.Printf("unknown4=%02x\n", raw.Unknown4[:])
		fmt.Printf("unknown5B=%02x\n", raw.Unknown5B[:])
		fmt.Printf("unknown6=%02x\n", raw.Unknown6[:])
		fmt.Printf("unknown7=%02x\n", raw.Unknown7[:])
		fmt.Printf("unknownLeft=%02x\n", left)
		fmt.Printf("unknownLeft=%d", raw.PACToUserR)
	}

	return nil
}
