package seal0

import "encoding/json"

// Patch is a field in a PATCH request: unset (the zero value) leaves the
// field unchanged, Set changes it, and Null clears it.
type Patch[T any] struct {
	set, null bool
	value     T
}

// Set returns a Patch that changes the field to v.
func Set[T any](v T) Patch[T] { return Patch[T]{set: true, value: v} }

// Null returns a Patch that clears the field.
func Null[T any]() Patch[T] { return Patch[T]{set: true, null: true} }

// IsSet reports whether the field will be sent.
func (p Patch[T]) IsSet() bool { return p.set }

// patchBody collects the fields of a PATCH request that are set.
type patchBody map[string]any

func (m patchBody) add(name string, p interface{ patchValue() (any, bool) }) {
	if v, ok := p.patchValue(); ok {
		m[name] = v
	}
}

func (p Patch[T]) patchValue() (any, bool) {
	switch {
	case !p.set:
		return nil, false
	case p.null:
		return nil, true // encoded as JSON null
	default:
		return p.value, true
	}
}

func (m patchBody) marshal() ([]byte, error) { return json.Marshal(map[string]any(m)) }
