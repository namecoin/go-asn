// Package aper implements ASN.1 Aligned Packed Encoding Rules (APER).
//
// APER is used in telecommunications protocols (5G/LTE). It aligns certain
// fields to byte boundaries for easier parsing at the cost of slightly
// larger encoded sizes compared to UPER.
//
// Basic usage:
//
//	type Message struct {
//	    ID   int    `asn1:"size:0..255"`
//	    Name string `asn1:"ia5string"`
//	}
//
//	data, err := aper.Marshal(msg)
//	err = aper.Unmarshal(data, &msg)
package aper
