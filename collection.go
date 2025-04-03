package firevault

import (
	"context"
	"errors"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"google.golang.org/api/iterator"
)

// CollectionRef holds a reference to a Firestore
// Collection and allows for the fetching and
// modifying (with validation) of documents in it.
//
// CollectionRef instances are lightweight and safe to
// create repeatedly. They can be freely used as needed,
// without concern for maintaining a singleton instance,
// as each instance independently references the
// specified Firestore collection.
type CollectionRef[T interface{}] struct {
	connection *Connection
	ref        *firestore.CollectionRef
}

// Document holds the ID and data related to a fetched
// document.
type Document[T interface{}] struct {
	ID       string
	Data     T
	Metadata metadata
}

// read-only document metadata
type metadata struct {
	// Read-only. The time at which the document was created.
	// Increases monotonically when a document is deleted then
	// recreated.
	CreateTime time.Time
	// Read-only. The time at which the document was last
	// changed. This value is initially set to CreateTime then
	// increases monotonically with each change to the document.
	UpdateTime time.Time
	// Read-only. The time at which the document was read.
	ReadTime time.Time
}

// Create a new CollectionRef instance.
//
// CollectionRef holds a reference to a Firestore
// Collection and allows for the fetching and
// modifying (with validation) of documents in it.
//
// CollectionRef instances are lightweight and safe to
// create repeatedly. They can be freely used as needed,
// without concern for maintaining a singleton instance,
// as each instance independently references the
// specified Firestore collection.
//
// The path argument is a sequence of IDs,
// separated by slashes.
//
// Returns nil if path contains an even number of IDs,
// or any ID is empty.
func Collection[T interface{}](connection *Connection, path string) *CollectionRef[T] {
	if connection == nil || connection.client == nil {
		return nil
	}

	collectionRef := connection.client.Collection(path)
	if collectionRef == nil {
		return nil
	}

	return &CollectionRef[T]{connection, collectionRef}
}

// Validate and transform provided data.
func (c *CollectionRef[T]) Validate(ctx context.Context, data *T, opts ...Options) error {
	if c == nil {
		return errors.New("firevault: nil CollectionRef")
	}

	valOpts, _, _, _, _ := c.parseOptions(validate, opts...)

	_, err := c.connection.validator.validate(ctx, data, valOpts)
	return err
}

// Create a Firestore document with provided data
// (after validation).
//
// By default, Firestore generates a unique document
// ID. Use Options to change this behaviour.
//
// An error is returned if a document with the same
// ID already exists, whether auto-generated
// (unlikely), or provided.
//
// To use inside a transaction, pass a transaction
// instance via Options.
func (c *CollectionRef[T]) Create(ctx context.Context, data *T, opts ...Options) (string, error) {
	if c == nil {
		return "", errors.New("firevault: nil CollectionRef")
	}

	valOpts, id, _, _, _ := c.parseOptions(create, opts...)

	dataMap, err := c.connection.validator.validate(ctx, data, valOpts)
	if err != nil {
		return "", err
	}

	if id == "" {
		docRef := c.ref.NewDoc() // generates doc ref with random id (used in c.ref.Add)
		id = docRef.ID
	}

	// perform transaction if provided opt
	if valOpts.tx != nil {
		query := Query{ids: []string{id}} // needed to run transac operation
		err = c.transacOperation(valOpts.tx, query, func(tx *firestore.Transaction, docID string) error {
			return tx.Create(c.ref.Doc(docID), dataMap)
		})
		if err != nil {
			return "", err
		}

		return id, nil
	}

	_, err = c.ref.Doc(id).Create(ctx, dataMap)
	if err != nil {
		return "", err
	}

	return id, nil
}

// Update all Firestore documents which match
// provided Query (after data validation).
//
// By default, data fields are merged, preserving
// existing document fields. Use Options to change
// this behaviour.
//
// If the Query doesn't contain an ID clause, documents
// are read first to retrieve their IDs. If no
// documents match, the operation does nothing and
// returns no error.
//
// If the Query does contain an ID clause, the operation
// fails for any ID not corresponding to an existing
// document, returning an error. Updates to matching
// documents are applied.
//
// In the event a precondition is set, the operation
// fails for any matching document that does not meet
// the precondition, returning an error. Other
// updates are still processed.
//
// The operation is not atomic, unless used inside a
// transaction via Options.
//
// Note: In a transaction with no ID clause, document
// IDs are read first, which prevents any prior writes
// or subsequent reads in the same transaction. To work
// around this, use the Find/FindOne method before
// calling Update.
func (c *CollectionRef[T]) Update(ctx context.Context, query Query, data *T, opts ...Options) error {
	if c == nil {
		return errors.New("firevault: nil CollectionRef")
	}

	valOpts, _, precond, merge, mergeFields := c.parseOptions(update, opts...)

	dataMap, err := c.connection.validator.validate(ctx, data, valOpts)
	if err != nil {
		return err
	}

	updates := c.parseUpdates(dataMap, merge, mergeFields)

	// perform transaction if provided opt
	if valOpts.tx != nil {
		return c.transacOperation(valOpts.tx, query, func(tx *firestore.Transaction, docID string) error {
			if precond != nil {
				return tx.Update(c.ref.Doc(docID), updates, precond)
			}

			return tx.Update(c.ref.Doc(docID), updates)
		})
	}

	return c.bulkOperation(ctx, query, func(bw *firestore.BulkWriter, docID string) error {
		if precond != nil {
			_, err := bw.Update(c.ref.Doc(docID), updates, precond)
			return err
		}

		_, err := bw.Update(c.ref.Doc(docID), updates)
		return err
	})
}

// Delete all Firestore documents which match
// provided Query.
//
// If the Query doesn't contain an ID clause, documents
// are read first to retrieve their IDs. If no
// documents match, the operation does nothing and
// returns no error.
//
// If the Query does contain an ID clause, the operation
// skips any ID that does not correspond to an
// existing document, without returning an error.
// Deletes to matching documents are applied.
//
// In the event a precondition is set, the operation
// fails for any matching document that does not meet
// the precondition, returning an error. Other
// deletes are still processed.
//
// The operation is not atomic, unless used inside a
// transaction via Options.
//
// Note: In a transaction with no ID clause, document
// IDs are read first, which prevents any prior writes
// or subsequent reads in the same transaction. To work
// around this, use the Find/FindOne method before
// calling Delete.
func (c *CollectionRef[T]) Delete(ctx context.Context, query Query, opts ...Options) error {
	if c == nil {
		return errors.New("firevault: nil CollectionRef")
	}

	valOpts, _, precond, _, _ := c.parseOptions(delete, opts...)

	// perform transaction if provided opt
	if valOpts.tx != nil {
		return c.transacOperation(valOpts.tx, query, func(tx *firestore.Transaction, docID string) error {
			if precond != nil {
				return tx.Delete(c.ref.Doc(docID), precond)
			}

			return tx.Delete(c.ref.Doc(docID))
		})
	}

	return c.bulkOperation(ctx, query, func(bw *firestore.BulkWriter, docID string) error {
		if precond != nil {
			_, err := bw.Delete(c.ref.Doc(docID), precond)
			return err
		}

		_, err := bw.Delete(c.ref.Doc(docID))
		return err
	})
}

// Find all Firestore documents which match
// provided Query.
//
// To use inside a transaction, pass a transaction
// instance via Options.
func (c *CollectionRef[T]) Find(ctx context.Context, query Query, opts ...Options) ([]Document[T], error) {
	if c == nil {
		return nil, errors.New("firevault: nil CollectionRef")
	}

	valOpts, _, _, _, _ := c.parseOptions(find, opts...)

	if len(query.ids) > 0 {
		return c.fetchDocsByID(ctx, valOpts.tx, query.ids)
	}

	return c.fetchDocsByQuery(ctx, valOpts.tx, query)
}

// Find the first Firestore document which
// matches provided Query.
//
// Returns an empty Document[T] (empty ID
// string and zero-value T Data), and no error
// if no documents are found.
//
// To use inside a transaction, pass a transaction
// instance via Options.
func (c *CollectionRef[T]) FindOne(ctx context.Context, query Query, opts ...Options) (Document[T], error) {
	if c == nil {
		return Document[T]{}, errors.New("firevault: nil CollectionRef")
	}

	valOpts, _, _, _, _ := c.parseOptions(find, opts...)

	if len(query.ids) > 0 {
		docs, err := c.fetchDocsByID(ctx, valOpts.tx, query.ids[0:1])
		if err != nil {
			return Document[T]{}, err
		}

		if len(docs) == 0 {
			return Document[T]{}, nil
		}

		return docs[0], nil
	}

	docs, err := c.fetchDocsByQuery(ctx, valOpts.tx, query.Limit(1))
	if err != nil {
		return Document[T]{}, err
	}

	if len(docs) == 0 {
		return Document[T]{}, nil
	}

	return docs[0], nil
}

// Find number of Firestore documents which
// match provided Query.
func (c *CollectionRef[T]) Count(ctx context.Context, query Query) (int64, error) {
	if c == nil {
		return 0, errors.New("firevault: nil CollectionRef")
	}

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

// used to determine how to parse options
type methodType string

const (
	validate methodType = "validate"
	create   methodType = "create"
	update   methodType = "update"
	find     methodType = "find"
	delete   methodType = "delete"
	all      methodType = "all"
	none     methodType = "none"
)

// extract passed options
func (c *CollectionRef[T]) parseOptions(
	method methodType,
	opts ...Options,
) (validationOpts, string, firestore.Precondition, bool, []string) {
	if len(opts) == 0 {
		return validationOpts{method: method}, "", nil, true, nil
	}

	// parse options
	passedOpts := opts[0]
	options := validationOpts{
		method:             method,
		skipValidation:     passedOpts.skipValidation,
		skipValFields:      passedOpts.skipValFields,
		emptyFieldsAllowed: passedOpts.allowEmptyFields,
		modifyOriginal:     passedOpts.modifyOriginal,
		tx:                 passedOpts.transaction,
	}

	if method == validate && passedOpts.method != "" {
		options.method = passedOpts.method
	}

	if method == update && passedOpts.disableMerge {
		options.deleteEmpty = true
		return options, passedOpts.id, passedOpts.precondition, false, nil
	}

	if method == update && len(passedOpts.mergeFields) > 0 {
		return options, passedOpts.id, passedOpts.precondition, true, passedOpts.mergeFields
	}

	return options, passedOpts.id, passedOpts.precondition, true, nil
}

// build a new firestore query
func (c *CollectionRef[T]) buildQuery(query Query) firestore.Query {
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

// prepares firestore updates based on merge logic
func (c *CollectionRef[T]) parseUpdates(
	dataMap map[string]interface{},
	merge bool,
	mergeFields []string,
) []firestore.Update {
	updates := []firestore.Update{}

	// convert mergeFields to a map for O(1) lookup
	mergeFieldsMap := make(map[string]bool, len(mergeFields))
	if merge && len(mergeFields) > 0 {
		for _, field := range mergeFields {
			mergeFieldsMap[field] = true
		}
	}

	// recursive closure to process map
	var processMap func(data map[string]interface{}, prefix string)
	processMap = func(data map[string]interface{}, prefix string) {
		for k, v := range data {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}

			// if path is in mergeFields, add to updates without recursive processing
			if merge && mergeFieldsMap[path] {
				updates = append(updates, firestore.Update{Path: path, Value: v})
				continue
			}

			// if it's a non-empty nested map, process it recursively
			subMap, ok := v.(map[string]interface{})
			if ok && len(subMap) > 0 {
				processMap(subMap, path)
				continue
			}

			// determine whether to include field based on merge settings
			if !merge || (merge && len(mergeFields) == 0) {
				updates = append(updates, firestore.Update{Path: path, Value: v})
			}
		}
	}

	// start processing from the root
	processMap(dataMap, "")
	return updates
}

// perform an operation within a started transaction
func (c *CollectionRef[T]) transacOperation(
	tx *firestore.Transaction,
	query Query,
	operation func(*firestore.Transaction, string) error,
) error {
	if tx == nil {
		return errors.New("firevault: no transaction provided")
	}

	docIDs := query.ids

	if len(docIDs) == 0 {
		docs, err := c.fetchDocsByQuery(context.Background(), tx, query) // bg ctx since we're using tx
		if err != nil {
			return err
		}

		for _, doc := range docs {
			docIDs = append(docIDs, doc.ID)
		}
	}

	if len(docIDs) == 0 {
		return nil // no matching documents
	}

	var mu sync.Mutex
	var errs []error

	for _, docID := range docIDs {
		err := operation(tx, docID)
		if err != nil {
			mu.Lock()
			errs = append(errs, errors.New(err.Error()+" (docID: "+docID+")"))
			mu.Unlock()
		}
	}

	return errors.Join(errs...)
}

// perform a bulk operation
func (c *CollectionRef[T]) bulkOperation(
	ctx context.Context,
	query Query,
	operation func(*firestore.BulkWriter, string) error,
) error {
	bulkWriter := c.connection.client.BulkWriter(ctx)
	defer bulkWriter.End()

	docIDs := query.ids

	if len(docIDs) == 0 {
		docs, err := c.fetchDocsByQuery(ctx, nil, query)
		if err != nil {
			return err
		}

		for _, doc := range docs {
			docIDs = append(docIDs, doc.ID)
		}
	}

	if len(docIDs) == 0 {
		return nil // no matching documents
	}

	var mu sync.Mutex
	var errs []error

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
func (c *CollectionRef[T]) fetchDocsByID(
	ctx context.Context,
	tx *Transaction,
	ids []string,
) ([]Document[T], error) {
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

		var snapshots []*firestore.DocumentSnapshot
		var err error

		if tx != nil {
			snapshots, err = tx.GetAll(batchRefs) // use transaction
		} else {
			snapshots, err = c.connection.client.GetAll(ctx, batchRefs)
		}
		if err != nil {
			return nil, err
		}

		for _, docSnap := range snapshots {
			if !docSnap.Exists() {
				continue
			}

			var doc T

			err = docSnap.DataTo(&doc)
			if err != nil {
				return nil, err
			}

			docs = append(
				docs,
				Document[T]{
					docSnap.Ref.ID,
					doc,
					metadata{
						docSnap.CreateTime,
						docSnap.UpdateTime,
						docSnap.ReadTime,
					},
				},
			)
		}
	}

	return docs, nil
}

// fetch documents based on provided Query
func (c *CollectionRef[T]) fetchDocsByQuery(
	ctx context.Context,
	tx *Transaction,
	query Query,
) ([]Document[T], error) {
	builtQuery := c.buildQuery(query)

	var iter *firestore.DocumentIterator
	if tx != nil {
		iter = tx.Documents(builtQuery) // use transaction
	} else {
		iter = builtQuery.Documents(ctx)
	}
	defer iter.Stop()

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

		docs = append(
			docs,
			Document[T]{
				docSnap.Ref.ID,
				doc,
				metadata{
					docSnap.CreateTime,
					docSnap.UpdateTime,
					docSnap.ReadTime,
				},
			},
		)
	}

	return docs, nil
}
