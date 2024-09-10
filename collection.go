package firevault

import (
	"context"
	"errors"
	"strings"
	"sync"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"google.golang.org/api/iterator"
)

// A Firevault Collection allows for the fetching and
// modifying (with validation) of documents in a
// Firestore Collection.
type Collection[T interface{}] struct {
	connection *Connection
	ref        *firestore.CollectionRef
}

// A Firevault Document holds the ID and data related to
// fetched document.
type Document[T interface{}] struct {
	ID   string
	Data T
}

// Create a new Collection instance.
//
// A Firevault Collection allows for the fetching and
// modifying (with validation) of documents in a
// Firestore Collection.
func NewCollection[T interface{}](connection *Connection, path string) (*Collection[T], error) {
	if path == "" {
		return nil, errors.New("firevault: collection path cannot be empty")
	}

	collection := &Collection[T]{
		connection,
		connection.client.Collection(path),
	}

	return collection, nil
}

// Validate provided data.
func (c *Collection[T]) Validate(ctx context.Context, data *T, opts ...Options) error {
	valOptions, _, _ := c.parseOptions(validate, opts...)

	_, err := c.connection.validator.validate(ctx, data, valOptions)
	return err
}

// Create a Firestore document with provided data (after validation).
func (c *Collection[T]) Create(ctx context.Context, data *T, opts ...Options) (string, error) {
	valOptions, id, _ := c.parseOptions(create, opts...)

	dataMap, err := c.connection.validator.validate(ctx, data, valOptions)
	if err != nil {
		return "", err
	}

	if id == "" {
		docRef, _, err := c.ref.Add(ctx, dataMap)
		if err != nil {
			return "", err
		}

		id = docRef.ID
	} else {
		_, err = c.ref.Doc(id).Set(ctx, dataMap)
		if err != nil {
			return "", err
		}
	}

	return id, nil
}

// Update all Firestore documents which match provided Query
// (after data validation). The operation is not atomic.
func (c *Collection[T]) Update(ctx context.Context, query Query, data *T, opts ...Options) error {
	valOptions, _, mergeFields := c.parseOptions(update, opts...)

	dataMap, err := c.connection.validator.validate(ctx, data, valOptions)
	if err != nil {
		return err
	}

	return c.bulkOperation(ctx, query, func(bw *firestore.BulkWriter, docID string) error {
		_, err := bw.Set(c.ref.Doc(docID), dataMap, mergeFields)
		return err
	})
}

// Delete all Firestore documents which match provided Query.
// The operation is not atomic.
func (c *Collection[T]) Delete(ctx context.Context, query Query) error {
	return c.bulkOperation(ctx, query, func(bw *firestore.BulkWriter, docID string) error {
		_, err := bw.Delete(c.ref.Doc(docID))
		return err
	})
}

// Find all Firestore documents which match provided Query.
func (c *Collection[T]) Find(ctx context.Context, query Query) ([]Document[T], error) {
	if len(query.ids) > 0 {
		return c.fetchDocsByID(ctx, query.ids)
	}

	return c.fetchDocsByQuery(ctx, query)
}

// Find the first Firestore document which matches provided Query.
func (c *Collection[T]) FindOne(ctx context.Context, query Query) (Document[T], error) {
	if len(query.ids) > 0 {
		docs, err := c.fetchDocsByID(ctx, query.ids[0:1])
		if err != nil {
			return Document[T]{}, err
		}

		return docs[0], nil
	}

	docs, err := c.fetchDocsByQuery(ctx, query.Limit(1))
	if err != nil {
		return Document[T]{}, err
	}

	return docs[0], nil
}

// Find number of Firestore documents which match provided Query.
func (c *Collection[T]) Count(ctx context.Context, query Query) (int64, error) {
	if len(query.ids) > 0 {
		return int64(len(query.ids)), nil
	}

	builtQuery := c.buildQuery(query)
	results, err := builtQuery.NewAggregationQuery().WithCount("all").Get(ctx)
	if err != nil {
		return 0, err
	}

	count, ok := results["all"]
	if !ok {
		return 0, errors.New("firestore: couldn't get alias for COUNT from results")
	}

	countValue := count.(*firestorepb.Value)
	countInt := countValue.GetIntegerValue()

	return countInt, nil
}

// extract passed options
func (c *Collection[T]) parseOptions(
	method methodType,
	opts ...Options,
) (validationOpts, string, firestore.SetOption) {
	options := validationOpts{
		method:             method,
		skipValidation:     false,
		emptyFieldsAllowed: make([]string, 0),
	}

	if len(opts) == 0 {
		return options, "", firestore.MergeAll
	}

	// parse options
	passedOpts := opts[0]

	if passedOpts.skipValidation {
		options.skipValidation = true
	}

	if len(passedOpts.allowEmptyFields) > 0 {
		options.emptyFieldsAllowed = passedOpts.allowEmptyFields
	}

	if method == update && len(passedOpts.mergeFields) > 0 {
		fps := make([]firestore.FieldPath, 0)

		for i := 0; i < len(passedOpts.mergeFields); i++ {
			fp := firestore.FieldPath(strings.Split(passedOpts.mergeFields[i], "."))
			fps = append(fps, fp)
		}

		return options, passedOpts.id, firestore.Merge(fps...)
	}

	return options, passedOpts.id, firestore.MergeAll
}

// build a new firestore query
func (c *Collection[T]) buildQuery(query Query) firestore.Query {
	newQuery := c.ref.Query

	for _, filter := range query.filters {
		newQuery = newQuery.Where(filter.path, filter.operator, filter.value)
	}

	for _, order := range query.orders {
		newQuery = newQuery.OrderBy(order.path, firestore.Direction(order.direction))
	}

	if len(query.startAt) > 0 {
		newQuery = newQuery.StartAt(query.startAt...)
	}

	if len(query.startAfter) > 0 {
		newQuery = newQuery.StartAfter(query.startAfter...)
	}

	if len(query.endBefore) > 0 {
		newQuery = newQuery.EndBefore(query.endBefore...)
	}

	if len(query.endAt) > 0 {
		newQuery = newQuery.EndAt(query.endAt...)
	}

	if query.limit > 0 {
		newQuery = newQuery.Limit(query.limit)
	}

	if query.limitToLast > 0 {
		newQuery = newQuery.LimitToLast(query.limitToLast)
	}

	if query.offset > 0 {
		newQuery = newQuery.Offset(query.offset)
	}

	return newQuery
}

// perform a bulk operation
func (c *Collection[T]) bulkOperation(
	ctx context.Context,
	query Query,
	operation func(*firestore.BulkWriter, string) error,
) error {
	bulkWriter := c.connection.client.BulkWriter(ctx)
	defer bulkWriter.End()

	var mu sync.Mutex
	var errs []error

	docIDs := query.ids

	if len(docIDs) == 0 {
		docs, err := c.Find(ctx, query)
		if err != nil {
			return err
		}

		for _, doc := range docs {
			docIDs = append(docIDs, doc.ID)
		}
	}

	for _, docID := range docIDs {
		err := operation(bulkWriter, docID)
		if err != nil {
			mu.Lock()
			errs = append(errs, errors.New(err.Error()+" (docID: "+docID+")"))
			mu.Unlock()
		}
	}

	// wait for all operations to complete
	bulkWriter.Flush()

	return errors.Join(errs...)
}

// fetch documents based on provided ids
func (c *Collection[T]) fetchDocsByID(ctx context.Context, ids []string) ([]Document[T], error) {
	const batchSize = 100
	var docRefs []*firestore.DocumentRef
	var docs []Document[T]

	for _, docID := range ids {
		docRefs = append(docRefs, c.ref.Doc(docID))
	}

	for i := 0; i < len(docRefs); i += batchSize {
		end := i + batchSize
		if end > len(docRefs) {
			end = len(docRefs)
		}

		batchRefs := docRefs[i:end]
		snapshots, err := c.connection.client.GetAll(ctx, batchRefs)
		if err != nil {
			return nil, err
		}

		for _, docSnap := range snapshots {
			var doc T

			err = docSnap.DataTo(&doc)
			if err != nil {
				return nil, err
			}

			docs = append(docs, Document[T]{docSnap.Ref.ID, doc})
		}
	}

	return docs, nil
}

// fetch documents based on provided Query
func (c *Collection[T]) fetchDocsByQuery(ctx context.Context, query Query) ([]Document[T], error) {
	builtQuery := c.buildQuery(query)
	iter := builtQuery.Documents(ctx)

	var docs []Document[T]

	for {
		docSnap, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var doc T

		err = docSnap.DataTo(&doc)
		if err != nil {
			return nil, err
		}

		docs = append(docs, Document[T]{docSnap.Ref.ID, doc})
	}

	return docs, nil
}
