package firevault

import "cloud.google.com/go/firestore"

// Transaction is an alias of Firestore's
// Transaction.
type Transaction = firestore.Transaction

// DocumentID is the special field name representing
// the ID of a document in queries.
const DocumentID = firestore.DocumentID

// Direction is the sort direction for result ordering.
type Direction = firestore.Direction

// Asc sorts results from smallest to largest.
const Asc Direction = firestore.Asc

// Desc sorts results from largest to smallest.
const Desc Direction = firestore.Desc

// ServerTimestamp is used as a value in a call to
// Update to indicate that the key's value should be
// set to the time at which the server processed the
// request.
//
// ServerTimestamp must be the value of a field
// directly; it cannot appear in array or struct
// values, or in any value that is itself inside an
// array or struct.
const ServerTimestamp = firestore.ServerTimestamp

// Delete is used as a value in a call to Update
// to indicate that the corresponding key should be
// deleted.
const Delete = firestore.Delete

// Increment returns a value that can be used in Update
// operations to increment a numeric value atomically.
//
// If the field does not yet exist, the transformation
// will set the field to the given value.
//
// The supported values are:
//
//	int, int8, int16, int32, int64
//	uint8, uint16, uint32
//	float32, float64
func Increment(n interface{}) interface{} {
	return firestore.Increment(n)
}

// ArrayUnion specifies elements to be added to
// whatever array already exists, or to create an
// array if no value exists.
//
// If a value exists and it's an array, values are
// appended to it. Any duplicate value is ignored.
// If a value exists and it's not an array, the value
// is replaced by an array of the values in
// the ArrayUnion. If a value does not exist, an
// array of the values in the ArrayUnion is created.
//
// ArrayUnion must be the value of a field directly;
// it cannot appear in array or struct values, or in
// any value that is itself inside an array or struct.
func ArrayUnion(elements ...interface{}) interface{} {
	return firestore.ArrayUnion(elements...)
}

// ArrayRemove specifies elements to be removed from
// whatever array already exists.
//
// If a value exists and it's an array, values are
// removed from it. All duplicate values are removed.
// If a value exists and it's not an array, the value
// is replaced by an empty array. If a value does not
// exist, an empty array is created.
//
// ArrayRemove must be the value of a field directly;
// it cannot appear in array or struct values, or in
// any value that is itself inside an array or struct.
func ArrayRemove(elements ...interface{}) interface{} {
	return firestore.ArrayRemove(elements...)
}
