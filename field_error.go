package firevault

import (
	"fmt"
	"reflect"
)

// fieldError contains a single field's validation
// error, along with other properties that may be
// needed for error message creation.
// It complies with the FieldError interface.
type fieldError struct {
	collPath     string
	field        string
	structField  string
	displayField string
	path         string
	structPath   string
	value        reflect.Value
	kind         reflect.Kind
	typ          reflect.Type
	rule         string
	param        string
}

// FieldError interface gives access to all field
// validation error details, which aid in
// constructing a custom error message.
type FieldError interface {
	// Collection returns the path of the
	// collection that contains the document
	// modeled by the top-level struct.
	Collection() string
	// Field returns the field's name, with the tag
	// name taking precedence over the field's
	// struct name.
	Field() string
	// StructField returns the field's actual name
	// from the struct.
	StructField() string
	// DisplayField returns the field's struct name
	// in a human-readable form. It splits camel,
	// pascal, and snake case names into
	// space-separated words, including separating
	// adjacent letters and numbers
	// (e.g. "FirstName" -> "First Name").
	DisplayField() string
	// Path returns the field's dot-separated path,
	// with the tag names taking precedence over the
	// fields' struct names (e.g. "names.first").
	Path() string
	// StructPath returns the field's actual
	// dot-separated path from the stuct
	// (e.g. "Names.First").
	StructPath() string
	// Value returns the field's reflect Value.
	Value() reflect.Value
	// Kind returns the Value's reflect Kind
	// (eg. time.Time's kind is a struct).
	Kind() reflect.Kind
	// Type returns the Value's reflect Type
	// (eg. time.Time's type is time.Time).
	Type() reflect.Type
	// Rule returns the validation rule that failed.
	Rule() string
	// Param returns the param value, in string form
	// for comparison.
	Param() string
	// Error returns the error message.
	Error() string
}

// Collection returns the path of the
// collection that contains the document
// modeled by the top-level struct.
func (fe *fieldError) Collection() string {
	return fe.collPath
}

// Field returns the field's name, with the tag
// name taking precedence over the field's
// struct name.
func (fe *fieldError) Field() string {
	return fe.field
}

// StructField returns the field's actual name
// from the struct.
func (fe *fieldError) StructField() string {
	return fe.structField
}

// DisplayField returns the field's struct name
// in a human-readable form. It splits camel,
// pascal, and snake case names into
// space-separated words, including separating
// adjacent letters and numbers
// (e.g. "FirstName" -> "First Name").
func (fe *fieldError) DisplayField() string {
	return fe.displayField
}

// Path returns the field's dot-separated path,
// with the tag names taking precedence over the
// fields' struct names (e.g. "names.first").
func (fe *fieldError) Path() string {
	return fe.path
}

// StructPath returns the field's actual
// dot-separated path from the stuct
// (e.g. "Names.First").
func (fe *fieldError) StructPath() string {
	return fe.structPath
}

// Value returns the field's reflect Value.
func (fe *fieldError) Value() reflect.Value {
	return fe.value
}

// Kind returns the Value's reflect Kind
// (eg. time.Time's kind is a struct).
func (fe *fieldError) Kind() reflect.Kind {
	return fe.kind
}

// Type returns the Value's reflect Type
// (eg. time.Time's type is time.Time).
func (fe *fieldError) Type() reflect.Type {
	return fe.typ
}

// Rule returns the validation rule that failed.
func (fe *fieldError) Rule() string {
	return fe.rule
}

// Param returns the param value, in string form
// for comparison.
func (fe *fieldError) Param() string {
	return fe.param
}

// Error returns the error message.
func (fe *fieldError) Error() string {
	return fmt.Sprintf("firevault: field validation for '%s' failed on the '%s' rule", fe.field, fe.rule)
}
