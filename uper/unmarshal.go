package uper

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/namecoin/go-asn/asn1"
)

// Unmarshal decodes UPER (Unaligned Packed Encoding Rules) data into a value.
// The value must be a pointer to a struct or a basic type.
func Unmarshal(data []byte, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &asn1.Error{Op: "unmarshal", Type: "nil", Reason: "value must be a non-nil pointer"}
	}

	r := asn1.NewBitReader(data, false) // UPER is unaligned
	return UnmarshalValue(r, rv.Elem(), asn1.FieldOptions{})
}

// Namecoin: Public in order to facilitate using an out of band length for SEQUENCE OF.
// UnmarshalValue decodes a single value based on its type.
func UnmarshalValue(r *asn1.BitReader, v reflect.Value, opts asn1.FieldOptions) error {
	// Handle pointers - allocate if nil
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Bool:
		return unmarshalBool(r, v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return unmarshalInt(r, v, opts)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return unmarshalUint(r, v, opts)
	case reflect.Struct:
		return unmarshalStruct(r, v)
	case reflect.String:
		return unmarshalString(r, v, opts)
	case reflect.Slice:
		// Check if this is a byte slice (OCTET STRING)
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return unmarshalOctetString(r, v, opts)
		}
		return unmarshalSequenceOf(r, v, opts)
	default:
		return &asn1.Error{
			Op:     "unmarshal",
			Type:   v.Type().String(),
			Reason: fmt.Sprintf("unsupported type: %s", v.Kind()),
		}
	}
}

// unmarshalBool decodes a boolean from a single bit.
// In UPER, 1 is true and 0 is false.
func unmarshalBool(r *asn1.BitReader, v reflect.Value) error {
	bit, err := r.ReadBits(1)
	if err != nil {
		return &asn1.Error{
			Op:     "unmarshal",
			Type:   "bool",
			Reason: fmt.Sprintf("failed to read bit: %v", err),
		}
	}
	v.SetBool(bit == 1)
	return nil
}

func unmarshalAnyInt(r *asn1.BitReader, v reflect.Value, opts asn1.FieldOptions, unsigned bool) error {
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
	rangeSize := upperBound - lowerBound + 1
	numBits := bitsNeeded(uint64(rangeSize - 1))

	// Read the offset value
	offset, err := r.ReadBits(numBits)
	if err != nil {
		return &asn1.Error{
			Op:     "unmarshal",
			Type:   typeName,
			Reason: fmt.Sprintf("failed to read %d bits: %v", numBits, err),
		}
	}

	// Calculate the actual value by adding the minimum
	if unsigned {
		value := offset + uint64(lowerBound)
		v.SetUint(value)
	} else {
		value := int64(offset) + lowerBound
		v.SetInt(value)
	}

	return nil
}

// unmarshalInt decodes a constrained integer using UPER encoding.
// The value is decoded as an offset from the minimum, using the minimum
// number of bits required to represent the range.
func unmarshalInt(r *asn1.BitReader, v reflect.Value, opts asn1.FieldOptions) error {
	return unmarshalAnyInt(r, v, opts, false)
}

// unmarshalUint decodes a constrained unsigned integer using UPER encoding.
func unmarshalUint(r *asn1.BitReader, v reflect.Value, opts asn1.FieldOptions) error {
	return unmarshalAnyInt(r, v, opts, true)
}

// unmarshalStruct decodes each exported field of a struct in sequence.
// For UPER, optional fields are decoded with a presence bitmap (preamble)
// that precedes all field values.
func unmarshalStruct(r *asn1.BitReader, v reflect.Value) error {
	t := v.Type()

	// Check if this struct represents a CHOICE type
	if isChoiceStruct(v) {
		return unmarshalChoice(r, v)
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
		bit, err := r.ReadBits(1)
		if err != nil {
			return &asn1.Error{
				Op:     "unmarshal",
				Type:   t.Name(),
				Reason: fmt.Sprintf("failed to read optional preamble: %v", err),
			}
		}
		optionalFields[i].present = bit == 1
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

		if err := UnmarshalValue(r, field, opts); err != nil {
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
func unmarshalChoice(r *asn1.BitReader, v reflect.Value) error {
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
	numBits := bitsNeeded(uint64(numAlternatives - 1))
	choiceIdx, err := r.ReadBits(numBits)
	if err != nil {
		return &asn1.Error{
			Op:     "unmarshal",
			Type:   t.Name(),
			Reason: fmt.Sprintf("failed to read choice index: %v", err),
		}
	}

	// Find the alternative with the matching choice index
	var selectedAlt *choiceAlt
	for i := range alternatives {
		if alternatives[i].choiceIndex == int(choiceIdx) {
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

	if err := UnmarshalValue(r, target, selectedAlt.opts); err != nil {
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
func unmarshalOctetString(r *asn1.BitReader, v reflect.Value, opts asn1.FieldOptions) error {
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
		numBits := bitsNeeded(uint64(rangeSize - 1))
		offset, err := r.ReadBits(numBits)
		if err != nil {
			return &asn1.Error{
				Op:     "unmarshal",
				Type:   "[]byte",
				Reason: fmt.Sprintf("failed to read length: %v", err),
			}
		}
		length = int64(offset) + lowerBound
	}

	// Read the data bytes
	data := make([]byte, length)
	for i := int64(0); i < length; i++ {
		b, err := r.ReadBits(8)
		if err != nil {
			return &asn1.Error{
				Op:     "unmarshal",
				Type:   "[]byte",
				Reason: fmt.Sprintf("failed to read byte %d: %v", i, err),
			}
		}
		data[i] = byte(b)
	}

	v.SetBytes(data)
	return nil
}

// unmarshalString decodes a string using UPER encoding.
func unmarshalString(r *asn1.BitReader, v reflect.Value, opts asn1.FieldOptions) error {
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
		rangeSize := upperBound - lowerBound + 1
		numBits := bitsNeeded(uint64(rangeSize - 1))
		offset, err := r.ReadBits(numBits)
		if err != nil {
			return &asn1.Error{
				Op:     "unmarshal",
				Type:   "string",
				Reason: fmt.Sprintf("failed to read length: %v", err),
			}
		}
		length = int64(offset) + lowerBound
	}

	// Decode the characters based on the string type
	var s string
	var err error

	switch opts.StringType {
	case asn1.StringTypeIA5:
		s, err = unmarshalIA5String(r, int(length))
	case asn1.StringTypeVisible:
		s, err = unmarshalVisibleString(r, int(length))
	case asn1.StringTypePrintable:
		s, err = unmarshalPrintableString(r, int(length))
	default: // UTF8 is the default
		s, err = unmarshalUTF8String(r, int(length))
	}

	if err != nil {
		return err
	}

	v.SetString(s)
	return nil
}

// unmarshalIA5String decodes an IA5String (7 bits per character).
func unmarshalIA5String(r *asn1.BitReader, length int) (string, error) {
	chars := make([]byte, length)
	for i := 0; i < length; i++ {
		c, err := r.ReadBits(7)
		if err != nil {
			return "", &asn1.Error{
				Op:     "unmarshal",
				Type:   "string",
				Reason: fmt.Sprintf("failed to read IA5 character %d: %v", i, err),
			}
		}
		chars[i] = byte(c)
	}
	return string(chars), nil
}

// unmarshalVisibleString decodes a VisibleString (7 bits per character).
func unmarshalVisibleString(r *asn1.BitReader, length int) (string, error) {
	chars := make([]byte, length)
	for i := 0; i < length; i++ {
		c, err := r.ReadBits(7)
		if err != nil {
			return "", &asn1.Error{
				Op:     "unmarshal",
				Type:   "string",
				Reason: fmt.Sprintf("failed to read VisibleString character %d: %v", i, err),
			}
		}
		chars[i] = byte(c)
	}
	return string(chars), nil
}

// unmarshalPrintableString decodes a PrintableString (7 bits per character).
func unmarshalPrintableString(r *asn1.BitReader, length int) (string, error) {
	chars := make([]byte, length)
	for i := 0; i < length; i++ {
		c, err := r.ReadBits(7)
		if err != nil {
			return "", &asn1.Error{
				Op:     "unmarshal",
				Type:   "string",
				Reason: fmt.Sprintf("failed to read PrintableString character %d: %v", i, err),
			}
		}
		chars[i] = byte(c)
	}
	return string(chars), nil
}

// unmarshalUTF8String decodes a UTF8String (8 bits per byte).
func unmarshalUTF8String(r *asn1.BitReader, length int) (string, error) {
	bytes := make([]byte, length)
	for i := 0; i < length; i++ {
		b, err := r.ReadBits(8)
		if err != nil {
			return "", &asn1.Error{
				Op:     "unmarshal",
				Type:   "string",
				Reason: fmt.Sprintf("failed to read UTF8 byte %d: %v", i, err),
			}
		}
		bytes[i] = byte(b)
	}
	return string(bytes), nil
}

// unmarshalSequenceOf decodes a slice as an ASN.1 SEQUENCE OF.
func unmarshalSequenceOf(r *asn1.BitReader, v reflect.Value, opts asn1.FieldOptions) error {
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
		rangeSize := upperBound - lowerBound + 1
		numBits := bitsNeeded(uint64(rangeSize - 1))
		offset, err := r.ReadBits(numBits)
		if err != nil {
			return &asn1.Error{
				Op:     "unmarshal",
				Type:   v.Type().String(),
				Reason: fmt.Sprintf("failed to read length: %v", err),
			}
		}
		length = int64(offset) + lowerBound
	}

	// Create the slice
	elemType := v.Type().Elem()
	slice := reflect.MakeSlice(v.Type(), int(length), int(length))

	// Decode each element
	for i := int64(0); i < length; i++ {
		elem := slice.Index(int(i))
		// Pass empty options for elements - they should have their own constraints
		// defined by the element type's struct tags
		if err := UnmarshalValue(r, elem, asn1.FieldOptions{}); err != nil {
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
