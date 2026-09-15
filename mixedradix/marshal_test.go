package mixedradix

import (
	"testing"
)

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
