package asn1

import (
	"reflect"
	"testing"
)

func TestParseTag_Empty(t *testing.T) {
	opts, err := ParseTag("")
	if err != nil {
		t.Fatalf("ParseTag() error = %v", err)
	}

	if opts.Optional {
		t.Error("Optional should be false")
	}
	if opts.Tag != nil {
		t.Error("Tag should be nil")
	}
}

func TestParseTag_Optional(t *testing.T) {
	opts, err := ParseTag("optional")
	if err != nil {
		t.Fatalf("ParseTag() error = %v", err)
	}

	if !opts.Optional {
		t.Error("Optional should be true")
	}
}

func TestParseTag_Tag(t *testing.T) {
	opts, err := ParseTag("tag:5")
	if err != nil {
		t.Fatalf("ParseTag() error = %v", err)
	}

	if opts.Tag == nil || *opts.Tag != 5 {
		t.Errorf("Tag = %v, want 5", opts.Tag)
	}
}

func TestParseTag_Size(t *testing.T) {
	opts, err := ParseTag("size:0..255")
	if err != nil {
		t.Fatalf("ParseTag() error = %v", err)
	}

	if opts.SizeMin == nil || *opts.SizeMin != 0 {
		t.Errorf("SizeMin = %v, want 0", opts.SizeMin)
	}
	if opts.SizeMax == nil || *opts.SizeMax != 255 {
		t.Errorf("SizeMax = %v, want 255", opts.SizeMax)
	}
}

func TestParseTag_SizeSingleValue(t *testing.T) {
	opts, err := ParseTag("size:8")
	if err != nil {
		t.Fatalf("ParseTag() error = %v", err)
	}

	if opts.SizeMin == nil || *opts.SizeMin != 8 {
		t.Errorf("SizeMin = %v, want 8", opts.SizeMin)
	}
	if opts.SizeMax == nil || *opts.SizeMax != 8 {
		t.Errorf("SizeMax = %v, want 8", opts.SizeMax)
	}
}

func TestParseTag_StringTypes(t *testing.T) {
	tests := []struct {
		tag  string
		want StringType
	}{
		{"ia5string", StringTypeIA5},
		{"utf8string", StringTypeUTF8},
		{"printablestring", StringTypePrintable},
		{"visiblestring", StringTypeVisible},
	}

	for _, tt := range tests {
		opts, err := ParseTag(tt.tag)
		if err != nil {
			t.Fatalf("ParseTag(%q) error = %v", tt.tag, err)
		}
		if opts.StringType != tt.want {
			t.Errorf("ParseTag(%q).StringType = %v, want %v", tt.tag, opts.StringType, tt.want)
		}
	}
}

func TestParseTag_Choice(t *testing.T) {
	opts, err := ParseTag("choice:2")
	if err != nil {
		t.Fatalf("ParseTag() error = %v", err)
	}

	if opts.Choice == nil || *opts.Choice != 2 {
		t.Errorf("Choice = %v, want 2", opts.Choice)
	}
}

func TestParseTag_Default(t *testing.T) {
	opts, err := ParseTag("default:42")
	if err != nil {
		t.Fatalf("ParseTag() error = %v", err)
	}

	if opts.Default == nil || !reflect.DeepEqual(opts.Default, int64(42)) {
		t.Errorf("Default = %v, want 42", opts.Default)
	}
}

func TestParseTag_Extensible(t *testing.T) {
	opts, err := ParseTag("extensible")
	if err != nil {
		t.Fatalf("ParseTag() error = %v", err)
	}

	if !opts.Extensible {
		t.Error("Extensible should be true")
	}
}

func TestParseTag_Multiple(t *testing.T) {
	opts, err := ParseTag("optional,tag:0,size:1..100,ia5string")
	if err != nil {
		t.Fatalf("ParseTag() error = %v", err)
	}

	if !opts.Optional {
		t.Error("Optional should be true")
	}
	if opts.Tag == nil || *opts.Tag != 0 {
		t.Errorf("Tag = %v, want 0", opts.Tag)
	}
	if opts.SizeMin == nil || *opts.SizeMin != 1 {
		t.Errorf("SizeMin = %v, want 1", opts.SizeMin)
	}
	if opts.SizeMax == nil || *opts.SizeMax != 100 {
		t.Errorf("SizeMax = %v, want 100", opts.SizeMax)
	}
	if opts.StringType != StringTypeIA5 {
		t.Errorf("StringType = %v, want IA5", opts.StringType)
	}
}

func TestParseTag_InvalidSize(t *testing.T) {
	_, err := ParseTag("size:abc")
	if err == nil {
		t.Error("ParseTag() should return error for invalid size")
	}
}

func TestParseTag_InvalidTag(t *testing.T) {
	_, err := ParseTag("tag:notanumber")
	if err == nil {
		t.Error("ParseTag() should return error for invalid tag")
	}
}
