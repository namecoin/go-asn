package uper

import (
	"reflect"
	"testing"
)

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
