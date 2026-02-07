package aper

import (
	"bytes"
	"testing"
)

func TestMarshal_Bool(t *testing.T) {
	tests := []struct {
		name  string
		value bool
		want  []byte
	}{
		{"true", true, []byte{0x80}},
		{"false", false, []byte{0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(tt.value)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Marshal() = %X, want %X", got, tt.want)
			}
		})
	}
}

func TestMarshal_BoolInStruct(t *testing.T) {
	type Message struct {
		Flag1 bool
		Flag2 bool
	}

	msg := Message{Flag1: true, Flag2: false}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Two bools: 1, 0 = 10 padded to 10000000
	want := []byte{0x80}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_ConstrainedInt(t *testing.T) {
	type Message struct {
		Value int `asn1:"size:0..255"`
	}

	tests := []struct {
		name  string
		value int
		want  []byte
	}{
		{"zero", 0, []byte{0x00}},
		{"max", 255, []byte{0xFF}},
		{"mid", 128, []byte{0x80}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := Message{Value: tt.value}
			got, err := Marshal(msg)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Marshal() = %X, want %X", got, tt.want)
			}
		})
	}
}

func TestMarshal_ConstrainedIntSmallRange(t *testing.T) {
	type Message struct {
		Value int `asn1:"size:0..7"` // 3 bits needed
	}

	tests := []struct {
		name  string
		value int
		want  []byte
	}{
		{"zero", 0, []byte{0x00}},
		{"max", 7, []byte{0xE0}},
		{"mid", 4, []byte{0x80}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := Message{Value: tt.value}
			got, err := Marshal(msg)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Marshal() = %X, want %X", got, tt.want)
			}
		})
	}
}

func TestMarshal_LargeInt_Aligned(t *testing.T) {
	// Test that large integers (>8 bits) are byte-aligned in APER
	type Message struct {
		Flag  bool
		Value int `asn1:"size:0..65535"` // 16 bits, should be aligned
	}

	msg := Message{Flag: true, Value: 0x1234}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Flag: 1 (1 bit)
	// Align to byte boundary (7 zero bits)
	// Value: 0x1234 (16 bits) = 0x12 0x34
	// = 0x80 0x12 0x34
	want := []byte{0x80, 0x12, 0x34}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_OctetStringFixed(t *testing.T) {
	type Message struct {
		Data []byte `asn1:"size:4"`
	}

	msg := Message{Data: []byte{0xDE, 0xAD, 0xBE, 0xEF}}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Fixed size, data is aligned (already at byte boundary)
	want := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_OctetStringVariable_Aligned(t *testing.T) {
	// Test that OCTET STRING data is byte-aligned in APER
	type Message struct {
		Flag bool
		Data []byte `asn1:"size:0..255"`
	}

	msg := Message{Flag: true, Data: []byte{0xCA, 0xFE}}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Flag: 1 (1 bit)
	// Length: 2 (8 bits) = starts at bit 1
	// After length, we have 9 bits written, align to 16 bits (byte 2)
	// Data: 0xCA 0xFE (16 bits)
	//
	// Bit layout:
	// 1 00000010 0000000 (7 padding bits for alignment) 11001010 11111110
	// = 1000 0001 0000 0000 1100 1010 1111 1110
	// = 0x81 0x00 0xCA 0xFE
	want := []byte{0x81, 0x00, 0xCA, 0xFE}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_IA5String_Aligned(t *testing.T) {
	type Message struct {
		Flag bool
		Text string `asn1:"ia5string,size:0..255"`
	}

	msg := Message{Flag: true, Text: "AB"}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Flag: 1 (1 bit)
	// Length: 2 (8 bits)
	// Align to byte boundary (7 bits padding)
	// 'A' = 65 = 1000001 (7 bits), 'B' = 66 = 1000010 (7 bits)
	//
	// 1 00000010 (9 bits so far)
	// Padding: 0000000 (7 bits to reach 16)
	// A: 1000001 B: 1000010 (14 bits)
	// Total: 16 + 14 = 30 bits = 4 bytes (padded)
	//
	// 1000 0001 0000 0000 1000 0011 0000 10xx
	// = 0x81 0x00 0x83 0x08
	want := []byte{0x81, 0x00, 0x83, 0x08}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_OptionalPresent(t *testing.T) {
	type Message struct {
		Required bool
		Optional *int `asn1:"optional,size:0..255"`
	}

	value := 42
	msg := Message{Required: true, Optional: &value}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Preamble: 1 (present)
	// Required: 1 (true)
	// Optional: 42 (8 bits, already at byte boundary after 2 bits... wait)
	// Actually: 1 1 00101010 = 11 00101010 = 0xCA 0x80
	want := []byte{0xCA, 0x80}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_Choice(t *testing.T) {
	type Choice struct {
		A *bool `asn1:"choice:0"`
		B *int  `asn1:"choice:1,size:0..255"`
	}

	t.Run("first alternative", func(t *testing.T) {
		val := true
		msg := Choice{A: &val, B: nil}
		got, err := Marshal(msg)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		// Choice index 0 (1 bit) + bool true (1 bit) = 01
		want := []byte{0x40}
		if !bytes.Equal(got, want) {
			t.Errorf("Marshal() = %X, want %X", got, want)
		}
	})

	t.Run("second alternative", func(t *testing.T) {
		val := 42
		msg := Choice{A: nil, B: &val}
		got, err := Marshal(msg)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		// Choice index 1 (1 bit) + int 42 (8 bits)
		want := []byte{0x95, 0x00}
		if !bytes.Equal(got, want) {
			t.Errorf("Marshal() = %X, want %X", got, want)
		}
	})
}

func TestMarshal_SequenceOfBools(t *testing.T) {
	type Message struct {
		Flags []bool `asn1:"size:1..4"`
	}

	msg := Message{Flags: []bool{true, false, true}}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Length offset 2 (2 bits), then true, false, true
	// Same as UPER for bools (no alignment needed)
	want := []byte{0xA8}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}
