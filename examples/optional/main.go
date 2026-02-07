// Example: Optional fields in ASN.1
//
// Demonstrates how to use pointer types for OPTIONAL fields.
//
// Run with: go run main.go
package main

import (
	"fmt"
	"log"

	"github.com/shaneshort/go-asn/uper"
)

// Config demonstrates optional fields
type Config struct {
	Name        string  `asn1:"ia5string,size:1..32"`
	Port        int     `asn1:"size:1..65535"`
	Timeout     *int    `asn1:"optional,size:1..3600"` // Optional: 1-3600 seconds
	MaxRetries  *int    `asn1:"optional,size:0..10"`   // Optional: 0-10 retries
	Description *string `asn1:"optional,ia5string,size:1..256"`
}

// Helper to create pointer to int
func intPtr(v int) *int {
	return &v
}

// Helper to create pointer to string
func strPtr(v string) *string {
	return &v
}

func main() {
	fmt.Println("=== Example 1: All optional fields present ===")
	config1 := Config{
		Name:        "server1",
		Port:        8080,
		Timeout:     intPtr(30),
		MaxRetries:  intPtr(3),
		Description: strPtr("Main application server"),
	}
	encodeAndDecode(config1)

	fmt.Println("\n=== Example 2: Some optional fields absent ===")
	config2 := Config{
		Name:        "server2",
		Port:        443,
		Timeout:     intPtr(60),
		MaxRetries:  nil, // Absent
		Description: nil, // Absent
	}
	encodeAndDecode(config2)

	fmt.Println("\n=== Example 3: All optional fields absent ===")
	config3 := Config{
		Name:        "minimal",
		Port:        80,
		Timeout:     nil,
		MaxRetries:  nil,
		Description: nil,
	}
	encodeAndDecode(config3)
}

func encodeAndDecode(original Config) {
	// Encode
	encoded, err := uper.Marshal(original)
	if err != nil {
		log.Fatalf("Failed to encode: %v", err)
	}

	fmt.Printf("Encoded (%d bytes): %X\n", len(encoded), encoded)

	// Decode
	var decoded Config
	if err := uper.Unmarshal(encoded, &decoded); err != nil {
		log.Fatalf("Failed to decode: %v", err)
	}

	// Print decoded values
	fmt.Printf("  Name: %s\n", decoded.Name)
	fmt.Printf("  Port: %d\n", decoded.Port)

	if decoded.Timeout != nil {
		fmt.Printf("  Timeout: %d seconds\n", *decoded.Timeout)
	} else {
		fmt.Println("  Timeout: (not set)")
	}

	if decoded.MaxRetries != nil {
		fmt.Printf("  MaxRetries: %d\n", *decoded.MaxRetries)
	} else {
		fmt.Println("  MaxRetries: (not set)")
	}

	if decoded.Description != nil {
		fmt.Printf("  Description: %s\n", *decoded.Description)
	} else {
		fmt.Println("  Description: (not set)")
	}
}
