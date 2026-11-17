package seal0

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// FeatureValue is the generated name for Value.
type FeatureValue = Value

type valueKind uint8

const (
	kindNone valueKind = iota
	kindBool
	kindNumber
	kindUnlimited
	kindEnum
)

// MaxNumber is the largest number value the API accepts (2^53-1).
const MaxNumber int64 = 1<<53 - 1

// Value is a feature value: a boolean, a whole number limit, the unlimited
// marker, or an enum option. The zero Value is invalid.
//
// Value only carries the wire encoding; whether a value is allowed for a
// feature is decided by the API.
type Value struct {
	kind valueKind
	b    bool
	n    int64
	s    string
}

// BoolValue returns a boolean value.
func BoolValue(b bool) Value { return Value{kind: kindBool, b: b} }

// NumberValue returns a whole-number limit from 0 to MaxNumber.
func NumberValue(n int64) Value { return Value{kind: kindNumber, n: n} }

// UnlimitedValue returns the unlimited marker for number features.
func UnlimitedValue() Value { return Value{kind: kindUnlimited} }

// EnumValue returns an enum option.
func EnumValue(option string) Value { return Value{kind: kindEnum, s: option} }

// Bool returns the boolean; ok is false for other kinds.
func (v Value) Bool() (b, ok bool) { return v.b, v.kind == kindBool }

// Limit returns a number value: the limit n, or unlimited=true. ok is false
// for non-number values.
func (v Value) Limit() (n int64, unlimited, ok bool) {
	switch v.kind {
	case kindNumber:
		return v.n, false, true
	case kindUnlimited:
		return 0, true, true
	default:
		return 0, false, false
	}
}

// IsUnlimited reports whether v is the unlimited marker.
func (v Value) IsUnlimited() bool { return v.kind == kindUnlimited }

// Enum returns the enum option; ok is false for other kinds.
func (v Value) Enum() (option string, ok bool) { return v.s, v.kind == kindEnum }

// String renders the value as it appears in JSON.
func (v Value) String() string {
	b, err := v.MarshalJSON()
	if err != nil {
		return "<invalid>"
	}
	return string(b)
}

var errInvalidValue = errors.New("seal0: invalid feature value")

// MarshalJSON encodes the public value encoding.
func (v Value) MarshalJSON() ([]byte, error) {
	switch v.kind {
	case kindBool:
		return json.Marshal(v.b)
	case kindNumber:
		return []byte(strconv.FormatInt(v.n, 10)), nil
	case kindUnlimited:
		return []byte(`"unlimited"`), nil
	case kindEnum:
		return json.Marshal(v.s)
	default:
		return nil, errInvalidValue
	}
}

// UnmarshalJSON decodes the public value encoding.
func (v *Value) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	switch x := raw.(type) {
	case bool:
		*v = BoolValue(x)
	case json.Number:
		n, err := strconv.ParseInt(x.String(), 10, 64)
		if err != nil {
			return fmt.Errorf("%w: %s", errInvalidValue, x)
		}
		*v = NumberValue(n)
	case string:
		if x == "unlimited" {
			*v = UnlimitedValue()
		} else {
			*v = EnumValue(x)
		}
	default:
		return fmt.Errorf("%w: %s", errInvalidValue, data)
	}
	return nil
}
