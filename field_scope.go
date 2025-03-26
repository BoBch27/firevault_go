package firevault

import "reflect"

// fieldScope contains a single field's
// information to help validate it.
// It complies with the FieldScope interface.
type fieldScope struct {
	strct        reflect.Value
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
	// used for caching
	pointer   bool
	dive      bool
	dynamic   bool
	omitEmpty methodType
	rules     []*ruleData
}

// ruleData contains the information
// about each parsed rule.
type ruleData struct {
	name        string
	valFn       valFuncInternal
	transFn     TransformationFn
	param       string
	isTransform bool
	runOnNil    bool
	methodOnly  methodType
}

// A Firevault FieldScope interface gives access
// to all information needed to validate a field.
type FieldScope interface {
	// Struct returns the reflected parent struct
	// of the current field, if any.
	Struct() reflect.Value
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
	// fields' actual names (e.g. "names.first").
	Path() string
	// StructPath returns the field's actual
	// dot-separated path from the stuct
	// (e.g. "Names.First").
	StructPath() string
	// Value returns the current field's reflected
	// value to be validated.
	Value() reflect.Value
	// Kind returns the Value's reflect Kind
	// (eg. time.Time's kind is a struct).
	Kind() reflect.Kind
	// Type returns the Value's reflect Type
	// (eg. time.Time's type is time.Time).
	Type() reflect.Type
	// Rule returns the current validation's rule name.
	Rule() string
	// Param returns the param value, in string form
	// for comparison.
	Param() string
}

// Struct returns the reflected parent struct
// of the current field, if any.
func (fs *fieldScope) Struct() reflect.Value {
	return fs.strct
}

// Field returns the field's name, with the tag
// name taking precedence over the field's
// actual name.
func (fs *fieldScope) Field() string {
	return fs.field
}

// StructField returns the field's actual name
// from the struct.
func (fs *fieldScope) StructField() string {
	return fs.structField
}

// DisplayField returns the field's struct name
// in a human-readable form. It splits camel,
// pascal, and snake case names into
// space-separated words, including separating
// adjacent letters and numbers
// (e.g. "FirstName" -> "First Name").
func (fs *fieldScope) DisplayField() string {
	return fs.displayField
}

// Path returns the field's dot-separated path,
// with the tag names taking precedence over the
// fields' actual names (e.g. "names.first").
func (fs *fieldScope) Path() string {
	return fs.path
}

// StructPath returns the field's actual
// dot-separated path from the stuct
// (e.g. "Names.First").
func (fs *fieldScope) StructPath() string {
	return fs.structPath
}

// Value returns the current field's reflected
// value to be validated.
func (fs *fieldScope) Value() reflect.Value {
	return fs.value
}

// Kind returns the Value's reflect Kind
// (eg. time.Time's kind is a struct).
func (fs *fieldScope) Kind() reflect.Kind {
	return fs.kind
}

// Type returns the Value's reflect Type
// (eg. time.Time's type is time.Time).
func (fs *fieldScope) Type() reflect.Type {
	return fs.typ
}

// Rule returns the current validation's rule name.
func (fs *fieldScope) Rule() string {
	return fs.rule
}

// Param returns the param value, in string form
// for comparison.
func (fs *fieldScope) Param() string {
	return fs.param
}
