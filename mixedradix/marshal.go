package mixedradix

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/namecoin/go-asn/asn1"
)

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

	value := asn1.MixedRadixNumber{
		Value: new(big.Int),
		Base:  big.NewInt(1),
	}
	if err := MarshalValue(&value, rv, asn1.FieldOptions{}); err != nil {
		return nil, err
	}

	return value.Value.Bytes(), nil
}

// Namecoin: Public in order to facilitate using an out of band length for SEQUENCE OF.
// MarshalValue encodes a single value based on its type.
func MarshalValue(mixedRadixCtx *asn1.MixedRadixNumber, v reflect.Value, opts asn1.FieldOptions) error {
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
		marshalBool(mixedRadixCtx, v.Bool())
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return marshalInt(mixedRadixCtx, v.Int(), opts)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return marshalInt(mixedRadixCtx, int64(v.Uint()), opts)
	case reflect.Struct:
		return marshalStruct(mixedRadixCtx, v)
	case reflect.String:
		return marshalString(mixedRadixCtx, v.String(), opts)
	case reflect.Slice:
		// Check if this is a byte slice (OCTET STRING)
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return marshalOctetString(mixedRadixCtx, v.Bytes(), opts)
		}
		return marshalSequenceOf(mixedRadixCtx, v, opts)
	default:
		return &asn1.Error{
			Op:     "marshal",
			Type:   v.Type().String(),
			Reason: fmt.Sprintf("unsupported type: %s", v.Kind()),
		}
	}
}

// marshalBool encodes a boolean as a single bit.
func marshalBool(mixedRadixCtx *asn1.MixedRadixNumber, v bool) {
	value := int64(0)
	if v {
		value = 1
	}
	mult := new(big.Int).Mul(big.NewInt(value), mixedRadixCtx.Base)
	mixedRadixCtx.Value.Add(mixedRadixCtx.Value, mult)
	mixedRadixCtx.Base.Mul(mixedRadixCtx.Base, big.NewInt(2))
}

// marshalStruct encodes each exported field of a struct in sequence.
// Optional fields are encoded with a presence bitmap (preamble)
// that precedes all field values. Each optional field contributes one bit
// to the preamble: 1 if present, 0 if absent.
//
// If the struct represents a CHOICE (all exported fields have choice:N tags
// or are pointer types with exactly one non-nil), it is encoded as a CHOICE.
func marshalStruct(mixedRadixCtx *asn1.MixedRadixNumber, v reflect.Value) error {
	t := v.Type()

	// Check if this struct represents a CHOICE type
	if isChoiceStruct(v) {
		return marshalChoice(mixedRadixCtx, v)
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
		value := 0
		if present {
			value = 1
		}

		mul := new(big.Int).Mul(big.NewInt(int64(value)), mixedRadixCtx.Base)
		mixedRadixCtx.Value.Add(mixedRadixCtx.Value, mul)
		mixedRadixCtx.Base.Mul(mixedRadixCtx.Base, big.NewInt(2))
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

		if err := MarshalValue(mixedRadixCtx, field, opts); err != nil {
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
func marshalChoice(mixedRadixCtx *asn1.MixedRadixNumber, v reflect.Value) error {
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
	mul := new(big.Int).Mul(big.NewInt(int64(selectedChoiceIndex)), mixedRadixCtx.Base)
	mixedRadixCtx.Value.Add(mixedRadixCtx.Value, mul)
	mixedRadixCtx.Base.Mul(mixedRadixCtx.Base, big.NewInt(int64(numAlternatives)))

	// Encode the selected value
	field := v.Field(selectedField)
	sf := t.Field(selectedField)
	tag := sf.Tag.Get("asn1")
	opts, _ := asn1.ParseTag(tag) // Already validated above

	// Dereference the pointer and marshal the value
	if field.Kind() == reflect.Ptr {
		field = field.Elem()
	}

	if err := MarshalValue(mixedRadixCtx, field, opts); err != nil {
		var e *asn1.Error
		if errors.As(err, &e) && e.Field == "" {
			e.Field = sf.Name
		}
		return err
	}

	return nil
}

// The value is encoded as an offset from the minimum, using the minimum
// number of bits required to represent the range.
func marshalInt(mixedRadixCtx *asn1.MixedRadixNumber, v int64, opts asn1.FieldOptions) error {
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

	// Encode the offset value (value relative to minimum)
	offset := uint64(v - lowerBound)

	mult := new(big.Int).Mul(new(big.Int).SetUint64(offset), mixedRadixCtx.Base)
	mixedRadixCtx.Value.Add(mixedRadixCtx.Value, mult)
	mixedRadixCtx.Base.Mul(mixedRadixCtx.Base, big.NewInt(rangeSize))

	return nil
}

// marshalOctetString encodes a byte slice as an ASN.1 OCTET STRING.
// For fixed-size constraints (min == max), the data is written directly.
// For variable-size constraints, the length (as an offset from min) is
// encoded first, followed by the data.
func marshalOctetString(mixedRadixCtx *asn1.MixedRadixNumber, data []byte, opts asn1.FieldOptions) error {
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
		offset := uint64(length - lowerBound)
		mul := new(big.Int).Mul(new(big.Int).SetUint64(offset), mixedRadixCtx.Base)
		mixedRadixCtx.Value.Add(mixedRadixCtx.Value, mul)
		mixedRadixCtx.Base.Mul(mixedRadixCtx.Base, big.NewInt(rangeSize))
	}

	// Encode each byte of the data
	for _, b := range data {
		mul := new(big.Int).Mul(big.NewInt(int64(b)), mixedRadixCtx.Base)
		mixedRadixCtx.Value.Add(mixedRadixCtx.Value, mul)
		mixedRadixCtx.Base.Mul(mixedRadixCtx.Base, big.NewInt(256))
	}

	return nil
}

// marshalSequenceOf encodes a slice as an ASN.1 SEQUENCE OF.
// The length (as an offset from the minimum) is encoded first, followed by each element.
func marshalSequenceOf(mixedRadixCtx *asn1.MixedRadixNumber, v reflect.Value, opts asn1.FieldOptions) error {
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
		offset := uint64(length - lowerBound)
		mul := new(big.Int).Mul(new(big.Int).SetUint64(offset), mixedRadixCtx.Base)
		mixedRadixCtx.Value.Add(mixedRadixCtx.Value, mul)
		mixedRadixCtx.Base.Mul(mixedRadixCtx.Base, big.NewInt(rangeSize))
	}

	// Encode each element
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		// Pass empty options for elements - they should have their own constraints
		// defined by the element type's struct tags
		if err := MarshalValue(mixedRadixCtx, elem, asn1.FieldOptions{}); err != nil {
			return &asn1.Error{
				Op:     "marshal",
				Type:   v.Type().String(),
				Reason: fmt.Sprintf("element %d: %v", i, err),
			}
		}
	}

	return nil
}

// The string is encoded based on its ASN.1 type (IA5String, UTF8String, etc.)
// with the appropriate character width and validation.
func marshalString(mixedRadixCtx *asn1.MixedRadixNumber, s string, opts asn1.FieldOptions) error {
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
		offset := uint64(length - lowerBound)

		mul := new(big.Int).Mul(new(big.Int).SetUint64(offset), mixedRadixCtx.Base)
		mixedRadixCtx.Value.Add(mixedRadixCtx.Value, mul)
		mixedRadixCtx.Base.Mul(mixedRadixCtx.Base, big.NewInt(rangeSize))
	}

	// Encode the characters based on the string type
	switch opts.StringType {
	case asn1.StringTypeIA5:
		return marshalIA5String(mixedRadixCtx, s)
	case asn1.StringTypeVisible:
		return marshalVisibleString(mixedRadixCtx, s)
	case asn1.StringTypePrintable:
		return marshalPrintableString(mixedRadixCtx, s)
	default: // UTF8 is the default
		marshalUTF8String(mixedRadixCtx, s)
		return nil
	}
}

// marshalIA5String encodes a string as IA5String (7 bits per character).
// IA5String is a subset of ASCII containing characters 0-127.
func marshalIA5String(mixedRadixCtx *asn1.MixedRadixNumber, s string) error {
	for i, c := range s {
		if c > 127 {
			return &asn1.Error{
				Op:     "marshal",
				Type:   "string",
				Reason: fmt.Sprintf("character at position %d (0x%X) is not valid IA5", i, c),
			}
		}
		mul := new(big.Int).Mul(big.NewInt(int64(c)), mixedRadixCtx.Base)
		mixedRadixCtx.Value.Add(mixedRadixCtx.Value, mul)
		mixedRadixCtx.Base.Mul(mixedRadixCtx.Base, big.NewInt(128))
	}
	return nil
}

// marshalVisibleString encodes a string as VisibleString (7 bits per character).
// VisibleString is a subset of IA5 containing ASCII characters 32-126 (printable ASCII).
func marshalVisibleString(mixedRadixCtx *asn1.MixedRadixNumber, s string) error {
	for i, c := range s {
		if c < 32 || c > 126 {
			return &asn1.Error{
				Op:     "marshal",
				Type:   "string",
				Reason: fmt.Sprintf("character at position %d (0x%X) is not valid VisibleString", i, c),
			}
		}
		mul := new(big.Int).Mul(big.NewInt(int64(c-32)), mixedRadixCtx.Base)
		mixedRadixCtx.Value.Add(mixedRadixCtx.Value, mul)
		mixedRadixCtx.Base.Mul(mixedRadixCtx.Base, visibleCharCount)
	}
	return nil
}

func printableOffset(c rune) int64 {
	switch c {
	case ' ':
		return 0
	case '\'', '(', ')':
		return int64(c) - 38
	case '+', ',', '-', '.', '/':
		return int64(c) - 39
	case ':':
		return 9
	case '=':
		return 10
	case '?':
		return 11
	case '\\':
		return 12
	default:
		switch {
		case c >= 'A' && c <= 'Z':
			return int64(c) - 52
		case c >= '0' && c <= '9':
			return int64(c) - 9
		case c >= 'a' && c <= 'z':
			return int64(c) - 48
		default:
			return -1
		}
	}
}

var offsetToPrintable map[int64]rune

func fillReversePrintableMap() {
	if offsetToPrintable == nil {
		offsetToPrintable = map[int64]rune{}
		for x := range 122 {
			runeValue := rune(x)
			if isPrintableChar(runeValue) {
				offsetToPrintable[printableOffset(runeValue)] = runeValue
			}
		}
	}
}

// marshalPrintableString encodes a string as PrintableString (7 bits per character).
// PrintableString is a restricted subset: A-Z, a-z, 0-9, space, and '()+,-./:=?
func marshalPrintableString(mixedRadixCtx *asn1.MixedRadixNumber, s string) error {
	fillReversePrintableMap()
	for i, c := range s {
		if !isPrintableChar(c) {
			return &asn1.Error{
				Op:     "marshal",
				Type:   "string",
				Reason: fmt.Sprintf("character at position %d (%q) is not valid PrintableString", i, c),
			}
		}
		mul := new(big.Int).Mul(big.NewInt(printableOffset(c)), mixedRadixCtx.Base)
		mixedRadixCtx.Value.Add(mixedRadixCtx.Value, mul)
		mixedRadixCtx.Base.Mul(mixedRadixCtx.Base, big.NewInt(int64(len(offsetToPrintable))))
	}
	return nil
}

var visibleCharCount = big.NewInt(126 - 32 + 1)

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
func marshalUTF8String(mixedRadixCtx *asn1.MixedRadixNumber, s string) {
	for _, b := range []byte(s) {
		mul := new(big.Int).Mul(big.NewInt(int64(b)), mixedRadixCtx.Base)
		mixedRadixCtx.Value.Add(mixedRadixCtx.Value, mul)
		mixedRadixCtx.Base.Mul(mixedRadixCtx.Base, big.NewInt(256))
	}
}
