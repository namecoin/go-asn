package asn1

import (
	"errors"
	"testing"
)

func TestError_Error(t *testing.T) {
	err := &Error{
		Op:     "marshal",
		Type:   "FlightInfo",
		Field:  "Altitude",
		Reason: "value 500 exceeds maximum 255",
	}

	want := "asn1: marshal FlightInfo.Altitude: value 500 exceeds maximum 255"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestError_ErrorNoField(t *testing.T) {
	err := &Error{
		Op:     "unmarshal",
		Type:   "Message",
		Reason: "unexpected end of data",
	}

	want := "asn1: unmarshal Message: unexpected end of data"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestError_Unwrap(t *testing.T) {
	inner := errors.New("underlying error")
	err := &Error{
		Op:      "marshal",
		Type:    "Test",
		Reason:  "failed",
		Wrapped: inner,
	}

	if !errors.Is(err, inner) {
		t.Error("Unwrap() should return wrapped error")
	}
}
