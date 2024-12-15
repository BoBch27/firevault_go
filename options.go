package firevault

// A Firevault Options instance allows for the overriding of
// default options for validation, creation and updating methods.
//
// Options values are immutable. Each Options method creates
// a new instance - it does not modify the old.
type Options struct {
	// Skip all validations. Default is "false".
	skipValidation bool
	// Specify which field paths (using dot-separated strings)
	// should ignore the "omitempty" (including method-specific)
	// rules.
	//
	// This can be useful when zero values are needed only during
	// a specific method call.
	//
	// If left empty, those rules will be honoured for all fields.
	allowEmptyFields []string
	// Allows the updating of the original struct's values during
	// transformations. Default is "false".
	//
	// Note: Setting this to "true" makes the struct validation
	// thread-unsafe. Use with caution.
	modifyOriginal bool
	// Specify which field paths (using dot-separated strings)
	// to be overwritten. Other fields on the existing document
	// will be untouched.
	//
	// If a provided field path does not refer to a value in the
	// data passed, that field will be deleted from the document.
	//
	// If left empty, all the field paths given in the data argument
	// will be overwritten.
	//
	// Only used for updating method.
	mergeFields []string
	// Specify custom doc ID. If left empty, Firestore will
	// automatically create one.
	//
	// Only used for creation method.
	id string
}

// Create a new Options instance.
//
// A Firevault Options instance allows for the overriding of
// default options for validation, creation and updating methods.
//
// Options values are immutable. Each Options method creates
// a new instance - it does not modify the old.
func NewOptions() Options {
	return Options{}
}

// Skip all validations - the "name" rule, "omitempty" rules and
// "ignore" rule will still be honoured.
func (o Options) SkipValidation() Options {
	o.skipValidation = true
	return o
}

// Specify which field paths (using dot-separated strings)
// should ignore the "omitempty" (including method-specific)
// rules.
//
// This can be useful when zero values are needed only during
// a specific method call.
//
// If left empty, those rules will be honoured for all fields.
func (o Options) AllowEmptyFields(fields ...string) Options {
	o.allowEmptyFields = append(o.allowEmptyFields, fields...)
	return o
}

// Allows the updating of the original struct's values during
// transformations.
//
// Note: Using this option makes the struct validation
// thread-unsafe. Use with caution.
func (o Options) ModifyOriginal() Options {
	o.modifyOriginal = true
	return o
}

// Specify which field paths (using dot-separated strings)
// to be overwritten. Other fields on the existing document
// will be untouched.
//
// If a provided field path does not refer to a value in the
// data passed, that field will be deleted from the document.
//
// Only used for updating method.
func (o Options) MergeFields(fields ...string) Options {
	o.mergeFields = append(o.mergeFields, fields...)
	return o
}

// Specify custom doc ID. If left empty, Firestore will
// automatically create one.
//
// Only used for creation method.
func (o Options) CustomID(id string) Options {
	o.id = id
	return o
}
