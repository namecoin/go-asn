// Example: SEQUENCE OF (arrays/slices) in ASN.1
//
// Demonstrates how to encode and decode lists of items.
//
// Run with: go run main.go
package main

import (
	"fmt"
	"log"

	"github.com/shaneshort/go-asn/uper"
)

// Inventory contains a list of items
type Inventory struct {
	WarehouseID int    `asn1:"size:1..999"`
	Location    string `asn1:"ia5string,size:1..50"`
	Items       []Item `asn1:"size:0..100"` // 0 to 100 items
}

// Item represents a single inventory item
type Item struct {
	SKU      string `asn1:"ia5string,size:8"` // Fixed 8-character SKU
	Name     string `asn1:"ia5string,size:1..64"`
	Quantity int    `asn1:"size:0..9999"`
	InStock  bool
}

func main() {
	// Create an inventory with multiple items
	inventory := Inventory{
		WarehouseID: 42,
		Location:    "Building A, Section 3",
		Items: []Item{
			{
				SKU:      "WIDGET01",
				Name:     "Standard Widget",
				Quantity: 150,
				InStock:  true,
			},
			{
				SKU:      "GADGET02",
				Name:     "Deluxe Gadget",
				Quantity: 0,
				InStock:  false,
			},
			{
				SKU:      "THING003",
				Name:     "Mystery Thing",
				Quantity: 42,
				InStock:  true,
			},
		},
	}

	fmt.Println("Original inventory:")
	printInventory(inventory)

	// Encode
	encoded, err := uper.Marshal(inventory)
	if err != nil {
		log.Fatalf("Failed to encode: %v", err)
	}

	fmt.Printf("\nEncoded (%d bytes): %X\n", len(encoded), encoded)

	// Decode
	var decoded Inventory
	if err := uper.Unmarshal(encoded, &decoded); err != nil {
		log.Fatalf("Failed to decode: %v", err)
	}

	fmt.Println("\nDecoded inventory:")
	printInventory(decoded)

	// Demonstrate empty list
	fmt.Println("\n=== Empty List Example ===")
	emptyInventory := Inventory{
		WarehouseID: 1,
		Location:    "Empty warehouse",
		Items:       []Item{}, // Empty slice
	}

	encodedEmpty, err := uper.Marshal(emptyInventory)
	if err != nil {
		log.Fatalf("Failed to encode empty: %v", err)
	}

	fmt.Printf("Empty inventory encoded (%d bytes): %X\n", len(encodedEmpty), encodedEmpty)

	var decodedEmpty Inventory
	if err := uper.Unmarshal(encodedEmpty, &decodedEmpty); err != nil {
		log.Fatalf("Failed to decode empty: %v", err)
	}

	fmt.Printf("Decoded: WarehouseID=%d, Location=%q, Items=%d\n",
		decodedEmpty.WarehouseID, decodedEmpty.Location, len(decodedEmpty.Items))
}

func printInventory(inv Inventory) {
	fmt.Printf("  WarehouseID: %d\n", inv.WarehouseID)
	fmt.Printf("  Location: %s\n", inv.Location)
	fmt.Printf("  Items (%d):\n", len(inv.Items))

	for i, item := range inv.Items {
		status := "Out of Stock"
		if item.InStock {
			status = "In Stock"
		}
		fmt.Printf("    [%d] %s - %s (Qty: %d) [%s]\n",
			i+1, item.SKU, item.Name, item.Quantity, status)
	}
}
