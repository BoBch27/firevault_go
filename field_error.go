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
	code        string
	tag         string
	field       string
	structField string
	value       interface{}
	param       string
	kind        reflect.Kind
	typ         reflect.Type
}

// A Firevault FieldError interface gives access
// to all field validation error details,
// which aid in constructing a custom error message.
type FieldError interface {
	// Code returns a reason for the error
	// (e.g. unknown-validation-rule).
	Code() string
	// Tag returns the validation tag that failed.
	Tag() string
	// Field returns the field's name, with the tag
	// name taking precedence over the field's
	// struct name.
	Field() string
	// StructField returns the field's actual name
	// from the struct.
	StructField() string
	// Value returns the field's actual value.
	Value() interface{}
	// Param returns the param value, in string form
	// for comparison.
	Param() string
	// Kind returns the Value's reflect Kind
	// (eg. time.Time's kind is a struct).
	Kind() reflect.Kind
	// Type returns the Value's reflect Type
	// (eg. time.Time's type is time.Time).
	Type() reflect.Type
	// Error returns the error message.
	Error() string
}

// Code returns the error code.
func (fe *fieldError) Code() string {
	return fe.code
}

// Tag returns the validation tag that failed.
func (fe *fieldError) Tag() string {
	return fe.tag
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

// Value returns the field's actual value.
func (fe *fieldError) Value() interface{} {
	return fe.value
}

// Param returns the param value, in string form
// for comparison.
func (fe *fieldError) Param() string {
	return fe.param
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

// Error returns the error message.
func (fe *fieldError) Error() string {
	return fmt.Sprintf("firevault: field validation for '%s' failed on the '%s' tag", fe.field, fe.tag)
}
