package uper

import (
	"reflect"
	"testing"
)

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

func TestUnmarshal_OctetStringEmpty(t *testing.T) {
	type Message struct {
		Data []byte `asn1:"size:0..10"`
	}

	// Length = 0 (4 bits for range 0..10)
	data := []byte{0x00}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(got.Data) != 0 {
		t.Errorf("Unmarshal() = %X, want empty", got.Data)
	}
}

// Round-trip tests - marshal then unmarshal should return the original value
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

func TestRoundTrip_Struct(t *testing.T) {
	type Message struct {
		Flag  bool
		Value int `asn1:"size:0..255"`
	}

	orig := Message{Flag: true, Value: 42}
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

func TestRoundTrip_Strings(t *testing.T) {
	type Message struct {
		IA5       string `asn1:"ia5string,size:0..100"`
		UTF8      string `asn1:"utf8string,size:0..100"`
		Visible   string `asn1:"visiblestring,size:0..100"`
		Printable string `asn1:"printablestring,size:0..100"`
	}

	orig := Message{
		IA5:       "Hello",
		UTF8:      "World",
		Visible:   "Test",
		Printable: "Meow69+",
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

	if got != orig {
		t.Errorf("Round-trip failed: got %+v, want %+v", got, orig)
	}
}
