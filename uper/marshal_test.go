package uper

import (
	"bytes"
	"testing"
)

func TestMarshal_OctetStringFixed(t *testing.T) {
	type Message struct {
		Data []byte `asn1:"size:4"`
	}

	msg := Message{Data: []byte{0xDE, 0xAD, 0xBE, 0xEF}}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Fixed size: just the raw bytes (no length prefix needed)
	want := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_OctetStringVariable(t *testing.T) {
	type Message struct {
		Data []byte `asn1:"size:0..255"`
	}

	msg := Message{Data: []byte{0xCA, 0xFE}}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Variable size: length (8 bits for 0..255 range) + data
	// Length = 2 (offset from min 0), Data = 0xCAFE
	want := []byte{0x02, 0xCA, 0xFE}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_OctetStringLengthOutOfRange(t *testing.T) {
	type Message struct {
		Data []byte `asn1:"size:2..4"`
	}

	msg := Message{Data: []byte{0x01}} // Length 1, min is 2
	_, err := Marshal(msg)
	if err == nil {
		t.Error("Marshal() should return error for length out of range")
	}
}

func TestMarshal_OctetStringEmpty(t *testing.T) {
	type Message struct {
		Data []byte `asn1:"size:0..10"`
	}

	msg := Message{Data: []byte{}}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Empty data: length 0 (4 bits for 0..10 range = needs 4 bits)
	// 0 in 4 bits = 0000, padded to 00000000
	want := []byte{0x00}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_OctetStringVariableOffset(t *testing.T) {
	type Message struct {
		Data []byte `asn1:"size:2..5"`
	}

	// Length = 3, offset from min (2) = 1
	// Range is 4 (2,3,4,5), needs 2 bits for length
	// 1 in 2 bits = 01, then data ABC
	msg := Message{Data: []byte{0xAB, 0xCD, 0xEF}}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// 01 (length offset=1) + ABC DEF = 01 10101011 11001101 11101111
	// = 0110 1010 1111 0011 0111 1011 11xx xxxx
	// = 0x6A 0xF3 0x7B 0xC0
	want := []byte{0x6A, 0xF3, 0x7B, 0xC0}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_IA5String(t *testing.T) {
	type Message struct {
		Text string `asn1:"ia5string,size:0..255"`
	}

	msg := Message{Text: "ABC"}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Length (8 bits) + ASCII characters (7 bits each for IA5)
	// Length = 3
	// 'A' = 65 = 0x41, 'B' = 66 = 0x42, 'C' = 67 = 0x43
	// In 7-bit encoding: A=1000001, B=1000010, C=1000011
	// Byte layout:
	// Length: 00000011 (8 bits) = 0x03
	// 'A' (7 bits): 1000001
	// 'B' (7 bits): 1000010
	// 'C' (7 bits): 1000011
	// Total: 8 + 21 = 29 bits
	// Byte 0: 00000011 (length)
	// Byte 1: 1000001 1 (A + first bit of B)
	// Byte 2: 000010 10 (rest of B + first 2 bits of C)
	// Byte 3: 00011 000 (rest of C + padding)
	// = 0x03, 0x83, 0x0A, 0x18
	want := []byte{0x03, 0x83, 0x0A, 0x18}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_IA5StringFixed(t *testing.T) {
	type Message struct {
		Text string `asn1:"ia5string,size:2"`
	}

	msg := Message{Text: "OK"}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Fixed length, no length prefix
	// 'O' = 79 = 1001111, 'K' = 75 = 1001011
	// = 1001111 1001011 00 (padded)
	// = 0x9F, 0x2C
	want := []byte{0x9F, 0x2C}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_IA5StringInvalidChar(t *testing.T) {
	type Message struct {
		Text string `asn1:"ia5string,size:0..10"`
	}

	msg := Message{Text: "Hello\x80"} // 0x80 is not valid IA5 (>127)
	_, err := Marshal(msg)
	if err == nil {
		t.Error("Marshal() should return error for invalid IA5 character")
	}
}

func TestMarshal_UTF8String(t *testing.T) {
	type Message struct {
		Text string `asn1:"utf8string,size:0..255"`
	}

	msg := Message{Text: "Hi"}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// UTF8 uses 8 bits per byte
	// Length = 2, 'H' = 0x48, 'i' = 0x69
	want := []byte{0x02, 0x48, 0x69}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}
