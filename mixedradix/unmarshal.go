package mixedradix

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/namecoin/go-asn/asn1"
)

// The value must be a pointer to a struct or a basic type.
func Unmarshal(data []byte, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &asn1.Error{Op: "unmarshal", Type: "nil", Reason: "value must be a non-nil pointer"}
	}

	mixedRadix := new(big.Int).SetBytes(data)
	return UnmarshalValue(mixedRadix, rv.Elem(), asn1.FieldOptions{})
}

// Namecoin: Public in order to facilitate using an out of band length for SEQUENCE OF.
// UnmarshalValue decodes a single value based on its type.
func UnmarshalValue(mixedRadix *big.Int, v reflect.Value, opts asn1.FieldOptions) error {
	// Handle pointers - allocate if nil
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Bool:
		unmarshalBool(mixedRadix, v)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return unmarshalInt(mixedRadix, v, opts)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return unmarshalUint(mixedRadix, v, opts)
	case reflect.Struct:
		return unmarshalStruct(mixedRadix, v)
	case reflect.String:
		return unmarshalString(mixedRadix, v, opts)
	case reflect.Slice:
		// Check if this is a byte slice (OCTET STRING)
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return unmarshalOctetString(mixedRadix, v, opts)
		}
		return unmarshalSequenceOf(mixedRadix, v, opts)
	default:
		return &asn1.Error{
			Op:     "unmarshal",
			Type:   v.Type().String(),
			Reason: fmt.Sprintf("unsupported type: %s", v.Kind()),
		}
	}
}

// unmarshalBool decodes a boolean from a single bit.
func unmarshalBool(mixedRadix *big.Int, v reflect.Value) {
	base := big.NewInt(2)
	bit := new(big.Int)
	_, bit = mixedRadix.DivMod(mixedRadix, base, bit)
	v.SetBool(bit.Uint64() == 1)
}

func unmarshalAnyInt(mixedRadix *big.Int, v reflect.Value, opts asn1.FieldOptions, unsigned bool) error {
	typeName := "int"
	if unsigned {
		typeName = "uint"
	}

	if opts.SizeMin == nil || opts.SizeMax == nil {
		return &asn1.Error{
			Op:     "unmarshal",
			Type:   typeName,
			Reason: "integer requires size constraint (e.g., size:0..255)",
		}
	}

	lowerBound := *opts.SizeMin
	upperBound := *opts.SizeMax

	// Calculate the number of bits needed for the range
	rangeSize := big.NewInt(upperBound - lowerBound + 1)
	offset := new(big.Int)
	_, offset = mixedRadix.DivMod(mixedRadix, rangeSize, offset)

	// Calculate the actual value by adding the minimum
	if unsigned {
		value := offset.Uint64() + uint64(lowerBound)
		v.SetUint(value)
	} else {
		value := offset.Int64() + lowerBound
		v.SetInt(value)
	}

	return nil
}

// The value is decoded as an offset from the minimum, using the minimum
// number of bits required to represent the range.
func unmarshalInt(mixedRadix *big.Int, v reflect.Value, opts asn1.FieldOptions) error {
	return unmarshalAnyInt(mixedRadix, v, opts, false)
}

func unmarshalUint(mixedRadix *big.Int, v reflect.Value, opts asn1.FieldOptions) error {
	return unmarshalAnyInt(mixedRadix, v, opts, true)
}

// unmarshalStruct decodes each exported field of a struct in sequence.
// Optional fields are decoded with a presence bitmap (preamble)
// that precedes all field values.
func unmarshalStruct(mixedRadix *big.Int, v reflect.Value) error {
	t := v.Type()

	// Check if this struct represents a CHOICE type
	if isChoiceStruct(v) {
		return unmarshalChoice(mixedRadix, v)
	}

	// First pass: identify optional fields and read the presence preamble.
	type optionalFieldInfo struct {
		index   int
		present bool
	}
	var optionalFields []optionalFieldInfo

	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}

		tag := sf.Tag.Get("asn1")
		opts, err := asn1.ParseTag(tag)
		if err != nil {
			return &asn1.Error{
				Op:     "unmarshal",
				Type:   t.Name(),
				Field:  sf.Name,
				Reason: fmt.Sprintf("invalid tag: %v", err),
			}
		}

		if opts.Optional {
			optionalFields = append(optionalFields, optionalFieldInfo{index: i, present: false})
		}
	}

	// Read the presence bitmap for optional fields
	for i := range optionalFields {
		base := big.NewInt(2)
		bit := new(big.Int)
		_, bit = mixedRadix.DivMod(mixedRadix, base, bit)
		optionalFields[i].present = bit.Uint64() == 1
	}

	// Build a map of field index to presence for quick lookup
	optionalPresence := make(map[int]bool)
	for _, of := range optionalFields {
		optionalPresence[of.index] = of.present
	}

	// Second pass: decode field values in order
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		sf := t.Field(i)

		// Skip unexported fields
		if !sf.IsExported() {
			continue
		}

		// Parse the asn1 tag
		tag := sf.Tag.Get("asn1")
		opts, err := asn1.ParseTag(tag)
		if err != nil {
			return &asn1.Error{
				Op:     "unmarshal",
				Type:   t.Name(),
				Field:  sf.Name,
				Reason: fmt.Sprintf("invalid tag: %v", err),
			}
		}

		// Check if this is an optional field
		if opts.Optional {
			present, isOptional := optionalPresence[i]
			if isOptional && !present {
				// Field is absent - leave it as nil
				continue
			}
		}

		if err := UnmarshalValue(mixedRadix, field, opts); err != nil {
			// Wrap the error with field context if not already wrapped
			var e *asn1.Error
			if errors.As(err, &e) && e.Field == "" {
				e.Field = sf.Name
			}
			return err
		}
	}

	return nil
}

// unmarshalChoice decodes a CHOICE type.
// The choice index is decoded first, followed by the chosen value.
func unmarshalChoice(mixedRadix *big.Int, v reflect.Value) error {
	t := v.Type()

	// Build a list of choice alternatives
	type choiceAlt struct {
		fieldIndex  int
		choiceIndex int
		opts        asn1.FieldOptions
	}

	var alternatives []choiceAlt

	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}

		tag := sf.Tag.Get("asn1")
		opts, err := asn1.ParseTag(tag)
		if err != nil {
			return &asn1.Error{
				Op:     "unmarshal",
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
	}

	if len(alternatives) < 2 {
		return &asn1.Error{
			Op:     "unmarshal",
			Type:   t.Name(),
			Reason: "CHOICE must have at least 2 alternatives",
		}
	}

	// Read the choice index
	numAlternatives := len(alternatives)
	base := big.NewInt(int64(numAlternatives))
	choiceIdx := new(big.Int)
	_, choiceIdx = mixedRadix.DivMod(mixedRadix, base, choiceIdx)

	// Find the alternative with the matching choice index
	var selectedAlt *choiceAlt
	for i := range alternatives {
		if alternatives[i].choiceIndex == int(choiceIdx.Int64()) {
			selectedAlt = &alternatives[i]
			break
		}
	}

	if selectedAlt == nil {
		return &asn1.Error{
			Op:     "unmarshal",
			Type:   t.Name(),
			Reason: fmt.Sprintf("invalid choice index %d", choiceIdx),
		}
	}

	// Decode the selected value
	field := v.Field(selectedAlt.fieldIndex)

	// Allocate the pointer if necessary
	if field.Kind() == reflect.Ptr && field.IsNil() {
		field.Set(reflect.New(field.Type().Elem()))
	}

	// Unmarshal the value (dereference the pointer)
	target := field
	if target.Kind() == reflect.Ptr {
		target = target.Elem()
	}

	if err := UnmarshalValue(mixedRadix, target, selectedAlt.opts); err != nil {
		sf := t.Field(selectedAlt.fieldIndex)
		var e *asn1.Error
		if errors.As(err, &e) && e.Field == "" {
			e.Field = sf.Name
		}
		return err
	}

	return nil
}

// unmarshalOctetString decodes a byte slice as an ASN.1 OCTET STRING.
func unmarshalOctetString(mixedRadix *big.Int, v reflect.Value, opts asn1.FieldOptions) error {
	if opts.SizeMin == nil || opts.SizeMax == nil {
		return &asn1.Error{
			Op:     "unmarshal",
			Type:   "[]byte",
			Reason: "OCTET STRING requires size constraint",
		}
	}

	lowerBound := *opts.SizeMin
	upperBound := *opts.SizeMax

	// Determine the length
	var length int64
	if lowerBound == upperBound {
		// Fixed size
		length = lowerBound
	} else {
		// Variable size - read the length offset first
		rangeSize := upperBound - lowerBound + 1
		offset := new(big.Int)
		_, offset = mixedRadix.DivMod(mixedRadix, big.NewInt(rangeSize), offset)
		length = offset.Int64() + lowerBound
	}

	// Read the data bytes
	data := make([]byte, length)
	base := big.NewInt(256)
	for i := int64(0); i < length; i++ {
		b := new(big.Int)
		_, b = mixedRadix.DivMod(mixedRadix, base, b)
		data[i] = byte(b.Uint64())
	}

	v.SetBytes(data)
	return nil
}

func unmarshalString(mixedRadix *big.Int, v reflect.Value, opts asn1.FieldOptions) error {
	if opts.SizeMin == nil || opts.SizeMax == nil {
		return &asn1.Error{
			Op:     "unmarshal",
			Type:   "string",
			Reason: "string requires size constraint",
		}
	}

	lowerBound := *opts.SizeMin
	upperBound := *opts.SizeMax

	// Determine the length
	var length int64
	if lowerBound == upperBound {
		// Fixed size
		length = lowerBound
	} else {
		// Variable size - read the length offset first
		rangeSize := big.NewInt(upperBound - lowerBound + 1)
		offset := new(big.Int)
		_, offset = mixedRadix.DivMod(mixedRadix, rangeSize, offset)
		length = offset.Int64() + lowerBound
	}

	// Decode the characters based on the string type
	var s string

	switch opts.StringType {
	case asn1.StringTypeIA5:
		s = unmarshalIA5String(mixedRadix, int(length))
	case asn1.StringTypeVisible:
		s = unmarshalVisibleString(mixedRadix, int(length))
	case asn1.StringTypePrintable:
		s = unmarshalPrintableString(mixedRadix, int(length))
	default: // UTF8 is the default
		s = unmarshalUTF8String(mixedRadix, int(length))
	}

	v.SetString(s)
	return nil
}

// unmarshalIA5String decodes an IA5String (7 bits per character).
func unmarshalIA5String(mixedRadix *big.Int, length int) string {
	chars := make([]byte, length)
	for i := range length {
		base := big.NewInt(128)
		c := new(big.Int)
		_, c = mixedRadix.DivMod(mixedRadix, base, c)
		chars[i] = byte(c.Uint64())
	}
	return string(chars)
}

// unmarshalVisibleString decodes a VisibleString (7 bits per character).
func unmarshalVisibleString(mixedRadix *big.Int, length int) string {
	chars := make([]byte, length)
	for i := range length {
		c := new(big.Int)
		_, c = mixedRadix.DivMod(mixedRadix, visibleCharCount, c)
		chars[i] = byte(c.Uint64() + 32)
	}
	return string(chars)
}

// unmarshalPrintableString decodes a PrintableString (7 bits per character).
func unmarshalPrintableString(mixedRadix *big.Int, length int) string {
	fillReversePrintableMap()
	chars := make([]byte, length)
	for i := range length {
		c := new(big.Int)
		_, c = mixedRadix.DivMod(mixedRadix, big.NewInt(int64(len(offsetToPrintable))), c)
		chars[i] = byte(offsetToPrintable[c.Int64()])
	}
	return string(chars)
}

// unmarshalUTF8String decodes a UTF8String (8 bits per byte).
func unmarshalUTF8String(mixedRadix *big.Int, length int) string {
	bytes := make([]byte, length)
	for i := range length {
		b := new(big.Int)
		_, b = mixedRadix.DivMod(mixedRadix, big.NewInt(256), b)
		bytes[i] = byte(b.Uint64())
	}
	return string(bytes)
}

// unmarshalSequenceOf decodes a slice as an ASN.1 SEQUENCE OF.
func unmarshalSequenceOf(mixedRadix *big.Int, v reflect.Value, opts asn1.FieldOptions) error {
	if opts.SizeMin == nil || opts.SizeMax == nil {
		return &asn1.Error{
			Op:     "unmarshal",
			Type:   v.Type().String(),
			Reason: "SEQUENCE OF requires size constraint",
		}
	}

	lowerBound := *opts.SizeMin
	upperBound := *opts.SizeMax

	// Determine the length
	var length int64
	if lowerBound == upperBound {
		// Fixed size
		length = lowerBound
	} else {
		// Variable size - read the length offset first
		rangeSize := big.NewInt(upperBound - lowerBound + 1)
		offset := new(big.Int)
		_, offset = mixedRadix.DivMod(mixedRadix, rangeSize, offset)
		length = offset.Int64() + lowerBound
	}

	// Create the slice
	elemType := v.Type().Elem()
	slice := reflect.MakeSlice(v.Type(), int(length), int(length))

	// Decode each element
	for i := int64(0); i < length; i++ {
		elem := slice.Index(int(i))
		// Pass empty options for elements - they should have their own constraints
		// defined by the element type's struct tags
		if err := UnmarshalValue(mixedRadix, elem, asn1.FieldOptions{}); err != nil {
			return &asn1.Error{
				Op:     "unmarshal",
				Type:   elemType.String(),
				Reason: fmt.Sprintf("element %d: %v", i, err),
			}
		}
	}

	v.Set(slice)
	return nil
}
