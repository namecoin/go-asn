package uper

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
		{"true", true, []byte{0x80}},   // 1 bit: 1, padded to 10000000
		{"false", false, []byte{0x00}}, // 1 bit: 0, padded to 00000000
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
		{"zero", 0, []byte{0x00}}, // 000 -> 00000000
		{"max", 7, []byte{0xE0}},  // 111 -> 11100000
		{"mid", 4, []byte{0x80}},  // 100 -> 10000000
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

func TestMarshal_ConstrainedIntOffset(t *testing.T) {
	type Message struct {
		Value int `asn1:"size:10..20"` // Range of 11, needs 4 bits, offset by 10
	}

	tests := []struct {
		name  string
		value int
		want  []byte
	}{
		{"min", 10, []byte{0x00}}, // 0 -> 0000
		{"max", 20, []byte{0xA0}}, // 10 -> 1010
		{"mid", 15, []byte{0x50}}, // 5 -> 0101
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

func TestMarshal_IntOutOfRange(t *testing.T) {
	type Message struct {
		Value int `asn1:"size:0..255"`
	}

	msg := Message{Value: 300}
	_, err := Marshal(msg)
	if err == nil {
		t.Error("Marshal() should return error for out-of-range value")
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

	// Preamble: 1 bit for optional field presence (1 = present)
	// Required: 1 bit (true)
	// Optional: 8 bits (42)
	// Total: 1 + 1 + 8 = 10 bits
	// 1 1 00101010 -> 11001010 10000000 -> 0xCA, 0x80
	want := []byte{0xCA, 0x80}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_OptionalAbsent(t *testing.T) {
	type Message struct {
		Required bool
		Optional *int `asn1:"optional,size:0..255"`
	}

	msg := Message{Required: true, Optional: nil}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Preamble: 1 bit for optional field presence (0 = absent)
	// Required: 1 bit (true)
	// Total: 1 + 1 = 2 bits
	// 0 1 -> 01000000 -> 0x40
	want := []byte{0x40}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_MultipleOptionals(t *testing.T) {
	type Message struct {
		A *bool `asn1:"optional"`
		B *int  `asn1:"optional,size:0..3"`
		C bool
	}

	aVal := true
	msg := Message{A: &aVal, B: nil, C: false}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Preamble: 2 bits (A present=1, B absent=0)
	// A value: 1 bit (true)
	// C value: 1 bit (false)
	// Total: 2 + 1 + 1 = 4 bits
	// 10 1 0 -> 10100000 -> 0xA0
	want := []byte{0xA0}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_Choice(t *testing.T) {
	// A CHOICE with 2 alternatives needs 1 bit for the index
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

		// Choice index 0 (1 bit) + bool true (1 bit) = 0 1 -> 01000000 -> 0x40
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

		// Choice index 1 (1 bit) + int 42 (8 bits) = 1 00101010 -> 10010101 0xxxxxxx -> 0x95 0x00
		want := []byte{0x95, 0x00}
		if !bytes.Equal(got, want) {
			t.Errorf("Marshal() = %X, want %X", got, want)
		}
	})
}

func TestMarshal_ChoiceThreeAlternatives(t *testing.T) {
	// A CHOICE with 3 alternatives needs 2 bits for the index (0-2 requires 2 bits)
	type Choice struct {
		A *bool   `asn1:"choice:0"`
		B *int    `asn1:"choice:1,size:0..7"`
		C *string `asn1:"choice:2,ia5string,size:2"`
	}

	t.Run("first alternative", func(t *testing.T) {
		val := false
		msg := Choice{A: &val}
		got, err := Marshal(msg)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		// Choice index 0 (2 bits: 00) + bool false (1 bit: 0) = 000 -> 00000000 -> 0x00
		want := []byte{0x00}
		if !bytes.Equal(got, want) {
			t.Errorf("Marshal() = %X, want %X", got, want)
		}
	})

	t.Run("second alternative", func(t *testing.T) {
		val := 5
		msg := Choice{B: &val}
		got, err := Marshal(msg)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		// Choice index 1 (2 bits: 01) + int 5 (3 bits: 101) = 01101 -> 01101000 -> 0x68
		want := []byte{0x68}
		if !bytes.Equal(got, want) {
			t.Errorf("Marshal() = %X, want %X", got, want)
		}
	})

	t.Run("third alternative", func(t *testing.T) {
		val := "OK"
		msg := Choice{C: &val}
		got, err := Marshal(msg)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		// Choice index 2 (2 bits: 10)
		// 'O' = 79 = 1001111 (7 bits), 'K' = 75 = 1001011 (7 bits)
		// 10 1001111 1001011 = 16 bits exactly (no padding needed)
		// = 1010 0111 1100 1011
		// = 0xA7 0xCB
		want := []byte{0xA7, 0xCB}
		if !bytes.Equal(got, want) {
			t.Errorf("Marshal() = %X, want %X", got, want)
		}
	})
}

func TestMarshal_ChoiceNoSelection(t *testing.T) {
	type Choice struct {
		A *bool `asn1:"choice:0"`
		B *int  `asn1:"choice:1,size:0..255"`
	}

	msg := Choice{A: nil, B: nil}
	_, err := Marshal(msg)
	if err == nil {
		t.Error("Marshal() should return error when no CHOICE alternative is selected")
	}
}

func TestMarshal_ChoiceMultipleSelections(t *testing.T) {
	type Choice struct {
		A *bool `asn1:"choice:0"`
		B *int  `asn1:"choice:1,size:0..255"`
	}

	a := true
	b := 42
	msg := Choice{A: &a, B: &b}
	_, err := Marshal(msg)
	if err == nil {
		t.Error("Marshal() should return error when multiple CHOICE alternatives are selected")
	}
}

func TestMarshal_SequenceOf(t *testing.T) {
	type Message struct {
		Values []int `asn1:"size:0..3"`
	}

	t.Run("empty", func(t *testing.T) {
		msg := Message{Values: []int{}}
		got, err := Marshal(msg)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		// Length = 0 (2 bits for range 0-3) = 00 -> 00000000 -> 0x00
		want := []byte{0x00}
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

	// Length: 3, offset from min (1) = 2. Range is 4 (1,2,3,4), needs 2 bits.
	// 2 in 2 bits = 10
	// Elements: true (1), false (0), true (1) = 1 0 1
	// Total: 10 101 -> 10101000 -> 0xA8
	want := []byte{0xA8}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_SequenceOfFixed(t *testing.T) {
	type Message struct {
		Flags []bool `asn1:"size:3"`
	}

	msg := Message{Flags: []bool{true, true, false}}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Fixed length (no length prefix)
	// Elements: true (1), true (1), false (0) = 1 1 0
	// Total: 110 -> 11000000 -> 0xC0
	want := []byte{0xC0}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}

func TestMarshal_SequenceOfLengthOutOfRange(t *testing.T) {
	type Message struct {
		Values []bool `asn1:"size:2..4"`
	}

	msg := Message{Values: []bool{true}} // Length 1, min is 2
	_, err := Marshal(msg)
	if err == nil {
		t.Error("Marshal() should return error for SEQUENCE OF length out of range")
	}
}

func TestMarshal_SequenceOfStruct(t *testing.T) {
	type Item struct {
		Flag  bool
		Value int `asn1:"size:0..7"`
	}

	type Message struct {
		Items []Item `asn1:"size:1..2"`
	}

	msg := Message{
		Items: []Item{
			{Flag: true, Value: 3},
			{Flag: false, Value: 5},
		},
	}
	got, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Length: 2, offset from min (1) = 1. Range is 2, needs 1 bit.
	// 1 in 1 bit = 1
	// Item 1: Flag=true (1), Value=3 (011 in 3 bits) = 1 011
	// Item 2: Flag=false (0), Value=5 (101 in 3 bits) = 0 101
	// Total: 1 1011 0101 -> 11011010 1xxxxxxx -> 0xDA 0x80
	want := []byte{0xDA, 0x80}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal() = %X, want %X", got, want)
	}
}
