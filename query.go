package firevault

// A Firevault Query helps to filter and order
// Firestore documents.
//
// Query values are immutable. Each Query method creates
// a new Query - it does not modify the old.
type Query struct {
	ids         []string
	filters     []filter
	orders      []order
	startAt     []interface{}
	startAfter  []interface{}
	endBefore   []interface{}
	endAt       []interface{}
	limit       int
	limitToLast int
	offset      int
}

// represents a single filter in a Query
type filter struct {
	path     string
	operator string
	value    interface{}
}

// represents a single order in a Query
type order struct {
	path      string
	direction Direction
}

// Create a new Query instance.
//
// A Firevault Query helps to filter and order
// Firestore documents.
//
// Query values are immutable. Each Query method creates
// a new Query - it does not modify the old.
func NewQuery() Query {
	return Query{}
}

// ID returns a new Query that exclusively filters the
// set of results based on provided IDs.
//
// ID takes precedence over and completely overrides
// any previous or subsequent calls to other Query
// methods, including Where.
//
// If you need to filter by ID as well as other criteria,
// use the Where method with the special DocumentID field,
// instead of calling ID.
//
// Calling ID overrides a previous call to the method.
func (q Query) ID(ids ...string) Query {
	q.ids = ids
	return q
}

// Where returns a new Query that filters the set of results.
// A Query can have multiple filters.
//
// The path argument can be a single field or a dot-separated
// sequence of fields, and must not contain any of
// the runes "˜*/[]".
//
// The operator argument must be one of "==", "!=", "<", "<=",
// ">", ">=", "array-contains", "array-contains-any", "in" or
// "not-in".
func (q Query) Where(path string, operator string, value interface{}) Query {
	q.filters = append(q.filters, filter{path, operator, value})
	return q
}

// OrderBy returns a new Query that specifies the order in which
// results are returned. A Query can have multiple OrderBy
// specifications. It appends the specification to the list of
// existing ones.
//
// The path argument can be a single field or a dot-separated
// sequence of fields, and must not contain any of the runes
// "˜*/[]".
//
// To order by document name, use the special field path
// DocumentID.
func (q Query) OrderBy(path string, direction Direction) Query {
	q.orders = append(q.orders, order{path, direction})
	return q
}

// StartAt returns a new Query that specifies that results
// should start at the document with the given field values.
//
// StartAt should be called with one field value for each
// OrderBy clause, in the order that they appear.
//
// If an OrderBy call uses the special DocumentID field path,
// the corresponding value should be the document ID relative
// to the query's collection.
//
// Calling StartAt overrides a previous call to StartAt or
// StartAfter.
func (q Query) StartAt(values ...interface{}) Query {
	q.startAt = values
	return q
}

// StartAfter returns a new Query that specifies that results
// should start just after the document with the given field values.
//
// StartAfter should be called with one field value for each
// OrderBy clause, in the order that they appear.
//
// If an OrderBy call uses the special DocumentID field path,
// the corresponding value should be the document ID relative
// to the query's collection.
//
// Calling StartAfter overrides a previous call to StartAt or
// StartAfter.
func (q Query) StartAfter(values ...interface{}) Query {
	q.startAfter = values
	return q
}

// EndBefore returns a new Query that specifies that results
// should end just before the document with the given field values.
//
// EndBefore should be called with one field value for each
// OrderBy clause, in the order that they appear.
//
// If an OrderBy call uses the special DocumentID field path,
// the corresponding value should be the document ID relative
// to the query's collection.
//
// Calling EndBefore overrides a previous call to EndAt or
// EndBefore.
func (q Query) EndBefore(values ...interface{}) Query {
	q.endBefore = values
	return q
}

// EndAt returns a new Query that specifies that results
// should end at the document with the given field values.
//
// EndAt should be called with one field value for each
// OrderBy clause, in the order that they appear.
//
// If an OrderBy call uses the special DocumentID field path,
// the corresponding value should be the document ID relative
// to the query's collection.
//
// Calling EndAt overrides a previous call to EndAt or
// EndBefore.
func (q Query) EndAt(values ...interface{}) Query {
	q.endAt = values
	return q
}

// Limit returns a new Query that specifies the maximum number of
// first results to return. It must not be negative.
func (q Query) Limit(num int) Query {
	q.limit = num
	return q
}

// LimitToLast returns a new Query that specifies the maximum number
// of last results to return. It must not be negative.
func (q Query) LimitToLast(num int) Query {
	q.limitToLast = num
	return q
}

// Offset returns a new Query that specifies the number of
// initial results to skip. It must not be negative.
func (q Query) Offset(num int) Query {
	q.offset = num
	return q
}
