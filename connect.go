package firevault

import (
	"context"
	"errors"

	"cloud.google.com/go/firestore"
)

// A Firevault Connection provides access to
// Firevault services.
type Connection struct {
	client    *firestore.Client
	validator *validator
}

// Create a new Connection instance.
//
// A Firevault Connection provides access to
// Firevault services.
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
// Registering such functions is not thread-safe;
// it is intended that all rules be registered,
// prior to any validation.
func (c *Connection) RegisterValidation(name string, validation ValidationFn) error {
	if c == nil {
		return errors.New("firevault: nil Connection")
	}

	return c.validator.registerValidation(name, validation)
}

// Register a new transformation rule.
//
// If a transformation rule with the same name
// already exists, the previous one will be replaced.
//
// Registering such functions is not thread-safe;
// it is intended that all rules be registered,
// prior to any validation.
func (c *Connection) RegisterTransformation(name string, transformation TransformationFn) error {
	if c == nil {
		return errors.New("firevault: nil Connection")
	}

	return c.validator.registerTransformation(name, transformation)
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
func (c *Connection) RegisterErrorFormatter(errorFormatter ErrorFormatterFn) error {
	if c == nil {
		return errors.New("firevault: nil Connection")
	}

	return c.validator.registerErrorFormatter(errorFormatter)
}
