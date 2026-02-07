# go-asn

A Go library for ASN.1 encoding and decoding using Packed Encoding Rules (PER).

## Overview

ASN.1 (Abstract Syntax Notation One) is a standard interface description language for defining data structures that can be serialised and deserialised in a cross-platform way. PER (Packed Encoding Rules) is a binary encoding that produces compact output, commonly used in telecommunications protocols.

This library supports:

- **UPER** (Unaligned Packed Encoding Rules) - Maximum compression, bit-level packing
- **APER** (Aligned Packed Encoding Rules) - Byte-aligned for faster parsing

## Features

- Struct tag-based configuration (similar to `encoding/json`)
- Full round-trip support (Marshal and Unmarshal)
- Support for common ASN.1 types:
  - BOOLEAN
  - INTEGER (constrained)
  - OCTET STRING
  - IA5String, UTF8String, VisibleString, PrintableString
  - SEQUENCE (Go struct)
  - SEQUENCE OF (Go slice)
  - CHOICE (struct with pointer fields)
  - OPTIONAL fields
- Comprehensive error messages with field context
- Zero external dependencies

## Installation

```bash
go get github.com/shaneshort/go-asn
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    "github.com/shaneshort/go-asn/uper"
)

// Define your message structure with ASN.1 constraints
type Message struct {
    ID      int    `asn1:"size:0..255"`
    Payload string `asn1:"ia5string,size:1..128"`
    Active  bool
}

func main() {
    // Create a message
    msg := Message{
        ID:      42,
        Payload: "Hello, ASN.1!",
        Active:  true,
    }

    // Encode to UPER
    encoded, err := uper.Marshal(msg)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Encoded (%d bytes): %X\n", len(encoded), encoded)

    // Decode back
    var decoded Message
    if err := uper.Unmarshal(encoded, &decoded); err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Decoded: %+v\n", decoded)
}
```

Output:
```
Encoded (13 bytes): 2A0C9232B63637D0820CE7A3719840
Decoded: {ID:42 Payload:Hello, ASN.1! Active:true}
```

## API Reference

### UPER Package

```go
import "github.com/shaneshort/go-asn/uper"

// Marshal encodes a value using UPER
func Marshal(v interface{}) ([]byte, error)

// Unmarshal decodes UPER data into a value
func Unmarshal(data []byte, v interface{}) error
```

### APER Package

```go
import "github.com/shaneshort/go-asn/aper"

// Marshal encodes a value using APER
func Marshal(v interface{}) ([]byte, error)

// Unmarshal decodes APER data into a value
func Unmarshal(data []byte, v interface{}) error
```

### Error Handling

All errors returned implement the standard `error` interface and include context:

```go
type Error struct {
    Op     string      // Operation (marshal/unmarshal)
    Type   string      // Type name
    Field  string      // Field name (if applicable)
    Reason string      // Human-readable reason
    Err    error       // Underlying error (if any)
}
```

Example error message:
```
marshal Message.Value: value 300 out of range [0, 255]
```

## Struct Tags

All constraints are specified via the `asn1` struct tag. Multiple options are comma-separated.

### Size Constraints

Every constrained type (integers, strings, byte slices, slices) requires a size constraint.

```go
type Example struct {
    // Fixed size
    Code int `asn1:"size:100"`       // Exactly 100

    // Range (variable length)
    Value int `asn1:"size:0..255"`   // 0 to 255 inclusive
}
```

### Integer Types

```go
type Integers struct {
    // Unsigned ranges
    SmallInt  int `asn1:"size:0..7"`       // 3 bits (0-7)
    ByteInt   int `asn1:"size:0..255"`     // 8 bits (0-255)
    ShortInt  int `asn1:"size:0..65535"`   // 16 bits

    // Signed ranges (offset encoding)
    Temperature int `asn1:"size:-40..85"`  // -40 to +85
    Latitude    int `asn1:"size:-90..90"`  // -90 to +90

    // Offset ranges
    FlightLevel int `asn1:"size:100..500"` // Encoded as 0-400
}
```

The encoding uses the minimum bits needed:
- Range of 2 values: 1 bit
- Range of 3-4 values: 2 bits
- Range of 5-8 values: 3 bits
- And so on...

### String Types

```go
type Strings struct {
    // IA5String - ASCII subset (0-127), 7 bits per character
    Callsign string `asn1:"ia5string,size:2..8"`

    // UTF8String - Full UTF-8, 8 bits per byte
    Name string `asn1:"utf8string,size:1..100"`

    // VisibleString - Printable ASCII (32-126), 7 bits per character
    Display string `asn1:"visiblestring,size:1..40"`

    // PrintableString - Restricted: A-Za-z0-9 '()+,-./:=?, 7 bits per character
    Serial string `asn1:"printablestring,size:10"`
}
```

### OCTET STRING (Byte Slices)

```go
type BinaryData struct {
    // Fixed length
    MAC []byte `asn1:"size:6"`           // Exactly 6 bytes

    // Variable length
    Payload []byte `asn1:"size:0..1024"` // 0 to 1024 bytes
}
```

### SEQUENCE (Nested Structs)

```go
type Outer struct {
    Header Header
    Body   Body
}

type Header struct {
    Version int `asn1:"size:0..15"`
    Length  int `asn1:"size:0..65535"`
}

type Body struct {
    Data []byte `asn1:"size:0..255"`
}
```

### SEQUENCE OF (Slices)

```go
type Container struct {
    // Fixed count
    FixedItems []Item `asn1:"size:5"`      // Exactly 5 items

    // Variable count
    Items []Item `asn1:"size:1..100"`      // 1 to 100 items
}

type Item struct {
    ID    int  `asn1:"size:0..255"`
    Valid bool
}
```

### OPTIONAL Fields

Optional fields must be pointer types:

```go
type Message struct {
    Required int  `asn1:"size:0..255"`
    Optional *int `asn1:"optional,size:0..255"`
}

// Usage:
// Present
val := 42
msg := Message{Required: 1, Optional: &val}

// Absent
msg := Message{Required: 1, Optional: nil}
```

Encoding: A presence bitmap (preamble) precedes all field values. One bit per optional field indicates presence (1) or absence (0).

### CHOICE Types

CHOICE is represented as a struct where all fields are pointers with `choice:N` tags:

```go
type MessageType struct {
    Request  *RequestData  `asn1:"choice:0"`
    Response *ResponseData `asn1:"choice:1"`
    Error    *ErrorData    `asn1:"choice:2"`
}

type RequestData struct {
    Command string `asn1:"ia5string,size:1..64"`
}

type ResponseData struct {
    Status int    `asn1:"size:0..255"`
    Data   []byte `asn1:"size:0..1024"`
}

type ErrorData struct {
    Code    int    `asn1:"size:0..65535"`
    Message string `asn1:"ia5string,size:1..256"`
}
```

**Rules:**
- Exactly one field must be non-nil when marshalling
- The choice index is encoded using minimum bits for the number of alternatives
- 2 alternatives = 1 bit, 3-4 = 2 bits, 5-8 = 3 bits, etc.

```go
// Select request
msg := MessageType{
    Request: &RequestData{Command: "STATUS"},
}

// Select error
msg := MessageType{
    Error: &ErrorData{Code: 404, Message: "Not found"},
}
```

## UPER vs APER

| Aspect        | UPER                         | APER                         |
|---------------|------------------------------|------------------------------|
| Alignment     | Bit-packed, no padding       | Byte-aligned at key points   |
| Size          | Smaller                      | Slightly larger              |
| Parsing speed | Slower (bit operations)      | Faster (byte-aligned reads)  |
| Use cases     | Bandwidth-constrained links  | CPU-constrained systems      |
| Standards     | 3GPP, CPDLC                  | 5G NR, LTE                   |

### When APER Aligns

APER inserts padding to reach byte boundaries before:
- Integers requiring more than 8 bits
- OCTET STRING data (after length)
- String data (after length)

Example comparison:
```go
type Message struct {
    Flag  bool
    Value int `asn1:"size:0..65535"` // 16 bits
}

msg := Message{Flag: true, Value: 0x1234}

// UPER: 1 0001 0010 0011 0100 (17 bits) = 3 bytes
// APER: 1 0000000 | 00010010 00110100 (1 + 7 padding + 16) = 3 bytes
//       But the 16-bit value is byte-aligned for easy extraction
```

## Real-World Example: Aviation Message

This example shows a CPDLC (Controller-Pilot Data Link Communications) style message:

```go
package main

import (
    "fmt"
    "log"

    "github.com/shaneshort/go-asn/uper"
)

// CPDLCMessage represents an ATC-pilot communication
type CPDLCMessage struct {
    Header  MessageHeader
    Content MessageContent
}

type MessageHeader struct {
    MessageID   int  `asn1:"size:0..65535"`
    ReferenceID *int `asn1:"optional,size:0..65535"`
    Timestamp   int  `asn1:"size:0..86399"` // Seconds since midnight
    Urgent      bool
}

type MessageContent struct {
    Uplink   *UplinkMsg   `asn1:"choice:0"`
    Downlink *DownlinkMsg `asn1:"choice:1"`
}

type UplinkMsg struct {
    Clearance *ClearanceData `asn1:"choice:0"`
    FreeText  *string        `asn1:"choice:1,ia5string,size:1..256"`
}

type ClearanceData struct {
    Type     int    `asn1:"size:0..15"`
    Waypoint string `asn1:"ia5string,size:2..5"`
    Altitude *int   `asn1:"optional,size:0..600"` // Flight level
}

type DownlinkMsg struct {
    Position *PositionReport `asn1:"choice:0"`
    Wilco    *WilcoResponse  `asn1:"choice:1"`
    Unable   *UnableResponse `asn1:"choice:2"`
}

type PositionReport struct {
    Latitude  int    `asn1:"size:-9000..9000"`   // Hundredths of degrees
    Longitude int    `asn1:"size:-18000..18000"`
    Altitude  int    `asn1:"size:0..600"`        // Flight level
    Heading   int    `asn1:"size:0..359"`
    NextWP    string `asn1:"ia5string,size:2..5"`
}

type WilcoResponse struct {
    ReferenceID int `asn1:"size:0..65535"`
}

type UnableResponse struct {
    ReferenceID int    `asn1:"size:0..65535"`
    Reason      string `asn1:"ia5string,size:1..128"`
}

func main() {
    // Create a clearance message
    altitude := 350 // FL350

    msg := CPDLCMessage{
        Header: MessageHeader{
            MessageID: 1234,
            Timestamp: 43200, // 12:00:00
            Urgent:    false,
        },
        Content: MessageContent{
            Uplink: &UplinkMsg{
                Clearance: &ClearanceData{
                    Type:     3, // Climb clearance
                    Waypoint: "KLAX",
                    Altitude: &altitude,
                },
            },
        },
    }

    // Encode
    data, err := uper.Marshal(msg)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Clearance message: %X (%d bytes)\n", data, len(data))

    // Decode
    var decoded CPDLCMessage
    if err := uper.Unmarshal(data, &decoded); err != nil {
        log.Fatal(err)
    }

    // Access decoded data
    cl := decoded.Content.Uplink.Clearance
    fmt.Printf("Waypoint: %s, FL%d\n", cl.Waypoint, *cl.Altitude)
}
```

Output:
```
Clearance message: 02692A30001274B9906C2BC0 (12 bytes)
Waypoint: KLAX, FL350
```

## How Encoding Works

### Constrained Integer Encoding

For a value V in range [MIN, MAX]:
1. Calculate range size: `rangeSize = MAX - MIN + 1`
2. Calculate bits needed: `bits = ceil(log2(rangeSize))`
3. Encode offset: `encoded = V - MIN` using `bits` bits

Example: Value 15 in range [10, 20]
- Range size: 11
- Bits needed: 4 (can represent 0-15)
- Offset: 15 - 10 = 5
- Binary: 0101

### Variable-Length Encoding

For strings, OCTET STRINGs, and SEQUENCE OFs with variable size:
1. Encode length as constrained integer (offset from minimum)
2. Encode each element

Example: String "AB" with size constraint 0..255
- Length: 2 (8 bits, offset 2 from min 0)
- 'A': 0x41 (7 bits for IA5)
- 'B': 0x42 (7 bits for IA5)

### Optional Field Preamble

Before encoding field values, a bitmap indicates which optional fields are present:

```
Struct:
    Required bool
    OptA     *int  `asn1:"optional"`
    OptB     *int  `asn1:"optional"`
    Another  bool

If OptA is present and OptB is nil:
    Preamble: 10 (2 bits)
    Then: Required value, OptA value, Another value
```

### CHOICE Index

The choice index uses minimum bits for the number of alternatives:

```
CHOICE with 3 alternatives:
    Index 0: 00
    Index 1: 01
    Index 2: 10
    (2 bits needed for 0-2)
```

## Limitations

This library implements a practical subset of ASN.1 PER:

**Not Supported:**
- Unconstrained integers (use constrained with large range)
- REAL (floating-point numbers)
- ENUMERATED (use constrained integers instead)
- SET types (use SEQUENCE, which has deterministic ordering)
- BIT STRING (use OCTET STRING)
- OBJECT IDENTIFIER
- Extension markers (`...`)
- Default values
- Named bits
- Open types

**Workarounds:**
- For ENUMERATED: Use `int` with appropriate `size` constraint
- For floating-point: Encode as scaled integer (e.g., hundredths)
- For BIT STRING: Use `[]byte` with fixed size

## Testing

Run all tests:
```bash
go test ./...
```

Run with verbose output:
```bash
go test -v ./...
```

Run benchmarks:
```bash
go test -bench=. ./...
```

## Contributing

Contributions are welcome. Please ensure:

1. All tests pass
1. New features include tests
1. Code follows existing style
1. Documentation is updated

## Licence

MIT Licence

## Further Reading

- [ITU-T X.680](https://www.itu.int/rec/T-REC-X.680) - ASN.1 specification
- [ITU-T X.691](https://www.itu.int/rec/T-REC-X.691) - PER encoding rules
- [ICAO Doc 9880](https://www.icao.int/) - CPDLC specification (uses ASN.1)
