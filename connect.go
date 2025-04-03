package firevault

import (
	"context"
	"errors"

	"cloud.google.com/go/firestore"
)

// Connection provides access to Firevault
// services.
//
// It is designed to be thread-safe and used
// as a singleton instance.
//
// A cache is used under the hood to store
// struct validation metadata, parsing
// validation tags once per struct type.
// Using multiple instances defeats the
// purpose of caching.
type Connection struct {
	client    *firestore.Client
	validator *validator
}

// Create a new Connection instance.
//
// Connection provides access to Firevault
// services.
//
// It is designed to be thread-safe and used
// as a singleton instance.
//
// A cache is used under the hood to store
// struct validation metadata, parsing
// validation tags once per struct type.
// Using multiple instances defeats the
// purpose of caching.
func Connect(ctx context.Context, projectID string) (*Connection, error) {
	val := newValidator()

	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return &Connection{client, val}, nil
}

// Close closes the connection to Firevault.
//
// Should be invoked when the connection is
// no longer required.
//
// Close need not be called at program exit.
func (c *Connection) Close() error {
	if c == nil || c.client == nil {
		return errors.New("firevault: nil Connection or Firestore Client")
	}

	return c.client.Close()
}

// Register a new validation rule.
//
// If a validation rule with the same name
// already exists, the previous one will be replaced.
//
// If the validation name includes a method-specific
// suffix ("_create", "_update", or "_validate"),
// the rule will be applied exclusively during
// calls to the corresponding method type and
// ignored for others.
//
// Registering such functions is not thread-safe;
// it is intended that all rules be registered,
// prior to any validation.
func (c *Connection) RegisterValidation(
	name string,
	validation Validation,
	runOnNil ...bool,
) error {
	if c == nil {
		return errors.New("firevault: nil Connection")
	}

	var nilCallable bool
	if len(runOnNil) > 0 {
		nilCallable = runOnNil[0]
	}

	return c.validator.registerValidation(
		name,
		validation.toValFuncInternal(),
		false,
		nilCallable,
	)
}

// Register a new transformation rule.
//
// If a transformation rule with the same name
// already exists, the previous one will be replaced.
//
// If the transformation name includes a
// method-specific suffix
// ("_create", "_update", or "_validate"),
// the rule will be applied exclusively during
// calls to the corresponding method type and
// ignored for others.
//
// Registering such functions is not thread-safe;
// it is intended that all rules be registered,
// prior to any validation.
func (c *Connection) RegisterTransformation(
	name string,
	transformation Transformation,
	runOnNil ...bool,
) error {
	if c == nil {
		return errors.New("firevault: nil Connection")
	}

	var nilCallable bool
	if len(runOnNil) > 0 {
		nilCallable = runOnNil[0]
	}

	return c.validator.registerTransformation(
		name,
		transformation.toTranFuncInternal(),
		false,
		nilCallable,
	)
}

// Register a new error formatter.
//
// Error formatters are used to generate a custom,
// user-friendly error message, whenever a
// FieldError is created (during a failed validation).
//
// If none are registered, or if a formatter returns
// a nil error, an instance of a FieldError will be
// returned instead.
//
// Registering error formatters is not thread-safe;
// it is intended that all such functions
// be registered, prior to any validation.
func (c *Connection) RegisterErrorFormatter(errorFormatter ErrorFormatterFunc) error {
	if c == nil {
		return errors.New("firevault: nil Connection")
	}

	return c.validator.registerErrorFormatter(errorFormatter)
}

// Run a Firestore transaction, ensuring all
// operations within the provided function are
// executed atomically.
//
// If any operation fails, the transaction is
// retried up to Firestore’s default limit. If all
// retries fail, the transaction is rolled back
// and the error is returned.
//
// The provided function receives a Transaction
// instance, which should be used for all reads and
// writes within the transaction.
//
// This instance can be passed into CollectionRef
// methods, via Options, to ensure they execute as
// part of the transaction.
//
// Returns an error if the transaction fails after
// all retries.
func (c *Connection) RunTransaction(
	ctx context.Context,
	fn func(context.Context, *Transaction) error,
) error {
	return c.client.RunTransaction(ctx, fn)
}
