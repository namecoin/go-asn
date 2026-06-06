package uper

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/sofia-nep/go-asn/asn1"
)

// Marshal encodes a value using UPER (Unaligned Packed Encoding Rules).
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

	w := asn1.NewBitWriter(false) // UPER is unaligned
	if err := marshalValue(w, rv, asn1.FieldOptions{}); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// marshalValue encodes a single value based on its type.
func marshalValue(w *asn1.BitWriter, v reflect.Value, opts asn1.FieldOptions) error {
	// Handle pointers - dereference to get the underlying value.
	// Optional fields use pointers to indicate presence (non-nil = present).
	// By the time we reach here, the preamble has already been written and
	// absent optional fields have been skipped.
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			// Optional field not present - already handled by the preamble
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
// In UPER, true is encoded as 1 and false as 0.
func marshalBool(w *asn1.BitWriter, v bool) error {
	if v {
		return w.WriteBits(1, 1)
	}
	return w.WriteBits(0, 1)
}

// marshalStruct encodes each exported field of a struct in sequence.
// For UPER, optional fields are encoded with a presence bitmap (preamble)
// that precedes all field values. Each optional field contributes one bit
// to the preamble: 1 if present, 0 if absent.
//
// If the struct represents a CHOICE (all exported fields have choice:N tags
// or are pointer types with exactly one non-nil), it is encoded as a CHOICE.
func marshalStruct(w *asn1.BitWriter, v reflect.Value) error {
	t := v.Type()

	// Check if this struct represents a CHOICE type
	if isChoiceStruct(v) {
		return marshalChoice(w, v)
	}

	// First pass: identify optional fields and write the presence preamble.
	// The preamble is a bitmap where each bit indicates whether the
	// corresponding optional field is present (1) or absent (0).
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

		// Skip absent optional fields - their presence bit is already 0 in the preamble
		if opts.Optional && isFieldAbsent(field) {
			continue
		}

		if err := marshalValue(w, field, opts); err != nil {
			// Wrap the error with field context if not already wrapped
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
// A pointer field is absent if it is nil.
func isFieldAbsent(v reflect.Value) bool {
	if v.Kind() == reflect.Ptr {
		return v.IsNil()
	}
	return false
}

// isChoiceStruct returns true if the struct represents a CHOICE type.
// A CHOICE struct has all exported pointer fields and exactly one is non-nil,
// OR has at least one field with a choice:N tag.
func isChoiceStruct(v reflect.Value) bool {
	t := v.Type()
	hasChoiceTag := false
	allPointers := true
	exportedCount := 0

	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		exportedCount++

		// Check for the choice tag
		tag := sf.Tag.Get("asn1")
		opts, err := asn1.ParseTag(tag)
		if err == nil && opts.Choice != nil {
			hasChoiceTag = true
		}

		// Check if the field is a pointer
		if sf.Type.Kind() != reflect.Ptr {
			allPointers = false
		}
	}

	// If any field has a choice tag, it's a CHOICE
	if hasChoiceTag {
		return true
	}

	// If all exported fields are pointers and there are at least 2, it could be a CHOICE
	// but we need explicit choice tags or other indicators
	// For now, require explicit choice tags
	_ = allPointers
	_ = exportedCount

	return false
}

// marshalChoice encodes a CHOICE type.
// The choice index is encoded first (using the minimum bits for the number of alternatives),
// followed by the chosen value.
func marshalChoice(w *asn1.BitWriter, v reflect.Value) error {
	t := v.Type()

	// Build a map of choice index to field index, and find the selected alternative
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

		// Determine the choice index
		choiceIdx := len(alternatives) // Default: use field order
		if opts.Choice != nil {
			choiceIdx = *opts.Choice
		}

		alternatives = append(alternatives, choiceAlt{
			fieldIndex:  i,
			choiceIndex: choiceIdx,
			opts:        opts,
		})

		// Check if this field is the selected one (non-nil pointer)
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
	// The index uses the minimum bits to represent the number of alternatives
	numAlternatives := len(alternatives)
	numBits := bitsNeeded(uint64(numAlternatives - 1))
	if err := w.WriteBits(uint64(selectedChoiceIndex), numBits); err != nil {
		return err
	}

	// Encode the selected value
	field := v.Field(selectedField)
	sf := t.Field(selectedField)
	tag := sf.Tag.Get("asn1")
	opts, _ := asn1.ParseTag(tag) // Already validated above

	// Dereference the pointer and marshal the value
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

// marshalInt encodes a constrained integer using UPER encoding.
// The value is encoded as an offset from the minimum, using the minimum
// number of bits required to represent the range.
func marshalInt(w *asn1.BitWriter, v int64, opts asn1.FieldOptions) error {
	// Constrained integer requires size bounds
	if opts.SizeMin == nil || opts.SizeMax == nil {
		return &asn1.Error{
			Op:     "marshal",
			Type:   "int",
			Reason: "integer requires size constraint (e.g., size:0..255)",
		}
	}

	lowerBound := *opts.SizeMin
	upperBound := *opts.SizeMax

	// Validate the value is within the range
	if v < lowerBound || v > upperBound {
		return &asn1.Error{
			Op:     "marshal",
			Type:   "int",
			Reason: fmt.Sprintf("value %d out of range [%d, %d]", v, lowerBound, upperBound),
		}
	}

	// Calculate the number of bits needed for the range
	rangeSize := upperBound - lowerBound + 1
	numBits := bitsNeeded(uint64(rangeSize - 1))

	// Encode the offset value (value relative to minimum)
	offset := uint64(v - lowerBound)
	return w.WriteBits(offset, numBits)
}

// bitsNeeded returns the number of bits required to represent the given value.
// Returns 1 for value 0 (at least 1 bit is needed).
func bitsNeeded(value uint64) int {
	if value == 0 {
		return 1 // At least 1 bit for the value 0
	}
	bits := 0
	for value > 0 {
		bits++
		value >>= 1
	}
	return bits
}

// marshalOctetString encodes a byte slice as an ASN.1 OCTET STRING.
// For fixed-size constraints (min == max), the data is written directly.
// For variable-size constraints, the length (as an offset from min) is
// encoded first, followed by the data.
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

	// Validate the length is within the specified range
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

	// Encode each byte of the data
	for _, b := range data {
		if err := w.WriteBits(uint64(b), 8); err != nil {
			return err
		}
	}

	return nil
}

// marshalSequenceOf encodes a slice as an ASN.1 SEQUENCE OF.
// The length (as an offset from the minimum) is encoded first, followed by each element.
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

	// Validate the length is within the specified range
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
		// Pass empty options for elements - they should have their own constraints
		// defined by the element type's struct tags
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

// marshalString encodes a string using UPER encoding.
// The string is encoded based on its ASN.1 type (IA5String, UTF8String, etc.)
// with the appropriate character width and validation.
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

	// For UTF8String, the length is the number of bytes, not characters.
	// For other string types (IA5, Visible, Printable), length is the number of characters.
	var length int64
	if opts.StringType == asn1.StringTypeUTF8 {
		length = int64(len(s))
	} else {
		length = int64(len([]rune(s)))
	}

	// Validate the length is within the specified range
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

	// Encode the characters based on the string type
	switch opts.StringType {
	case asn1.StringTypeIA5:
		return marshalIA5String(w, s)
	case asn1.StringTypeVisible:
		return marshalVisibleString(w, s)
	case asn1.StringTypePrintable:
		return marshalPrintableString(w, s)
	default: // UTF8 is the default
		return marshalUTF8String(w, s)
	}
}

// marshalIA5String encodes a string as IA5String (7 bits per character).
// IA5String is a subset of ASCII containing characters 0-127.
func marshalIA5String(w *asn1.BitWriter, s string) error {
	for i, c := range s {
		if c > 127 {
			return &asn1.Error{
				Op:     "marshal",
				Type:   "string",
				Reason: fmt.Sprintf("character at position %d (0x%X) is not valid IA5", i, c),
			}
		}
		// IA5 uses 7 bits per character
		if err := w.WriteBits(uint64(c), 7); err != nil {
			return err
		}
	}
	return nil
}

// marshalVisibleString encodes a string as VisibleString (7 bits per character).
// VisibleString is a subset of IA5 containing ASCII characters 32-126 (printable ASCII).
func marshalVisibleString(w *asn1.BitWriter, s string) error {
	for i, c := range s {
		if c < 32 || c > 126 {
			return &asn1.Error{
				Op:     "marshal",
				Type:   "string",
				Reason: fmt.Sprintf("character at position %d (0x%X) is not valid VisibleString", i, c),
			}
		}
		// VisibleString uses 7 bits per character in UPER
		if err := w.WriteBits(uint64(c), 7); err != nil {
			return err
		}
	}
	return nil
}

// marshalPrintableString encodes a string as PrintableString (7 bits per character).
// PrintableString is a restricted subset: A-Z, a-z, 0-9, space, and '()+,-./:=?
func marshalPrintableString(w *asn1.BitWriter, s string) error {
	for i, c := range s {
		if !isPrintableChar(c) {
			return &asn1.Error{
				Op:     "marshal",
				Type:   "string",
				Reason: fmt.Sprintf("character at position %d (%q) is not valid PrintableString", i, c),
			}
		}
		// PrintableString uses 7 bits per character in UPER
		if err := w.WriteBits(uint64(c), 7); err != nil {
			return err
		}
	}
	return nil
}

// isPrintableChar returns true if the character is valid for ASN.1 PrintableString.
// Valid characters are: A-Z, a-z, 0-9, space, and '()+,-./:=?
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
// The raw UTF-8 bytes are written directly.
func marshalUTF8String(w *asn1.BitWriter, s string) error {
	for _, b := range []byte(s) {
		if err := w.WriteBits(uint64(b), 8); err != nil {
			return err
		}
	}
	return nil
}
