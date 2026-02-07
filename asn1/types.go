package asn1

// BitString represents an ASN.1 BIT STRING.
// Bits are indexed from 0 (MSB of the first byte) to BitLength-1.
// This follows the ASN.1 convention where bit 0 is the most significant bit.
type BitString struct {
	Bytes     []byte // The raw bytes containing the bits
	BitLength int    // The number of valid bits (may be less than len(Bytes)*8)
}

// NewBitString creates a BitString with the specified number of bits,
// all initialised to zero.
func NewBitString(bitLength int) BitString {
	byteLen := (bitLength + 7) / 8
	return BitString{
		Bytes:     make([]byte, byteLen),
		BitLength: bitLength,
	}
}

// Bit returns the value of the bit at the given index.
// Index 0 is the MSB of the first byte. Returns false if the index is out of range.
func (bs *BitString) Bit(index int) bool {
	if index < 0 || index >= bs.BitLength {
		return false
	}
	byteIndex := index / 8
	bitIndex := 7 - (index % 8) // MSB is bit 7 within the byte
	return (bs.Bytes[byteIndex] & (1 << bitIndex)) != 0
}

// SetBit sets the value of the bit at the given index.
// Index 0 is the MSB of the first byte. Does nothing if the index is out of range.
func (bs *BitString) SetBit(index int, value bool) {
	if index < 0 || index >= bs.BitLength {
		return
	}
	byteIndex := index / 8
	bitIndex := 7 - (index % 8)
	if value {
		bs.Bytes[byteIndex] |= 1 << bitIndex
	} else {
		bs.Bytes[byteIndex] &^= 1 << bitIndex
	}
}
