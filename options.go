package firevault

// A Firevault Options instance allows for
// the overriding of default options for
// validation, creation and updating methods.
//
// Options values are immutable.
// Each Options method creates a new instance
// - it does not modify the old.
type Options struct {
	skipValidation   bool
	allowEmptyFields []string
	modifyOriginal   bool
	method           methodType
	id               string
	updateFields     []string
	deleteFields     []string
}

// Create a new Options instance.
//
// A Firevault Options instance allows for
// the overriding of default options for
// validation, creation and updating methods.
//
// Options values are immutable.
// Each Options method creates a new instance
// - it does not modify the old.
func NewOptions() Options {
	return Options{}
}

// Skip all validations - the "name" rule,
// "omitempty" rules and "ignore" rule will
// still be honoured.
func (o Options) SkipValidation() Options {
	o.skipValidation = true
	return o
}

// Specify which field paths
// (using dot-separated strings) should ignore
// the "omitempty" (including method-specific)
// rules.
//
// This can be useful when zero values are
// needed only during a specific method call.
//
// If left empty, those rules will be honoured
// for all fields.
func (o Options) AllowEmptyFields(fields ...string) Options {
	o.allowEmptyFields = append(o.allowEmptyFields, fields...)
	return o
}

// Allows the updating of the original struct's
// values during transformations.
//
// Note: Using this option makes the struct
// validation thread-unsafe. Use with caution.
func (o Options) ModifyOriginal() Options {
	o.modifyOriginal = true
	return o
}

// Allows the application of the same rules
// as if performing a Create operation
// (e.g. "required_create"), i.e. perform
// the same validation as the one before
// document creation.
//
// Only used for validation method.
func (o Options) AsCreate() Options {
	o.method = create
	return o
}

// Allows the application of the same rules
// as if performing an Update operation
// (e.g. "required_update"), i.e. perform
// the same validation as the one before
// document updating.
//
// Only used for validation method.
func (o Options) AsUpdate() Options {
	o.method = update
	return o
}

// Specify custom doc ID. If left empty,
// Firestore will automatically create one.
//
// Only used for creation method.
func (o Options) CustomID(id string) Options {
	o.id = id
	return o
}

// Specify which field paths
// (using dot-separated strings) to be
// overwritten. Other fields on the existing
// document will be untouched.
//
// It is an error if a provided field path does
// not refer to a value in the data passed,
// unless it's also specified in DeleteFields.
//
// Only used for updating method.
func (o Options) UpdateFields(fields ...string) Options {
	o.updateFields = append(o.updateFields, fields...)
	return o
}

// Specify which field paths
// (using dot-separated strings) to be
// deleted, regardless of whether the provided
// paths refer to values in the data passed.
// Other fields on the existing document will
// be untouched.
//
// If this option has been used in conjunction
// with UpdateFields, any field paths passed in
// here must also be present in UpdateFields,
// otherwise they'll be ignored.
//
// DeleteFields circumvents the limitation that
// the Delete constant can only be used on fields
// of type interface{} (as it's a sentinel value).
//
// Only used for updating method.
func (o Options) DeleteFields(fields ...string) Options {
	o.deleteFields = append(o.deleteFields, fields...)
	return o
}
