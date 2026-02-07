// Package uper implements ASN.1 Unaligned Packed Encoding Rules (UPER).
//
// UPER is used in aviation (CPDLC, ADS-C) and automotive (V2X) protocols.
// It produces the most compact encoding by not aligning to byte boundaries.
//
// Basic usage:
//
//	type Message struct {
//	    ID   int    `asn1:"size:0..255"`
//	    Name string `asn1:"ia5string"`
//	}
//
//	data, err := uper.Marshal(msg)
//	err = uper.Unmarshal(data, &msg)
package uper
