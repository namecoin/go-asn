package asn1

import (
	"fmt"
	"strconv"
	"strings"
)

// StringType identifies the ASN.1 string type for encoding.
type StringType int

const (
	// StringTypeUTF8 represents UTF8String (default).
	StringTypeUTF8 StringType = iota
	// StringTypeIA5 represents IA5String (ASCII subset).
	StringTypeIA5
	// StringTypePrintable represents PrintableString.
	StringTypePrintable
	// StringTypeVisible represents VisibleString.
	StringTypeVisible
)

// FieldOptions holds the parsed options from an asn1 struct tag.
type FieldOptions struct {
	Optional   bool        // Field is OPTIONAL
	Tag        *int        // Explicit tag number
	SizeMin    *int64      // SIZE constraint lower bound
	SizeMax    *int64      // SIZE constraint upper bound
	Default    interface{} // DEFAULT value
	StringType StringType  // String encoding type
	Choice     *int        // CHOICE index
	Extensible bool        // Has extension marker (...)
}

// ParseTag parses an asn1 struct tag and returns the field options.
// Tag format: "option1,option2,key:value,..."
//
// Supported options:
//   - optional: field is OPTIONAL
//   - extensible: type has extension marker
//   - tag:N: explicit context tag number
//   - size:N or size:MIN..MAX: SIZE constraint
//   - choice:N: CHOICE alternative index
//   - default:N: DEFAULT value (integers only)
//   - ia5string, utf8string, printablestring, visiblestring: string type
func ParseTag(tag string) (FieldOptions, error) {
	var opts FieldOptions

	if tag == "" {
		return opts, nil
	}

	parts := strings.Split(tag, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Handle key:value pairs
		if idx := strings.Index(part, ":"); idx != -1 {
			key := part[:idx]
			value := part[idx+1:]

			switch key {
			case "tag":
				n, err := strconv.Atoi(value)
				if err != nil {
					return opts, fmt.Errorf("invalid tag value %q: %w", value, err)
				}
				opts.Tag = &n

			case "size":
				if err := parseSize(value, &opts); err != nil {
					return opts, err
				}

			case "choice":
				n, err := strconv.Atoi(value)
				if err != nil {
					return opts, fmt.Errorf("invalid choice value %q: %w", value, err)
				}
				opts.Choice = &n

			case "default":
				n, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return opts, fmt.Errorf("invalid default value %q: %w", value, err)
				}
				opts.Default = n

			default:
				return opts, fmt.Errorf("unknown tag option %q", key)
			}
			continue
		}

		// Handle boolean flags and string types
		switch part {
		case "optional":
			opts.Optional = true
		case "extensible":
			opts.Extensible = true
		case "ia5string":
			opts.StringType = StringTypeIA5
		case "utf8string":
			opts.StringType = StringTypeUTF8
		case "printablestring":
			opts.StringType = StringTypePrintable
		case "visiblestring":
			opts.StringType = StringTypeVisible
		default:
			return opts, fmt.Errorf("unknown tag option %q", part)
		}
	}

	return opts, nil
}

// parseSize parses a size constraint: "N" or "MIN..MAX"
func parseSize(value string, opts *FieldOptions) error {
	if idx := strings.Index(value, ".."); idx != -1 {
		// Range: MIN..MAX
		minStr := value[:idx]
		maxStr := value[idx+2:]

		lowerBound, err := strconv.ParseInt(minStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid size minimum %q: %w", minStr, err)
		}
		upperBound, err := strconv.ParseInt(maxStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid size maximum %q: %w", maxStr, err)
		}

		opts.SizeMin = &lowerBound
		opts.SizeMax = &upperBound
	} else {
		// Single value: both min and max are the same
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid size value %q: %w", value, err)
		}
		opts.SizeMin = &n
		opts.SizeMax = &n
	}
	return nil
}
