package firevault

import (
	"time"

	"cloud.google.com/go/firestore"
)

// Options allows for the overriding of default
// options for CollectionRef methods.
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
	precondition     firestore.Precondition
	transaction      *Transaction
}

// Create a new Options instance.
//
// Options allows for the overriding of default
// options for CollectionRef methods.
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
//
// Multiple calls to the method, with specified
// fields, are cumulative.
//
// Only applies to the Validate, Create and
// Update methods.
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
// Multiple calls to the method are cumulative.
//
// Only applies to the Validate, Create and
// Update methods.
func (o Options) AllowEmptyFields(fields ...string) Options {
	o.allowEmptyFields = append(o.allowEmptyFields, fields...)
	return o
}

// Allows the updating of the original struct's
// values during transformations.
//
// Note: Using this option makes the struct
// validation thread-unsafe. Use with caution.
//
// Only applies to the Validate, Create and
// Update methods.
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

// Ensure that the entire document is replaced
// with the passed in data, meaning no existing
// fields will be preserved. This works like
// Firestore's Set operation with disabled
// merging.
//
// The deletion of fields is based on the
// provided struct, not the Firestore document
// itself. If the struct has changed since the
// document was created, some fields may not be
// deleted.
//
// This option overrides any previous calls to
// ReplaceFields.
//
// Only applies to the Update method.
func (o Options) ReplaceAll() Options { // previously DisableMerge
	o.disableMerge = true
	o.mergeFields = []string{}
	return o
}

// Specify which field paths
// (using dot-separated strings) to be fully
// overwritten. Other fields on the existing
// document will be untouched. This works like
// Firestore's Set operation with specified
// fields to merge.
//
// If a provided field path does not refer to
// a value in the data passed, it'll be ignored.
//
// This option overrides any previous calls to
// ReplaceAll.
//
// Multiple calls to the method are cumulative.
//
// Only applies to the Update method.
func (o Options) ReplaceFields(fields ...string) Options { // previously MergeFields
	o.mergeFields = append(o.mergeFields, fields...)
	o.disableMerge = false
	return o
}

// Set a precondition that the document must
// exist and have the specified last update
// timestamp before applying an update.
//
// The operation will only proceed if the
// document's last update time matches the
// given timestamp exactly.
//
// Timestamp must be microsecond aligned.
//
// This option overrides any previous calls to
// RequireExists.
//
// Only applies to the Update and Delete
// methods.
func (o Options) RequireLastUpdateTime(t time.Time) Options {
	o.precondition = firestore.LastUpdateTime(t)
	return o
}

// Set a precondition that the document must
// exist before proceeding with the operation.
//
// This option overrides any previous calls to
// RequireLastUpdateTime.
//
// Only applies to the Delete method.
func (o Options) RequireExists() Options {
	o.precondition = firestore.Exists
	return o
}

// Execute the operation within the provided
// transaction.
//
// If set, all reads and writes performed by
// this operation will be executed as part
// of the given transaction, ensuring
// atomicity and automatic rollback on
// failure.
//
// This option overrides any previous calls
// to Transaction.
//
// Does not apply to the Validate method.
func (o Options) Transaction(t *Transaction) Options {
	o.transaction = t
	return o
}
