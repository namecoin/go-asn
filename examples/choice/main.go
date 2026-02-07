// Example: CHOICE types in ASN.1
//
// Demonstrates how to use CHOICE (union types) where exactly one
// alternative is selected.
//
// Run with: go run main.go
package main

import (
	"fmt"
	"log"

	"github.com/shaneshort/go-asn/uper"
)

// Response is a CHOICE type - exactly one field must be non-nil
type Response struct {
	Success *SuccessData `asn1:"choice:0"`
	Error   *ErrorData   `asn1:"choice:1"`
	Pending *PendingData `asn1:"choice:2"`
}

type SuccessData struct {
	Code    int    `asn1:"size:200..299"`
	Message string `asn1:"ia5string,size:1..100"`
	Data    []byte `asn1:"size:0..1024"`
}

type ErrorData struct {
	Code    int    `asn1:"size:400..599"`
	Message string `asn1:"ia5string,size:1..256"`
}

type PendingData struct {
	RequestID  int `asn1:"size:0..65535"`
	RetryAfter int `asn1:"size:1..3600"` // Seconds to wait
}

func main() {
	fmt.Println("=== CHOICE Example: Success Response ===")
	success := Response{
		Success: &SuccessData{
			Code:    200,
			Message: "Operation completed",
			Data:    []byte{0xDE, 0xAD, 0xBE, 0xEF},
		},
	}
	encodeAndDecode(success)

	fmt.Println("\n=== CHOICE Example: Error Response ===")
	errResp := Response{
		Error: &ErrorData{
			Code:    404,
			Message: "Resource not found",
		},
	}
	encodeAndDecode(errResp)

	fmt.Println("\n=== CHOICE Example: Pending Response ===")
	pending := Response{
		Pending: &PendingData{
			RequestID:  12345,
			RetryAfter: 30,
		},
	}
	encodeAndDecode(pending)
}

func encodeAndDecode(original Response) {
	// Encode
	encoded, err := uper.Marshal(original)
	if err != nil {
		log.Fatalf("Failed to encode: %v", err)
	}

	fmt.Printf("Encoded (%d bytes): %X\n", len(encoded), encoded)

	// Decode
	var decoded Response
	if err := uper.Unmarshal(encoded, &decoded); err != nil {
		log.Fatalf("Failed to decode: %v", err)
	}

	// Print which alternative was selected
	switch {
	case decoded.Success != nil:
		fmt.Println("  Type: Success")
		fmt.Printf("  Code: %d\n", decoded.Success.Code)
		fmt.Printf("  Message: %s\n", decoded.Success.Message)
		fmt.Printf("  Data: %X\n", decoded.Success.Data)

	case decoded.Error != nil:
		fmt.Println("  Type: Error")
		fmt.Printf("  Code: %d\n", decoded.Error.Code)
		fmt.Printf("  Message: %s\n", decoded.Error.Message)

	case decoded.Pending != nil:
		fmt.Println("  Type: Pending")
		fmt.Printf("  RequestID: %d\n", decoded.Pending.RequestID)
		fmt.Printf("  RetryAfter: %d seconds\n", decoded.Pending.RetryAfter)
	}
}
