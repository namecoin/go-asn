# Struct Tag Reference

This document provides a comprehensive reference for all ASN.1 struct tags supported by the go-asn library.

## Tag Format

Tags are specified in the `asn1` struct field tag:

```go
type Example struct {
    Field int `asn1:"option1,option2,key:value"`
}
```

Multiple options are comma-separated. Order does not matter.

## Quick Reference

| Tag               | Description                  | Example        |
|-------------------|------------------------------|----------------|
| `size:N`          | Fixed size                   | `size:4`       |
| `size:MIN..MAX`   | Size range                   | `size:0..255`  |
| `optional`        | Field may be absent          | `optional`     |
| `choice:N`        | CHOICE alternative           | `choice:0`     |
| `ia5string`       | IA5String (7-bit ASCII)      | `ia5string`    |
| `utf8string`      | UTF8String (8-bit)           | `utf8string`   |
| `visiblestring`   | VisibleString (printable)    | `visiblestring` |
| `printablestring` | PrintableString (restricted) | `printablestring` |

## Size Constraints

### `size:N` - Fixed Size

Specifies an exact size. No length prefix is encoded.

```go
type Example struct {
    // Integers
    Code int `asn1:"size:100"`  // Exactly 100 (7 bits needed)

    // Strings (character count)
    PIN string `asn1:"ia5string,size:4"`  // Exactly 4 characters

    // OCTET STRING (byte count)
    MAC []byte `asn1:"size:6"`  // Exactly 6 bytes

    // SEQUENCE OF (element count)
    Slots []Item `asn1:"size:10"`  // Exactly 10 items
}
```

### `size:MIN..MAX` - Range Constraint

Specifies a minimum and maximum. A length prefix is encoded for variable-length types.

```go
type Example struct {
    // Integer range
    Age int `asn1:"size:0..150"`  // 0 to 150 (8 bits, offset encoding)

    // Signed range
    Temp int `asn1:"size:-50..50"`  // -50 to +50 (7 bits, offset from -50)

    // Strings
    Name string `asn1:"ia5string,size:1..64"`  // 1 to 64 characters

    // OCTET STRING
    Data []byte `asn1:"size:0..1024"`  // 0 to 1024 bytes

    // SEQUENCE OF
    Items []Item `asn1:"size:1..100"`  // 1 to 100 items
}
```

### Bit Calculation

The number of bits used is the minimum needed to represent the range:

| Range    | Values | Bits |
|----------|--------|------|
| 0..1     | 2      | 1    |
| 0..3     | 4      | 2    |
| 0..7     | 8      | 3    |
| 0..15    | 16     | 4    |
| 0..31    | 32     | 5    |
| 0..63    | 64     | 6    |
| 0..127   | 128    | 7    |
| 0..255   | 256    | 8    |
| 0..65535 | 65536  | 16   |

For offset ranges (e.g., `10..20`), the range size determines bits, not the absolute values.

## Integer Encoding

All integers **must** have a `size` constraint.

### Unsigned Integers

```go
type Example struct {
    Byte    int `asn1:"size:0..255"`     // 8 bits
    Word    int `asn1:"size:0..65535"`   // 16 bits
    ThreeBit int `asn1:"size:0..7"`      // 3 bits
}
```

### Signed Integers (Offset Encoding)

Negative minimums are supported via offset encoding:

```go
type Example struct {
    // -40 to 85 has range 126, needs 7 bits
    // Encoded as: value - (-40) = value + 40
    Temperature int `asn1:"size:-40..85"`

    // -90 to 90 has range 181, needs 8 bits
    Latitude int `asn1:"size:-90..90"`

    // -180 to 180 has range 361, needs 9 bits
    Longitude int `asn1:"size:-180..180"`
}
```

### Offset Ranges

Non-zero minimums use offset encoding:

```go
type Example struct {
    // 100 to 600 has range 501, needs 9 bits
    // Encoded as: value - 100
    FlightLevel int `asn1:"size:100..600"`

    // 2000 to 2025 has range 26, needs 5 bits
    Year int `asn1:"size:2000..2025"`
}
```

## String Types

All strings **must** have both a type tag and a size constraint.

### `ia5string` - IA5String

ASCII subset (characters 0-127). Encoded as 7 bits per character.

```go
Field string `asn1:"ia5string,size:1..100"`
```

**Valid characters:** All ASCII 0-127
**Common use:** General text, identifiers

### `utf8string` - UTF8String

Full UTF-8 encoding. Encoded as 8 bits per byte.

```go
Field string `asn1:"utf8string,size:0..255"`
```

**Note:** The size constraint refers to **byte length**, not character count.

**Common use:** Internationalised text

### `visiblestring` - VisibleString

Printable ASCII subset (characters 32-126). Encoded as 7 bits per character.

```go
Field string `asn1:"visiblestring,size:1..50"`
```

**Valid characters:** Space (32) through tilde (126)
**Common use:** Display text

### `printablestring` - PrintableString

Restricted character set. Encoded as 7 bits per character.

```go
Field string `asn1:"printablestring,size:1..64"`
```

**Valid characters:**
- `A-Z` (uppercase letters)
- `a-z` (lowercase letters)
- `0-9` (digits)
- ` ` (space)
- `'()+,-./:=?` (punctuation)

**Common use:** Certificates, formal identifiers

### Character Encoding Summary

| Type            | Valid Range | Bits/Char | Use Case      |
|-----------------|-------------|-----------|---------------|
| ia5string       | 0-127       | 7         | General ASCII |
| utf8string      | Any UTF-8   | 8/byte    | International |
| visiblestring   | 32-126      | 7         | Display text  |
| printablestring | Restricted  | 7         | Formal IDs    |

## Optional Fields

### `optional` - Optional Field

Marks a field as OPTIONAL. **Must use a pointer type.**

```go
type Message struct {
    Required int  `asn1:"size:0..255"`
    Optional *int `asn1:"optional,size:0..255"`
}
```

### Encoding

A presence bitmap (preamble) is written at the start of the struct encoding:
- One bit per optional field, in field declaration order
- `1` = present, `0` = absent
- Absent fields are not encoded after the preamble

### Usage Patterns

```go
// Helper function
func intPtr(v int) *int { return &v }

// Field present
msg := Message{
    Required: 1,
    Optional: intPtr(42),
}

// Field absent
msg := Message{
    Required: 1,
    Optional: nil,
}
```

### Multiple Optional Fields

```go
type Config struct {
    Name     string `asn1:"ia5string,size:1..32"`
    Port     *int   `asn1:"optional,size:1..65535"`  // Optional 1
    Timeout  *int   `asn1:"optional,size:1..3600"`   // Optional 2
    Retries  *int   `asn1:"optional,size:0..10"`     // Optional 3
}

// Preamble will be 3 bits: one for each optional field
// Example: Port present, Timeout absent, Retries present = 101
```

## CHOICE Types

### `choice:N` - CHOICE Alternative

Specifies the index for a CHOICE alternative. All fields should have this tag and **must be pointers**.

```go
type MyChoice struct {
    Option1 *Type1 `asn1:"choice:0"`
    Option2 *Type2 `asn1:"choice:1"`
    Option3 *Type3 `asn1:"choice:2"`
}
```

### Encoding

1. Choice index is encoded using minimum bits for the number of alternatives
2. Selected value is encoded immediately after

| Alternatives | Index Bits |
|--------------|------------|
| 2            | 1          |
| 3-4          | 2          |
| 5-8          | 3          |
| 9-16         | 4          |

### Rules

- **Exactly one field must be non-nil** when marshalling
- Marshalling with zero or multiple non-nil fields returns an error
- Unmarshalling sets the appropriate field based on the decoded index

### Usage

```go
// Select first alternative
choice := MyChoice{
    Option1: &Type1{...},
}

// Select second alternative
choice := MyChoice{
    Option2: &Type2{...},
}

// After unmarshalling, check which is set
switch {
case decoded.Option1 != nil:
    // Handle Option1
case decoded.Option2 != nil:
    // Handle Option2
case decoded.Option3 != nil:
    // Handle Option3
}
```

### CHOICE with Nested Constraints

Each alternative can have its own constraints:

```go
type MessageContent struct {
    Text   *string `asn1:"choice:0,ia5string,size:1..256"`
    Binary *[]byte `asn1:"choice:1,size:0..1024"`
    Number *int    `asn1:"choice:2,size:0..65535"`
}
```

## OCTET STRING

Byte slices (`[]byte`) are encoded as OCTET STRING.

```go
type Example struct {
    // Fixed length
    MAC []byte `asn1:"size:6"`

    // Variable length
    Payload []byte `asn1:"size:0..1024"`
}
```

**Must have a size constraint.**

## SEQUENCE OF

Slices (except `[]byte`) are encoded as SEQUENCE OF.

```go
type Container struct {
    Items []Item `asn1:"size:1..100"`
}

type Item struct {
    ID    int  `asn1:"size:0..255"`
    Value bool
}
```

**Must have a size constraint.**

### Nested Constraints

Elements use their own struct tags:

```go
type List struct {
    Strings []string `asn1:"size:0..10"`  // ERROR: no string type for elements
}

// Use a wrapper struct instead:
type StringItem struct {
    Value string `asn1:"ia5string,size:1..100"`
}

type List struct {
    Strings []StringItem `asn1:"size:0..10"`  // OK
}
```

## Complete Example

```go
package main

import "github.com/shaneshort/go-asn/uper"

// Flight plan message
type FlightPlan struct {
    // Required fields
    FlightID    string `asn1:"ia5string,size:4..8"`      // e.g., "UAL123"
    AircraftType string `asn1:"ia5string,size:4"`        // e.g., "B738"
    Departure   Airport
    Arrival     Airport

    // Optional fields
    Alternate   *Airport `asn1:"optional"`
    Remarks     *string  `asn1:"optional,ia5string,size:1..256"`
}

type Airport struct {
    ICAO string `asn1:"ia5string,size:4"`  // e.g., "KLAX"
}

// Route with waypoints
type Route struct {
    Waypoints []Waypoint `asn1:"size:2..50"`
}

type Waypoint struct {
    Name      string `asn1:"ia5string,size:2..5"`
    Altitude  *int   `asn1:"optional,size:0..600"`  // Flight level
    Speed     *int   `asn1:"optional,size:0..999"`  // Knots
}

// Position report (CHOICE of formats)
type Position struct {
    LatLon    *LatLonPosition `asn1:"choice:0"`
    Waypoint  *WaypointPosition `asn1:"choice:1"`
    Bearing   *BearingPosition `asn1:"choice:2"`
}

type LatLonPosition struct {
    Latitude  int `asn1:"size:-9000..9000"`   // Hundredths of degrees
    Longitude int `asn1:"size:-18000..18000"`
}

type WaypointPosition struct {
    Name   string `asn1:"ia5string,size:2..5"`
    Offset *int   `asn1:"optional,size:0..99"`  // NM from waypoint
}

type BearingPosition struct {
    Reference string `asn1:"ia5string,size:3"`  // VOR ID
    Radial    int    `asn1:"size:0..359"`
    Distance  int    `asn1:"size:0..999"`
}

func main() {
    plan := FlightPlan{
        FlightID:     "UAL123",
        AircraftType: "B738",
        Departure:    Airport{ICAO: "KSFO"},
        Arrival:      Airport{ICAO: "KLAX"},
    }

    data, _ := uper.Marshal(plan)
    // Use data...
}
```

## Error Handling

Invalid tags result in errors at runtime:

```go
// Missing size constraint
type Bad1 struct {
    Value int  // ERROR: integer requires size constraint
}

// Missing string type
type Bad2 struct {
    Name string `asn1:"size:1..64"`  // ERROR: string requires type tag
}

// Invalid tag syntax
type Bad3 struct {
    Value int `asn1:"size:abc"`  // ERROR: invalid size value
}
```

Errors include context:
```
marshal Bad1.Value: integer requires size constraint (e.g., size:0..255)
```

## Future Tags (Not Yet Implemented)

### `extensible`

Extension marker for forward compatibility.

### `default:N`

Default value when field is absent.

### `tag:N`

Explicit context tag number.
