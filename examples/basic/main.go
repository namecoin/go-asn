// Example: Basic UPER encoding and decoding
//
// Run with: go run main.go
package main

import (
	"fmt"
	"log"

	"github.com/shaneshort/go-asn/uper"
)

// Message demonstrates basic ASN.1 types
type Message struct {
	ID      int    `asn1:"size:0..255"`
	Name    string `asn1:"ia5string,size:1..64"`
	Active  bool
	Counter int `asn1:"size:0..65535"`
}

func main() {
	// Create a message
	original := Message{
		ID:      42,
		Name:    "Hello, ASN.1!",
		Active:  true,
		Counter: 12345,
	}

	fmt.Println("Original message:")
	fmt.Printf("  ID:      %d\n", original.ID)
	fmt.Printf("  Name:    %s\n", original.Name)
	fmt.Printf("  Active:  %t\n", original.Active)
	fmt.Printf("  Counter: %d\n", original.Counter)
	fmt.Println()

	// Encode to UPER
	encoded, err := uper.Marshal(original)
	if err != nil {
		log.Fatalf("Failed to encode: %v", err)
	}

	fmt.Printf("Encoded (%d bytes): %X\n", len(encoded), encoded)
	fmt.Println()

	// Decode back
	var decoded Message
	if err := uper.Unmarshal(encoded, &decoded); err != nil {
		log.Fatalf("Failed to decode: %v", err)
	}

	fmt.Println("Decoded message:")
	fmt.Printf("  ID:      %d\n", decoded.ID)
	fmt.Printf("  Name:    %s\n", decoded.Name)
	fmt.Printf("  Active:  %t\n", decoded.Active)
	fmt.Printf("  Counter: %d\n", decoded.Counter)

	// Verify round-trip
	if original == decoded {
		fmt.Println("\nRound-trip successful!")
	} else {
		fmt.Println("\nRound-trip FAILED!")
	}
}
