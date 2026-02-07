# go-asn Examples

This directory contains runnable examples demonstrating the go-asn library.

## Running Examples

Each example is in its own directory and can be run with:

```bash
cd examples/<name>
go run main.go
```

## Examples

### basic

Basic encoding and decoding with common types (integers, strings, booleans).

```bash
cd examples/basic && go run main.go
```

Demonstrates:
- Struct definition with ASN.1 tags
- Marshal and Unmarshal
- Round-trip verification

### optional

Working with OPTIONAL fields using pointer types.

```bash
cd examples/optional && go run main.go
```

Demonstrates:
- Defining optional fields with pointers
- Setting fields as present (non-nil) or absent (nil)
- How presence affects encoded size

### choice

CHOICE types (unions) where exactly one alternative is selected.

```bash
cd examples/choice && go run main.go
```

Demonstrates:
- Defining CHOICE with `choice:N` tags
- Selecting different alternatives
- Type-switching on decoded data

### sequence_of

Lists and arrays using SEQUENCE OF.

```bash
cd examples/sequence_of && go run main.go
```

Demonstrates:
- Variable-length lists with size constraints
- Encoding structs within lists
- Empty lists

### aper_vs_uper

Comparing Aligned (APER) vs Unaligned (UPER) encoding.

```bash
cd examples/aper_vs_uper && go run main.go
```

Demonstrates:
- Size differences between APER and UPER
- When to use each encoding
- Byte alignment in APER

## Creating Your Own Messages

1. Define Go structs with `asn1` tags:

```go
type MyMessage struct {
    ID      int    `asn1:"size:0..255"`
    Name    string `asn1:"ia5string,size:1..64"`
    Active  bool
}
```

2. Encode:

```go
msg := MyMessage{ID: 1, Name: "Test", Active: true}
data, err := uper.Marshal(msg)
```

3. Decode:

```go
var decoded MyMessage
err := uper.Unmarshal(data, &decoded)
```

See the [main README](../README.md) for complete documentation.
