package firevault

import "cloud.google.com/go/firestore"

// DocumentID is the special field name representing
// the ID of a document in queries.
const DocumentID = firestore.DocumentID

// Direction is the sort direction for result ordering.
type Direction = firestore.Direction

// Asc sorts results from smallest to largest.
const Asc Direction = firestore.Asc

// Desc sorts results from largest to smallest.
const Desc Direction = firestore.Desc
