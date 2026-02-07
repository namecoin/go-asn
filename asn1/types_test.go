package asn1

import "testing"

func TestBitString_Bit(t *testing.T) {
	// 0b10110000 = 0xB0
	bs := BitString{Bytes: []byte{0xB0}, BitLength: 8}

	tests := []struct {
		index int
		want  bool
	}{
		{0, true}, // MSB
		{1, false},
		{2, true},
		{3, true},
		{4, false},
		{5, false},
		{6, false},
		{7, false},
	}

	for _, tt := range tests {
		if got := bs.Bit(tt.index); got != tt.want {
			t.Errorf("Bit(%d) = %v, want %v", tt.index, got, tt.want)
		}
	}
}

func TestBitString_SetBit(t *testing.T) {
	bs := BitString{Bytes: []byte{0x00}, BitLength: 8}

	bs.SetBit(0, true)
	bs.SetBit(2, true)
	bs.SetBit(3, true)

	if bs.Bytes[0] != 0xB0 {
		t.Errorf("Bytes = %02X, want B0", bs.Bytes[0])
	}

	bs.SetBit(0, false)
	if bs.Bytes[0] != 0x30 {
		t.Errorf("Bytes = %02X, want 30", bs.Bytes[0])
	}
}

func TestBitString_BitOutOfRange(t *testing.T) {
	bs := BitString{Bytes: []byte{0xFF}, BitLength: 4}

	// Accessing beyond BitLength should return false
	if bs.Bit(4) != false {
		t.Error("Bit(4) should return false when BitLength is 4")
	}
	if bs.Bit(100) != false {
		t.Error("Bit(100) should return false")
	}
}

func TestNewBitString(t *testing.T) {
	bs := NewBitString(12)

	if bs.BitLength != 12 {
		t.Errorf("BitLength = %d, want 12", bs.BitLength)
	}
	if len(bs.Bytes) != 2 {
		t.Errorf("len(Bytes) = %d, want 2", len(bs.Bytes))
	}
}
