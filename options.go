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
	skipValFields    []string
	allowEmptyFields []string
	modifyOriginal   bool
	method           methodType
	id               string
	disableMerge     bool
	mergeFields      []string
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
//
// If no field paths are provided, validation
// will be skipped for all fields.
// Otherwise, validation will only be skipped
// for the specified field paths
// (using dot-separated strings).
//
// Calling this method without specifying
// fields will override any previous calls that
// specified particular fields, ensuring
// validation is skipped for all fields.
func (o Options) SkipValidation(fields ...string) Options {
	o.skipValidation = true

	if len(fields) > 0 {
		o.skipValFields = append(o.skipValFields, fields...)
	} else {
		o.skipValFields = []string{}
	}

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
// Only applies to the Validate method.
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
// Only applies to the Validate method.
func (o Options) AsUpdate() Options {
	o.method = update
	return o
}

// Specify custom doc ID. If left empty,
// Firestore will automatically create one.
//
// Only applies to the Create method.
func (o Options) CustomID(id string) Options {
	o.id = id
	return o
}

// Disable the merging of fields, meaning the
// entire document will be replaced - no
// existing fields will be preserved.
//
// The deletion of fields is based on the
// provided struct, not the Firestore document
// itself. If the struct has changed since the
// document was created, some fields may not be
// deleted.
//
// This option overrides any previous calls to
// MergeFields.
//
// Only applies to the Update method.
func (o Options) DisableMerge() Options {
	o.disableMerge = true
	o.mergeFields = []string{}
	return o
}

// Specify which field paths
// (using dot-separated strings) to be
// overwritten. Other fields on the existing
// document will be untouched.
//
// If a provided field path does not refer to
// a value in the data passed, it'll be ignored.
//
// This option overrides any previous calls to
// DisableFields.
//
// Only applies to the Update method.
func (o Options) MergeFields(fields ...string) Options {
	o.mergeFields = append(o.mergeFields, fields...)
	o.disableMerge = false
	return o
}
