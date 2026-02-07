// Example: Comparing APER and UPER encoding
//
// Demonstrates the differences between Aligned (APER) and
// Unaligned (UPER) Packed Encoding Rules.
//
// Run with: go run main.go
package main

import (
	"fmt"
	"log"

	"github.com/shaneshort/go-asn/aper"
	"github.com/shaneshort/go-asn/uper"
)

// Message with various field types to show alignment differences
type Message struct {
	Flag     bool
	SmallInt int    `asn1:"size:0..7"`     // 3 bits
	LargeInt int    `asn1:"size:0..65535"` // 16 bits - APER will align this
	Data     []byte `asn1:"size:0..10"`    // Variable OCTET STRING
	Text     string `asn1:"ia5string,size:1..20"`
}

func main() {
	msg := Message{
		Flag:     true,
		SmallInt: 5,
		LargeInt: 0x1234,
		Data:     []byte{0xAB, 0xCD},
		Text:     "Hello",
	}

	fmt.Println("Original message:")
	fmt.Printf("  Flag:     %t\n", msg.Flag)
	fmt.Printf("  SmallInt: %d\n", msg.SmallInt)
	fmt.Printf("  LargeInt: 0x%04X (%d)\n", msg.LargeInt, msg.LargeInt)
	fmt.Printf("  Data:     %X\n", msg.Data)
	fmt.Printf("  Text:     %q\n", msg.Text)
	fmt.Println()

	// Encode with UPER
	uperData, err := uper.Marshal(msg)
	if err != nil {
		log.Fatalf("UPER encode failed: %v", err)
	}

	// Encode with APER
	aperData, err := aper.Marshal(msg)
	if err != nil {
		log.Fatalf("APER encode failed: %v", err)
	}

	fmt.Println("=== Encoding Comparison ===")
	fmt.Printf("UPER: %X (%d bytes)\n", uperData, len(uperData))
	fmt.Printf("APER: %X (%d bytes)\n", aperData, len(aperData))
	fmt.Printf("Difference: %d bytes\n", len(aperData)-len(uperData))
	fmt.Println()

	// Show bit-level layout
	fmt.Println("=== Bit-level Analysis ===")
	fmt.Println("UPER (Unaligned):")
	fmt.Println("  - All fields packed without padding")
	fmt.Println("  - Most compact representation")
	fmt.Println("  - Requires bit-level operations to parse")
	fmt.Println()
	fmt.Println("APER (Aligned):")
	fmt.Println("  - Aligns to byte boundary before:")
	fmt.Println("    * Large integers (>8 bits)")
	fmt.Println("    * OCTET STRING data")
	fmt.Println("    * String data")
	fmt.Println("  - Easier to parse (byte-aligned access)")
	fmt.Println("  - Slightly larger output")
	fmt.Println()

	// Verify both can be decoded correctly
	var decodedUper, decodedAper Message

	if err := uper.Unmarshal(uperData, &decodedUper); err != nil {
		log.Fatalf("UPER decode failed: %v", err)
	}

	if err := aper.Unmarshal(aperData, &decodedAper); err != nil {
		log.Fatalf("APER decode failed: %v", err)
	}

	fmt.Println("=== Verification ===")
	fmt.Printf("UPER round-trip: Flag=%t, SmallInt=%d, LargeInt=%d, Data=%X, Text=%q\n",
		decodedUper.Flag, decodedUper.SmallInt, decodedUper.LargeInt,
		decodedUper.Data, decodedUper.Text)
	fmt.Printf("APER round-trip: Flag=%t, SmallInt=%d, LargeInt=%d, Data=%X, Text=%q\n",
		decodedAper.Flag, decodedAper.SmallInt, decodedAper.LargeInt,
		decodedAper.Data, decodedAper.Text)

	// Size comparison with different data
	fmt.Println()
	fmt.Println("=== Size Comparison with Different Data ===")

	testCases := []Message{
		{Flag: true, SmallInt: 0, LargeInt: 0, Data: nil, Text: "A"},
		{Flag: false, SmallInt: 7, LargeInt: 255, Data: []byte{0xFF}, Text: "Test"},
		{Flag: true, SmallInt: 3, LargeInt: 65535, Data: []byte{1, 2, 3, 4, 5}, Text: "Longer string here"},
	}

	for i, tc := range testCases {
		u, _ := uper.Marshal(tc)
		a, _ := aper.Marshal(tc)
		fmt.Printf("  Case %d: UPER=%d bytes, APER=%d bytes (diff: %+d)\n",
			i+1, len(u), len(a), len(a)-len(u))
	}
}
