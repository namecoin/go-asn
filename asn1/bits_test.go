package asn1

import (
	"bytes"
	"testing"
)

func TestBitWriter_WriteBits(t *testing.T) {
	tests := []struct {
		name   string
		writes []struct {
			value uint64
			bits  int
		}
		want []byte
	}{
		{
			name: "a single byte",
			writes: []struct {
				value uint64
				bits  int
			}{
				{0b10110011, 8},
			},
			want: []byte{0xB3},
		},
		{
			name: "multiple small values",
			writes: []struct {
				value uint64
				bits  int
			}{
				{0b1, 1},    // 1
				{0b0, 1},    // 0
				{0b11, 2},   // 11
				{0b0000, 4}, // 0000
			},
			want: []byte{0b10110000}, // 1011 0000
		},
		{
			name: "crossing a byte boundary",
			writes: []struct {
				value uint64
				bits  int
			}{
				{0b1111, 4},
				{0b11110000, 8},
			},
			want: []byte{0xFF, 0x00},
		},
		{
			name: "a 16-bit value",
			writes: []struct {
				value uint64
				bits  int
			}{
				{0xABCD, 16},
			},
			want: []byte{0xAB, 0xCD},
		},
		{
			name: "write zero bits",
			writes: []struct {
				value uint64
				bits  int
			}{
				{0xFF, 0}, // Should write nothing
			},
			want: []byte{},
		},
		{
			name: "a 32-bit value",
			writes: []struct {
				value uint64
				bits  int
			}{
				{0xDEADBEEF, 32},
			},
			want: []byte{0xDE, 0xAD, 0xBE, 0xEF},
		},
		{
			name: "a partial last byte",
			writes: []struct {
				value uint64
				bits  int
			}{
				{0b101, 3},
			},
			want: []byte{0b10100000}, // 101 + 00000 padding
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewBitWriter(false) // unaligned
			for _, wr := range tt.writes {
				if err := w.WriteBits(wr.value, wr.bits); err != nil {
					t.Fatalf("WriteBits(%d, %d) error = %v", wr.value, wr.bits, err)
				}
			}
			got := w.Bytes()
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Bytes() = %X, want %X", got, tt.want)
			}
		})
	}
}

func TestBitWriter_WriteBits_Error(t *testing.T) {
	w := NewBitWriter(false)

	// Test numBits out of range (negative)
	if err := w.WriteBits(0, -1); err == nil {
		t.Error("WriteBits(0, -1) expected error, got nil")
	}

	// Test numBits out of range (too large)
	if err := w.WriteBits(0, 65); err == nil {
		t.Error("WriteBits(0, 65) expected error, got nil")
	}
}

func TestBitWriter_AlignToByte_Unaligned(t *testing.T) {
	w := NewBitWriter(false) // unaligned mode
	if err := w.WriteBits(0b111, 3); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}
	w.AlignToByte() // should be no-op in unaligned mode
	if err := w.WriteBits(0b1, 1); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}

	// In unaligned mode: 111 + 1 = 1111 (4 bits) = 0xF0
	got := w.Bytes()
	want := []byte{0xF0}
	if !bytes.Equal(got, want) {
		t.Errorf("Bytes() = %X, want %X", got, want)
	}
}

func TestBitWriter_AlignToByte_Aligned(t *testing.T) {
	w := NewBitWriter(true) // aligned mode
	if err := w.WriteBits(0b111, 3); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}
	w.AlignToByte() // should pad to byte boundary
	if err := w.WriteBits(0b1, 1); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}

	// In aligned mode: 111 + 00000 (pad) + 1 = 0xE0, 0x80
	got := w.Bytes()
	want := []byte{0xE0, 0x80}
	if !bytes.Equal(got, want) {
		t.Errorf("Bytes() = %X, want %X", got, want)
	}
}

func TestBitWriter_AlignToByte_AlreadyAligned(t *testing.T) {
	w := NewBitWriter(true) // aligned mode
	if err := w.WriteBits(0xFF, 8); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}
	w.AlignToByte() // should be no-op when already aligned
	if err := w.WriteBits(0xAA, 8); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}

	got := w.Bytes()
	want := []byte{0xFF, 0xAA}
	if !bytes.Equal(got, want) {
		t.Errorf("Bytes() = %X, want %X", got, want)
	}
}

func TestBitWriter_BitPosition(t *testing.T) {
	w := NewBitWriter(false)
	if w.BitPosition() != 0 {
		t.Errorf("BitPosition() = %d, want 0", w.BitPosition())
	}

	if err := w.WriteBits(0, 5); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}
	if w.BitPosition() != 5 {
		t.Errorf("BitPosition() = %d, want 5", w.BitPosition())
	}

	if err := w.WriteBits(0, 8); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}
	if w.BitPosition() != 13 {
		t.Errorf("BitPosition() = %d, want 13", w.BitPosition())
	}
}

func TestBitWriter_Reset(t *testing.T) {
	w := NewBitWriter(false)
	if err := w.WriteBits(0xFFFF, 16); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}

	if len(w.Bytes()) == 0 {
		t.Error("Bytes() should not be empty before Reset")
	}

	w.Reset()

	if len(w.Bytes()) != 0 {
		t.Errorf("Bytes() = %X, want empty after Reset", w.Bytes())
	}
	if w.BitPosition() != 0 {
		t.Errorf("BitPosition() = %d, want 0 after Reset", w.BitPosition())
	}
}

func TestBitWriter_64BitValue(t *testing.T) {
	w := NewBitWriter(false)
	// Write a 64-bit value
	if err := w.WriteBits(0x01_02_03_04_05_06_07_08, 64); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}

	got := w.Bytes()
	want := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	if !bytes.Equal(got, want) {
		t.Errorf("Bytes() = %X, want %X", got, want)
	}
}

func TestBitWriter_MasksExcessBits(t *testing.T) {
	w := NewBitWriter(false)
	// Write only the lower 4 bits of 0xFF (should write 0xF, not 0xFF)
	if err := w.WriteBits(0xFF, 4); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}

	got := w.Bytes()
	want := []byte{0xF0} // 1111 0000
	if !bytes.Equal(got, want) {
		t.Errorf("Bytes() = %X, want %X", got, want)
	}
}

// BitReader tests

func TestBitReader_ReadBits(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		reads []struct {
			bits int
			want uint64
		}
	}{
		{
			name: "a single byte",
			data: []byte{0xB3}, // 10110011
			reads: []struct {
				bits int
				want uint64
			}{
				{8, 0xB3},
			},
		},
		{
			name: "multiple small reads",
			data: []byte{0b10110000}, // 1011 0000
			reads: []struct {
				bits int
				want uint64
			}{
				{1, 1},
				{1, 0},
				{2, 3}, // 11
				{4, 0},
			},
		},
		{
			name: "crossing a byte boundary",
			data: []byte{0xFF, 0x00}, // 1111 1111 0000 0000
			reads: []struct {
				bits int
				want uint64
			}{
				{4, 0xF},
				{8, 0xF0},
				{4, 0x0},
			},
		},
		{
			name: "a 16-bit value",
			data: []byte{0xAB, 0xCD},
			reads: []struct {
				bits int
				want uint64
			}{
				{16, 0xABCD},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewBitReader(tt.data, false)
			for _, rd := range tt.reads {
				got, err := r.ReadBits(rd.bits)
				if err != nil {
					t.Fatalf("ReadBits(%d) error = %v", rd.bits, err)
				}
				if got != rd.want {
					t.Errorf("ReadBits(%d) = %X, want %X", rd.bits, got, rd.want)
				}
			}
		})
	}
}

func TestBitReader_ReadBits_EOF(t *testing.T) {
	r := NewBitReader([]byte{0xFF}, false)
	_, err := r.ReadBits(4)
	if err != nil {
		t.Fatalf("ReadBits(4) error = %v", err)
	}

	_, err = r.ReadBits(8) // Only 4 bits left
	if err == nil {
		t.Error("ReadBits(8) should return error when not enough bits")
	}
}

func TestBitReader_AlignToByte_Unaligned(t *testing.T) {
	r := NewBitReader([]byte{0xFF, 0xAA}, false)
	if _, err := r.ReadBits(3); err != nil {
		t.Fatalf("ReadBits: %v", err)
	}
	r.AlignToByte() // no-op in unaligned mode

	got, _ := r.ReadBits(5)
	// Position should still be at bit 3, so next 5 bits are 11111 (0x1F)
	if got != 0x1F {
		t.Errorf("ReadBits(5) = %X, want 1F", got)
	}
}

func TestBitReader_AlignToByte_Aligned(t *testing.T) {
	r := NewBitReader([]byte{0xFF, 0xAA}, true)
	if _, err := r.ReadBits(3); err != nil {
		t.Fatalf("ReadBits: %v", err)
	}
	r.AlignToByte() // should skip to byte boundary

	got, _ := r.ReadBits(8)
	// After alignment, should read second byte: 0xAA
	if got != 0xAA {
		t.Errorf("ReadBits(8) = %X, want AA", got)
	}
}

func TestBitReader_BitPosition(t *testing.T) {
	r := NewBitReader([]byte{0xFF, 0xFF}, false)
	if r.BitPosition() != 0 {
		t.Errorf("BitPosition() = %d, want 0", r.BitPosition())
	}

	if _, err := r.ReadBits(5); err != nil {
		t.Fatalf("ReadBits: %v", err)
	}
	if r.BitPosition() != 5 {
		t.Errorf("BitPosition() = %d, want 5", r.BitPosition())
	}

	if _, err := r.ReadBits(8); err != nil {
		t.Fatalf("ReadBits: %v", err)
	}
	if r.BitPosition() != 13 {
		t.Errorf("BitPosition() = %d, want 13", r.BitPosition())
	}
}

func TestBitReader_RemainingBits(t *testing.T) {
	r := NewBitReader([]byte{0xFF, 0xFF}, false) // 16 bits
	if r.RemainingBits() != 16 {
		t.Errorf("RemainingBits() = %d, want 16", r.RemainingBits())
	}

	if _, err := r.ReadBits(5); err != nil {
		t.Fatalf("ReadBits: %v", err)
	}
	if r.RemainingBits() != 11 {
		t.Errorf("RemainingBits() = %d, want 11", r.RemainingBits())
	}
}
