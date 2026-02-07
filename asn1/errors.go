package asn1

import "fmt"

// Error represents an ASN.1 encoding or decoding error.
// It includes context about where the error occurred, making it easier
// to diagnose issues in complex nested structures.
type Error struct {
	Op      string // Operation: "marshal" or "unmarshal"
	Type    string // Go type name being processed
	Field   string // Field path (empty if the error is at type level)
	Reason  string // Description of what went wrong
	Wrapped error  // Underlying error, if any
}

// Error returns a formatted error message including the operation,
// type, optional field, and reason.
func (e *Error) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("asn1: %s %s: %s", e.Op, e.Type, e.Reason)
	}
	return fmt.Sprintf("asn1: %s %s.%s: %s", e.Op, e.Type, e.Field, e.Reason)
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *Error) Unwrap() error {
	return e.Wrapped
}
