package uper

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
		{"true", []byte{0x80}, true},   // 10000000 -> bit 0 is 1
		{"false", []byte{0x00}, false}, // 00000000 -> bit 0 is 0
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

	data := []byte{0x80} // 10 -> true, false
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

func TestUnmarshal_ConstrainedIntSmallRange(t *testing.T) {
	type Message struct {
		Value int `asn1:"size:0..7"` // 3 bits needed
	}

	tests := []struct {
		name string
		data []byte
		want int
	}{
		{"zero", []byte{0x00}, 0}, // 000 -> 0
		{"max", []byte{0xE0}, 7},  // 111 -> 7
		{"mid", []byte{0x80}, 4},  // 100 -> 4
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

func TestUnmarshal_ConstrainedIntOffset(t *testing.T) {
	type Message struct {
		Value int `asn1:"size:10..20"` // Range of 11, needs 4 bits, offset by 10
	}

	tests := []struct {
		name string
		data []byte
		want int
	}{
		{"min", []byte{0x00}, 10}, // 0000 -> 0 + 10 = 10
		{"max", []byte{0xA0}, 20}, // 1010 -> 10 + 10 = 20
		{"mid", []byte{0x50}, 15}, // 0101 -> 5 + 10 = 15
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

func TestUnmarshal_OctetStringVariable(t *testing.T) {
	type Message struct {
		Data []byte `asn1:"size:0..255"`
	}

	// Length = 2 (8 bits), Data = 0xCA 0xFE
	data := []byte{0x02, 0xCA, 0xFE}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := []byte{0xCA, 0xFE}
	if !reflect.DeepEqual(got.Data, want) {
		t.Errorf("Unmarshal() = %X, want %X", got.Data, want)
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

func TestUnmarshal_IA5String(t *testing.T) {
	type Message struct {
		Text string `asn1:"ia5string,size:0..255"`
	}

	// Marshal output for "ABC" - Length = 3, then 7 bits per char
	data := []byte{0x03, 0x83, 0x0A, 0x18}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Text != "ABC" {
		t.Errorf("Unmarshal() = %q, want %q", got.Text, "ABC")
	}
}

func TestUnmarshal_IA5StringFixed(t *testing.T) {
	type Message struct {
		Text string `asn1:"ia5string,size:2"`
	}

	// No length prefix, just 2 chars at 7 bits each
	data := []byte{0x9F, 0x2C}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Text != "OK" {
		t.Errorf("Unmarshal() = %q, want %q", got.Text, "OK")
	}
}

func TestUnmarshal_UTF8String(t *testing.T) {
	type Message struct {
		Text string `asn1:"utf8string,size:0..255"`
	}

	// Length = 2, 'H' = 0x48, 'i' = 0x69
	data := []byte{0x02, 0x48, 0x69}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Text != "Hi" {
		t.Errorf("Unmarshal() = %q, want %q", got.Text, "Hi")
	}
}

func TestUnmarshal_OptionalPresent(t *testing.T) {
	type Message struct {
		Required bool
		Optional *int `asn1:"optional,size:0..255"`
	}

	// Preamble: 1 (present), Required: 1, Optional: 42
	data := []byte{0xCA, 0x80}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Required != true {
		t.Errorf("Required = %v, want true", got.Required)
	}
	if got.Optional == nil {
		t.Error("Optional should not be nil")
	} else if *got.Optional != 42 {
		t.Errorf("Optional = %d, want 42", *got.Optional)
	}
}

func TestUnmarshal_OptionalAbsent(t *testing.T) {
	type Message struct {
		Required bool
		Optional *int `asn1:"optional,size:0..255"`
	}

	// Preamble: 0 (absent), Required: 1
	data := []byte{0x40}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Required != true {
		t.Errorf("Required = %v, want true", got.Required)
	}
	if got.Optional != nil {
		t.Errorf("Optional should be nil, got %d", *got.Optional)
	}
}

func TestUnmarshal_MultipleOptionals(t *testing.T) {
	type Message struct {
		A *bool `asn1:"optional"`
		B *int  `asn1:"optional,size:0..3"`
		C bool
	}

	// Preamble: 10 (A present, B absent), A=1, C=0
	data := []byte{0xA0}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.A == nil || *got.A != true {
		t.Errorf("A = %v, want true", got.A)
	}
	if got.B != nil {
		t.Errorf("B should be nil, got %d", *got.B)
	}
	if got.C != false {
		t.Errorf("C = %v, want false", got.C)
	}
}

func TestUnmarshal_Choice(t *testing.T) {
	type Choice struct {
		A *bool `asn1:"choice:0"`
		B *int  `asn1:"choice:1,size:0..255"`
	}

	t.Run("first alternative", func(t *testing.T) {
		// Choice index 0, bool true
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
		// Choice index 1, int 42
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

func TestUnmarshal_ChoiceThreeAlternatives(t *testing.T) {
	type Choice struct {
		A *bool   `asn1:"choice:0"`
		B *int    `asn1:"choice:1,size:0..7"`
		C *string `asn1:"choice:2,ia5string,size:2"`
	}

	t.Run("first alternative", func(t *testing.T) {
		// Choice index 0, bool false
		data := []byte{0x00}
		var got Choice
		err := Unmarshal(data, &got)
		if err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if got.A == nil || *got.A != false {
			t.Errorf("A = %v, want false", got.A)
		}
	})

	t.Run("second alternative", func(t *testing.T) {
		// Choice index 1, int 5
		data := []byte{0x68}
		var got Choice
		err := Unmarshal(data, &got)
		if err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if got.B == nil || *got.B != 5 {
			t.Errorf("B = %v, want 5", got.B)
		}
	})

	t.Run("third alternative", func(t *testing.T) {
		// Choice index 2, string "OK"
		data := []byte{0xA7, 0xCB}
		var got Choice
		err := Unmarshal(data, &got)
		if err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if got.C == nil || *got.C != "OK" {
			t.Errorf("C = %v, want OK", got.C)
		}
	})
}

func TestUnmarshal_SequenceOfBools(t *testing.T) {
	type Message struct {
		Flags []bool `asn1:"size:1..4"`
	}

	// Length offset 2 (actual length 3), then true, false, true
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

func TestUnmarshal_SequenceOfFixed(t *testing.T) {
	type Message struct {
		Flags []bool `asn1:"size:3"`
	}

	// No length, just 3 bools: true, true, false
	data := []byte{0xC0}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := []bool{true, true, false}
	if !reflect.DeepEqual(got.Flags, want) {
		t.Errorf("Flags = %v, want %v", got.Flags, want)
	}
}

func TestUnmarshal_SequenceOfStruct(t *testing.T) {
	type Item struct {
		Flag  bool
		Value int `asn1:"size:0..7"`
	}

	type Message struct {
		Items []Item `asn1:"size:1..2"`
	}

	// Length offset 1 (actual length 2), Item1: true, 3, Item2: false, 5
	data := []byte{0xDA, 0x80}
	var got Message
	err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := []Item{
		{Flag: true, Value: 3},
		{Flag: false, Value: 5},
	}
	if !reflect.DeepEqual(got.Items, want) {
		t.Errorf("Items = %+v, want %+v", got.Items, want)
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
		IA5     string `asn1:"ia5string,size:0..100"`
		UTF8    string `asn1:"utf8string,size:0..100"`
		Visible string `asn1:"visiblestring,size:0..100"`
	}

	orig := Message{
		IA5:     "Hello",
		UTF8:    "World",
		Visible: "Test",
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
