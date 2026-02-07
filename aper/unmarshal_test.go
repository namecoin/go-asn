package aper

import (
	"reflect"
	"testing"
)

func TestUnmarshal_Bool(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"true", []byte{0x80}, true},
		{"false", []byte{0x00}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool
			err := Unmarshal(tt.data, &got)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Unmarshal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnmarshal_BoolInStruct(t *testing.T) {
	type Message struct {
		Flag1 bool
		Flag2 bool
	}

	data := []byte{0x80}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := Message{Flag1: true, Flag2: false}
	if got != want {
		t.Errorf("Unmarshal() = %+v, want %+v", got, want)
	}
}

func TestUnmarshal_ConstrainedInt(t *testing.T) {
	type Message struct {
		Value int `asn1:"size:0..255"`
	}

	tests := []struct {
		name string
		data []byte
		want int
	}{
		{"zero", []byte{0x00}, 0},
		{"max", []byte{0xFF}, 255},
		{"mid", []byte{0x80}, 128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Message
			err := Unmarshal(tt.data, &got)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if got.Value != tt.want {
				t.Errorf("Unmarshal() = %d, want %d", got.Value, tt.want)
			}
		})
	}
}

func TestUnmarshal_LargeInt_Aligned(t *testing.T) {
	type Message struct {
		Flag  bool
		Value int `asn1:"size:0..65535"`
	}

	// Flag: 1 (1 bit), padding to byte, Value: 0x1234
	data := []byte{0x80, 0x12, 0x34}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Flag != true {
		t.Errorf("Flag = %v, want true", got.Flag)
	}
	if got.Value != 0x1234 {
		t.Errorf("Value = %X, want 1234", got.Value)
	}
}

func TestUnmarshal_OctetStringFixed(t *testing.T) {
	type Message struct {
		Data []byte `asn1:"size:4"`
	}

	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if !reflect.DeepEqual(got.Data, want) {
		t.Errorf("Unmarshal() = %X, want %X", got.Data, want)
	}
}

func TestUnmarshal_OctetStringVariable_Aligned(t *testing.T) {
	type Message struct {
		Flag bool
		Data []byte `asn1:"size:0..255"`
	}

	// Flag: 1, Length: 2, padding, Data: 0xCA 0xFE
	data := []byte{0x81, 0x00, 0xCA, 0xFE}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Flag != true {
		t.Errorf("Flag = %v, want true", got.Flag)
	}
	want := []byte{0xCA, 0xFE}
	if !reflect.DeepEqual(got.Data, want) {
		t.Errorf("Data = %X, want %X", got.Data, want)
	}
}

func TestUnmarshal_IA5String_Aligned(t *testing.T) {
	type Message struct {
		Flag bool
		Text string `asn1:"ia5string,size:0..255"`
	}

	// Flag: 1, Length: 2, padding, "AB"
	data := []byte{0x81, 0x00, 0x83, 0x08}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Flag != true {
		t.Errorf("Flag = %v, want true", got.Flag)
	}
	if got.Text != "AB" {
		t.Errorf("Text = %q, want %q", got.Text, "AB")
	}
}

func TestUnmarshal_OptionalPresent(t *testing.T) {
	type Message struct {
		Required bool
		Optional *int `asn1:"optional,size:0..255"`
	}

	data := []byte{0xCA, 0x80}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Required != true {
		t.Errorf("Required = %v, want true", got.Required)
	}
	if got.Optional == nil || *got.Optional != 42 {
		t.Errorf("Optional = %v, want 42", got.Optional)
	}
}

func TestUnmarshal_Choice(t *testing.T) {
	type Choice struct {
		A *bool `asn1:"choice:0"`
		B *int  `asn1:"choice:1,size:0..255"`
	}

	t.Run("first alternative", func(t *testing.T) {
		data := []byte{0x40}
		var got Choice
		err := Unmarshal(data, &got)
		if err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if got.A == nil || *got.A != true {
			t.Errorf("A = %v, want true", got.A)
		}
		if got.B != nil {
			t.Error("B should be nil")
		}
	})

	t.Run("second alternative", func(t *testing.T) {
		data := []byte{0x95, 0x00}
		var got Choice
		err := Unmarshal(data, &got)
		if err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if got.A != nil {
			t.Error("A should be nil")
		}
		if got.B == nil || *got.B != 42 {
			t.Errorf("B = %v, want 42", got.B)
		}
	})
}

func TestUnmarshal_SequenceOfBools(t *testing.T) {
	type Message struct {
		Flags []bool `asn1:"size:1..4"`
	}

	data := []byte{0xA8}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := []bool{true, false, true}
	if !reflect.DeepEqual(got.Flags, want) {
		t.Errorf("Flags = %v, want %v", got.Flags, want)
	}
}

// Round-trip tests for APER
func TestRoundTrip_Bool(t *testing.T) {
	tests := []bool{true, false}

	for _, orig := range tests {
		data, err := Marshal(orig)
		if err != nil {
			t.Fatalf("Marshal(%v) error = %v", orig, err)
		}

		var got bool
		err = Unmarshal(data, &got)
		if err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if got != orig {
			t.Errorf("Round-trip failed: got %v, want %v", got, orig)
		}
	}
}

func TestRoundTrip_LargeInt(t *testing.T) {
	type Message struct {
		Flag  bool
		Value int `asn1:"size:0..65535"`
	}

	orig := Message{Flag: true, Value: 0x1234}
	data, err := Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Message
	err = Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got != orig {
		t.Errorf("Round-trip failed: got %+v, want %+v", got, orig)
	}
}

func TestRoundTrip_OctetString(t *testing.T) {
	type Message struct {
		Flag bool
		Data []byte `asn1:"size:0..255"`
	}

	orig := Message{Flag: true, Data: []byte{0xDE, 0xAD, 0xBE, 0xEF}}
	data, err := Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Message
	err = Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Flag != orig.Flag || !reflect.DeepEqual(got.Data, orig.Data) {
		t.Errorf("Round-trip failed: got %+v, want %+v", got, orig)
	}
}

func TestRoundTrip_String(t *testing.T) {
	type Message struct {
		Flag bool
		Text string `asn1:"ia5string,size:0..255"`
	}

	orig := Message{Flag: true, Text: "Hello, World!"}
	data, err := Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Message
	err = Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got != orig {
		t.Errorf("Round-trip failed: got %+v, want %+v", got, orig)
	}
}

func TestRoundTrip_Optional(t *testing.T) {
	type Message struct {
		Required bool
		Optional *int `asn1:"optional,size:0..255"`
	}

	t.Run("present", func(t *testing.T) {
		val := 42
		orig := Message{Required: true, Optional: &val}
		data, err := Marshal(orig)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		var got Message
		err = Unmarshal(data, &got)
		if err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if got.Required != orig.Required {
			t.Errorf("Required = %v, want %v", got.Required, orig.Required)
		}
		if got.Optional == nil || *got.Optional != *orig.Optional {
			t.Errorf("Optional = %v, want %v", got.Optional, *orig.Optional)
		}
	})

	t.Run("absent", func(t *testing.T) {
		orig := Message{Required: true, Optional: nil}
		data, err := Marshal(orig)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		var got Message
		err = Unmarshal(data, &got)
		if err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if got.Required != orig.Required {
			t.Errorf("Required = %v, want %v", got.Required, orig.Required)
		}
		if got.Optional != nil {
			t.Errorf("Optional should be nil, got %d", *got.Optional)
		}
	})
}

func TestRoundTrip_Choice(t *testing.T) {
	type Choice struct {
		A *bool `asn1:"choice:0"`
		B *int  `asn1:"choice:1,size:0..255"`
	}

	t.Run("first alternative", func(t *testing.T) {
		val := true
		orig := Choice{A: &val}
		data, err := Marshal(orig)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		var got Choice
		err = Unmarshal(data, &got)
		if err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if got.A == nil || *got.A != *orig.A {
			t.Errorf("A = %v, want %v", got.A, *orig.A)
		}
		if got.B != nil {
			t.Error("B should be nil")
		}
	})

	t.Run("second alternative", func(t *testing.T) {
		val := 42
		orig := Choice{B: &val}
		data, err := Marshal(orig)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		var got Choice
		err = Unmarshal(data, &got)
		if err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if got.A != nil {
			t.Error("A should be nil")
		}
		if got.B == nil || *got.B != *orig.B {
			t.Errorf("B = %v, want %v", got.B, *orig.B)
		}
	})
}

func TestRoundTrip_SequenceOf(t *testing.T) {
	type Item struct {
		Flag  bool
		Value int `asn1:"size:0..7"`
	}

	type Message struct {
		Items []Item `asn1:"size:1..4"`
	}

	orig := Message{
		Items: []Item{
			{Flag: true, Value: 3},
			{Flag: false, Value: 5},
		},
	}
	data, err := Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Message
	err = Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if !reflect.DeepEqual(got, orig) {
		t.Errorf("Round-trip failed: got %+v, want %+v", got, orig)
	}
}
