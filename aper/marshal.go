package aper

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/namecoin/go-asn/asn1"
)

// Marshal encodes a value using APER (Aligned Packed Encoding Rules).
// The value must be a struct or a pointer to a struct for complex types,
// or a basic type (bool, int, etc.) for simple encoding.
func Marshal(v interface{}) ([]byte, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, &asn1.Error{Op: "marshal", Type: "nil", Reason: "cannot marshal nil pointer"}
		}
		rv = rv.Elem()
	}

	w := asn1.NewBitWriter(true) // APER is aligned
	if err := marshalValue(w, rv, asn1.FieldOptions{}); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// marshalValue encodes a single value based on its type.
func marshalValue(w *asn1.BitWriter, v reflect.Value, opts asn1.FieldOptions) error {
	// Handle pointers - dereference to get the underlying value.
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Bool:
		return marshalBool(w, v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return marshalInt(w, v.Int(), opts)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return marshalInt(w, int64(v.Uint()), opts)
	case reflect.Struct:
		return marshalStruct(w, v)
	case reflect.String:
		return marshalString(w, v.String(), opts)
	case reflect.Slice:
		// Check if this is a byte slice (OCTET STRING)
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return marshalOctetString(w, v.Bytes(), opts)
		}
		return marshalSequenceOf(w, v, opts)
	default:
		return &asn1.Error{
			Op:     "marshal",
			Type:   v.Type().String(),
			Reason: fmt.Sprintf("unsupported type: %s", v.Kind()),
		}
	}
}

// marshalBool encodes a boolean as a single bit.
// In APER, true is encoded as 1 and false as 0.
func marshalBool(w *asn1.BitWriter, v bool) error {
	if v {
		return w.WriteBits(1, 1)
	}
	return w.WriteBits(0, 1)
}

// marshalStruct encodes each exported field of a struct in sequence.
// For APER, optional fields are encoded with a presence bitmap (preamble)
// that precedes all field values.
func marshalStruct(w *asn1.BitWriter, v reflect.Value) error {
	t := v.Type()

	// Check if this struct represents a CHOICE type
	if isChoiceStruct(v) {
		return marshalChoice(w, v)
	}

	// First pass: identify optional fields and write the presence preamble.
	var optionalFieldIndices []int
	for i := 0; i < v.NumField(); i++ {
		structField := t.Field(i)
		if !structField.IsExported() {
			continue
		}
		tag := structField.Tag.Get("asn1")
		opts, err := asn1.ParseTag(tag)
		if err != nil {
			return &asn1.Error{
				Op:     "marshal",
				Type:   t.Name(),
				Field:  structField.Name,
				Reason: fmt.Sprintf("invalid tag: %v", err),
			}
		}
		if opts.Optional {
			optionalFieldIndices = append(optionalFieldIndices, i)
		}
	}

	// Write the presence bitmap for optional fields
	for _, idx := range optionalFieldIndices {
		field := v.Field(idx)
		present := !isFieldAbsent(field)
		if present {
			if err := w.WriteBits(1, 1); err != nil {
				return err
			}
		} else {
			if err := w.WriteBits(0, 1); err != nil {
				return err
			}
		}
	}

	// Second pass: encode field values in order
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		structField := t.Field(i)

		// Skip unexported fields
		if !structField.IsExported() {
			continue
		}

		// Parse the asn1 tag
		tag := structField.Tag.Get("asn1")
		opts, err := asn1.ParseTag(tag)
		if err != nil {
			return &asn1.Error{
				Op:     "marshal",
				Type:   t.Name(),
				Field:  structField.Name,
				Reason: fmt.Sprintf("invalid tag: %v", err),
			}
		}

		// Skip absent optional fields
		if opts.Optional && isFieldAbsent(field) {
			continue
		}

		if err := marshalValue(w, field, opts); err != nil {
			var e *asn1.Error
			if errors.As(err, &e) && e.Field == "" {
				e.Field = structField.Name
			}
			return err
		}
	}

	return nil
}

// isFieldAbsent returns true if the field is considered absent for optional encoding.
func isFieldAbsent(v reflect.Value) bool {
	if v.Kind() == reflect.Ptr {
		return v.IsNil()
	}
	return false
}

// isChoiceStruct returns true if the struct represents a CHOICE type.
func isChoiceStruct(v reflect.Value) bool {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}

		tag := sf.Tag.Get("asn1")
		opts, err := asn1.ParseTag(tag)
		if err == nil && opts.Choice != nil {
			return true
		}
	}

	return false
}

// marshalChoice encodes a CHOICE type.
func marshalChoice(w *asn1.BitWriter, v reflect.Value) error {
	t := v.Type()

	type choiceAlt struct {
		fieldIndex  int
		choiceIndex int
		opts        asn1.FieldOptions
	}

	var alternatives []choiceAlt
	selectedField := -1
	selectedChoiceIndex := -1

	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}

		tag := sf.Tag.Get("asn1")
		opts, err := asn1.ParseTag(tag)
		if err != nil {
			return &asn1.Error{
				Op:     "marshal",
				Type:   t.Name(),
				Field:  sf.Name,
				Reason: fmt.Sprintf("invalid tag: %v", err),
			}
		}

		choiceIdx := len(alternatives)
		if opts.Choice != nil {
			choiceIdx = *opts.Choice
		}

		alternatives = append(alternatives, choiceAlt{
			fieldIndex:  i,
			choiceIndex: choiceIdx,
			opts:        opts,
		})

		field := v.Field(i)
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			if selectedField != -1 {
				return &asn1.Error{
					Op:     "marshal",
					Type:   t.Name(),
					Reason: "CHOICE must have exactly one non-nil field",
				}
			}
			selectedField = i
			selectedChoiceIndex = choiceIdx
		}
	}

	if selectedField == -1 {
		return &asn1.Error{
			Op:     "marshal",
			Type:   t.Name(),
			Reason: "CHOICE has no non-nil field selected",
		}
	}

	if len(alternatives) < 2 {
		return &asn1.Error{
			Op:     "marshal",
			Type:   t.Name(),
			Reason: "CHOICE must have at least 2 alternatives",
		}
	}

	// Encode the choice index
	numAlternatives := len(alternatives)
	numBits := bitsNeeded(uint64(numAlternatives - 1))
	if err := w.WriteBits(uint64(selectedChoiceIndex), numBits); err != nil {
		return err
	}

	// Encode the selected value
	field := v.Field(selectedField)
	sf := t.Field(selectedField)
	tag := sf.Tag.Get("asn1")
	opts, _ := asn1.ParseTag(tag)

	if field.Kind() == reflect.Ptr {
		field = field.Elem()
	}

	if err := marshalValue(w, field, opts); err != nil {
		var e *asn1.Error
		if errors.As(err, &e) && e.Field == "" {
			e.Field = sf.Name
		}
		return err
	}

	return nil
}

// marshalInt encodes a constrained integer using APER encoding.
// For APER, integers that require more than 8 bits are aligned to byte boundaries.
func marshalInt(w *asn1.BitWriter, v int64, opts asn1.FieldOptions) error {
	if opts.SizeMin == nil || opts.SizeMax == nil {
		return &asn1.Error{
			Op:     "marshal",
			Type:   "int",
			Reason: "integer requires size constraint (e.g., size:0..255)",
		}
	}

	lowerBound := *opts.SizeMin
	upperBound := *opts.SizeMax

	if v < lowerBound || v > upperBound {
		return &asn1.Error{
			Op:     "marshal",
			Type:   "int",
			Reason: fmt.Sprintf("value %d out of range [%d, %d]", v, lowerBound, upperBound),
		}
	}

	rangeSize := upperBound - lowerBound + 1
	numBits := bitsNeeded(uint64(rangeSize - 1))

	// In APER, if the integer requires more than 8 bits, align to byte boundary first
	if numBits > 8 {
		w.AlignToByte()
	}

	offset := uint64(v - lowerBound)
	return w.WriteBits(offset, numBits)
}

// bitsNeeded returns the number of bits required to represent the given value.
func bitsNeeded(value uint64) int {
	if value == 0 {
		return 1
	}
	bits := 0
	for value > 0 {
		bits++
		value >>= 1
	}
	return bits
}

// marshalOctetString encodes a byte slice as an ASN.1 OCTET STRING.
// In APER, the data is aligned to byte boundaries.
func marshalOctetString(w *asn1.BitWriter, data []byte, opts asn1.FieldOptions) error {
	if opts.SizeMin == nil || opts.SizeMax == nil {
		return &asn1.Error{
			Op:     "marshal",
			Type:   "[]byte",
			Reason: "OCTET STRING requires size constraint",
		}
	}

	lowerBound := *opts.SizeMin
	upperBound := *opts.SizeMax
	length := int64(len(data))

	if length < lowerBound || length > upperBound {
		return &asn1.Error{
			Op:     "marshal",
			Type:   "[]byte",
			Reason: fmt.Sprintf("length %d out of range [%d, %d]", length, lowerBound, upperBound),
		}
	}

	// For variable-length OCTET STRING, encode the length first
	if lowerBound != upperBound {
		rangeSize := upperBound - lowerBound + 1
		numBits := bitsNeeded(uint64(rangeSize - 1))
		offset := uint64(length - lowerBound)
		if err := w.WriteBits(offset, numBits); err != nil {
			return err
		}
	}

	// In APER, align to byte boundary before writing the data
	// (only if the data is non-empty)
	if length > 0 {
		w.AlignToByte()
	}

	// Encode each byte of the data
	for _, b := range data {
		if err := w.WriteBits(uint64(b), 8); err != nil {
			return err
		}
	}

	return nil
}

// marshalSequenceOf encodes a slice as an ASN.1 SEQUENCE OF.
func marshalSequenceOf(w *asn1.BitWriter, v reflect.Value, opts asn1.FieldOptions) error {
	if opts.SizeMin == nil || opts.SizeMax == nil {
		return &asn1.Error{
			Op:     "marshal",
			Type:   v.Type().String(),
			Reason: "SEQUENCE OF requires size constraint",
		}
	}

	lowerBound := *opts.SizeMin
	upperBound := *opts.SizeMax
	length := int64(v.Len())

	if length < lowerBound || length > upperBound {
		return &asn1.Error{
			Op:     "marshal",
			Type:   v.Type().String(),
			Reason: fmt.Sprintf("length %d out of range [%d, %d]", length, lowerBound, upperBound),
		}
	}

	// For variable-length SEQUENCE OF, encode the length first
	if lowerBound != upperBound {
		rangeSize := upperBound - lowerBound + 1
		numBits := bitsNeeded(uint64(rangeSize - 1))
		offset := uint64(length - lowerBound)
		if err := w.WriteBits(offset, numBits); err != nil {
			return err
		}
	}

	// Encode each element
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if err := marshalValue(w, elem, asn1.FieldOptions{}); err != nil {
			return &asn1.Error{
				Op:     "marshal",
				Type:   v.Type().String(),
				Reason: fmt.Sprintf("element %d: %v", i, err),
			}
		}
	}

	return nil
}

// marshalString encodes a string using APER encoding.
func marshalString(w *asn1.BitWriter, s string, opts asn1.FieldOptions) error {
	if opts.SizeMin == nil || opts.SizeMax == nil {
		return &asn1.Error{
			Op:     "marshal",
			Type:   "string",
			Reason: "string requires size constraint",
		}
	}

	lowerBound := *opts.SizeMin
	upperBound := *opts.SizeMax

	var length int64
	if opts.StringType == asn1.StringTypeUTF8 {
		length = int64(len(s))
	} else {
		length = int64(len([]rune(s)))
	}

	if length < lowerBound || length > upperBound {
		return &asn1.Error{
			Op:     "marshal",
			Type:   "string",
			Reason: fmt.Sprintf("length %d out of range [%d, %d]", length, lowerBound, upperBound),
		}
	}

	// For variable-length strings, encode the length first
	if lowerBound != upperBound {
		rangeSize := upperBound - lowerBound + 1
		numBits := bitsNeeded(uint64(rangeSize - 1))
		offset := uint64(length - lowerBound)
		if err := w.WriteBits(offset, numBits); err != nil {
			return err
		}
	}

	// In APER, align to byte boundary before writing string data (if non-empty)
	if length > 0 {
		w.AlignToByte()
	}

	// Encode the characters based on the string type
	switch opts.StringType {
	case asn1.StringTypeIA5:
		return marshalIA5String(w, s)
	case asn1.StringTypeVisible:
		return marshalVisibleString(w, s)
	case asn1.StringTypePrintable:
		return marshalPrintableString(w, s)
	default:
		return marshalUTF8String(w, s)
	}
}

// marshalIA5String encodes a string as IA5String (7 bits per character).
func marshalIA5String(w *asn1.BitWriter, s string) error {
	for i, c := range s {
		if c > 127 {
			return &asn1.Error{
				Op:     "marshal",
				Type:   "string",
				Reason: fmt.Sprintf("character at position %d (0x%X) is not valid IA5", i, c),
			}
		}
		if err := w.WriteBits(uint64(c), 7); err != nil {
			return err
		}
	}
	return nil
}

// marshalVisibleString encodes a string as VisibleString (7 bits per character).
func marshalVisibleString(w *asn1.BitWriter, s string) error {
	for i, c := range s {
		if c < 32 || c > 126 {
			return &asn1.Error{
				Op:     "marshal",
				Type:   "string",
				Reason: fmt.Sprintf("character at position %d (0x%X) is not valid VisibleString", i, c),
			}
		}
		if err := w.WriteBits(uint64(c), 7); err != nil {
			return err
		}
	}
	return nil
}

// marshalPrintableString encodes a string as PrintableString (7 bits per character).
func marshalPrintableString(w *asn1.BitWriter, s string) error {
	for i, c := range s {
		if !isPrintableChar(c) {
			return &asn1.Error{
				Op:     "marshal",
				Type:   "string",
				Reason: fmt.Sprintf("character at position %d (%q) is not valid PrintableString", i, c),
			}
		}
		if err := w.WriteBits(uint64(c), 7); err != nil {
			return err
		}
	}
	return nil
}

// isPrintableChar returns true if the character is valid for ASN.1 PrintableString.
func isPrintableChar(c rune) bool {
	if c >= 'A' && c <= 'Z' {
		return true
	}
	if c >= 'a' && c <= 'z' {
		return true
	}
	if c >= '0' && c <= '9' {
		return true
	}
	switch c {
	case ' ', '\'', '(', ')', '+', ',', '-', '.', '/', ':', '=', '?':
		return true
	}
	return false
}

// marshalUTF8String encodes a string as UTF8String (8 bits per byte).
func marshalUTF8String(w *asn1.BitWriter, s string) error {
	for _, b := range []byte(s) {
		if err := w.WriteBits(uint64(b), 8); err != nil {
			return err
		}
	}
	return nil
}
