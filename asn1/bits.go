package asn1

import (
	"fmt"
	"reflect"
)

// BitWriter writes individual bits to a byte buffer.
// Bits are written MSB-first within each byte, following the ASN.1 PER convention.
type BitWriter struct {
	buf     []byte // The buffer containing the written bytes
	bitPos  int    // Current bit position (0-7 within the current byte)
	aligned bool   // If true, AlignToByte pads to a byte boundary (APER mode)
}

// NewBitWriter creates a new BitWriter.
// If aligned is true, AlignToByte will pad to byte boundaries (APER mode).
// If aligned is false, AlignToByte is a no-op (UPER mode).
func NewBitWriter(aligned bool) *BitWriter {
	return &BitWriter{
		buf:     make([]byte, 0, 64),
		aligned: aligned,
	}
}

// WriteBits writes the lower numBits of value to the buffer.
// numBits must be between 0 and 64 inclusive.
// Bits are written MSB-first.
func (w *BitWriter) WriteBits(value uint64, numBits int) error {
	if numBits < 0 || numBits > 64 {
		return fmt.Errorf("numBits %d out of range [0, 64]", numBits)
	}
	if numBits == 0 {
		return nil
	}

	// Mask to keep only the lower numBits
	if numBits < 64 {
		value &= (1 << numBits) - 1
	}

	// Write bits from MSB to LSB
	for i := numBits - 1; i >= 0; i-- {
		bit := (value >> i) & 1
		w.writeBit(byte(bit))
	}

	return nil
}

// writeBit writes a single bit (0 or 1) to the buffer.
func (w *BitWriter) writeBit(bit byte) {
	if w.bitPos == 0 {
		w.buf = append(w.buf, 0)
	}
	if bit == 1 {
		w.buf[len(w.buf)-1] |= 1 << (7 - w.bitPos)
	}
	w.bitPos = (w.bitPos + 1) % 8
}

// AlignToByte pads the buffer to a byte boundary.
// In aligned mode (APER), this writes zero bits until aligned.
// In unaligned mode (UPER), this is a no-op.
func (w *BitWriter) AlignToByte() {
	if !w.aligned {
		return
	}
	if w.bitPos != 0 {
		// Pad with zeros to the next byte boundary
		for w.bitPos != 0 {
			w.writeBit(0)
		}
	}
}

// Bytes returns the written data. If the last byte is partial,
// the remaining bits are zero-padded (they were initialised to zero).
func (w *BitWriter) Bytes() []byte {
	return w.buf
}

// BitPosition returns the total number of bits written.
func (w *BitWriter) BitPosition() int {
	if len(w.buf) == 0 {
		return 0
	}
	// If bitPos is 0, we've just completed a byte, so all bits are accounted for
	if w.bitPos == 0 {
		return len(w.buf) * 8
	}
	// Otherwise, the last byte is partial
	return (len(w.buf)-1)*8 + w.bitPos
}

// Reset clears the buffer for reuse.
func (w *BitWriter) Reset() {
	w.buf = w.buf[:0]
	w.bitPos = 0
}

// BitReader reads individual bits from a byte buffer.
// Bits are read MSB-first within each byte, following the ASN.1 PER convention.
type BitReader struct {
	data    []byte // The buffer containing the data to read
	bitPos  int    // Current bit position (absolute, across all bytes)
	aligned bool   // If true, AlignToByte skips to a byte boundary (APER mode)
}

// NewBitReader creates a new BitReader.
// If aligned is true, AlignToByte will skip to byte boundaries (APER mode).
// If aligned is false, AlignToByte is a no-op (UPER mode).
func NewBitReader(data []byte, aligned bool) *BitReader {
	return &BitReader{
		data:    data,
		aligned: aligned,
	}
}

// ReadBits reads numBits from the buffer and returns them as a uint64.
// numBits must be between 0 and 64 inclusive.
// Bits are read MSB-first.
func (r *BitReader) ReadBits(numBits int) (uint64, error) {
	if numBits < 0 || numBits > 64 {
		return 0, fmt.Errorf("numBits %d out of range [0, 64]", numBits)
	}
	if numBits == 0 {
		return 0, nil
	}

	if r.bitPos+numBits > len(r.data)*8 {
		return 0, fmt.Errorf("not enough bits: need %d, have %d", numBits, r.RemainingBits())
	}

	var value uint64
	for i := 0; i < numBits; i++ {
		byteIdx := r.bitPos / 8
		bitIdx := 7 - (r.bitPos % 8)
		bit := (r.data[byteIdx] >> bitIdx) & 1

		value = (value << 1) | uint64(bit)
		r.bitPos++
	}

	return value, nil
}

// AlignToByte skips bits until aligned to a byte boundary.
// In aligned mode (APER), this advances to the next byte.
// In unaligned mode (UPER), this is a no-op.
func (r *BitReader) AlignToByte() {
	if !r.aligned {
		return
	}
	remainder := r.bitPos % 8
	if remainder != 0 {
		r.bitPos += 8 - remainder
	}
}

// BitPosition returns the current bit position.
func (r *BitReader) BitPosition() int {
	return r.bitPos
}

// RemainingBits returns the number of unread bits.
func (r *BitReader) RemainingBits() int {
	return len(r.data)*8 - r.bitPos
}

var MixedRadixKinds = []reflect.Kind{
	reflect.Bool,
	reflect.Int,
	reflect.Int8,
	reflect.Int16,
	reflect.Int32,
	reflect.Int64,
	reflect.Uint,
	reflect.Uint8,
	reflect.Uint16,
	reflect.Uint32,
	reflect.Uint64,
}
