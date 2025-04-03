package firevault

import "context"

// Validation interface wraps different
// validation function types.
type Validation interface {
	toValFuncInternal() valFuncInternal
}

// ValidationFunc is a function that's executed
// during a field validation.
type ValidationFunc func(fs FieldScope) (bool, error)

// turns exported func type to internal val func type
func (v ValidationFunc) toValFuncInternal() valFuncInternal {
	if v == nil {
		return nil
	}

	return func(_ context.Context, _ *Transaction, fs FieldScope) (bool, error) {
		return v(fs)
	}
}

// ValidationFuncCtx is a context-aware function
// that's executed during a field validation.
//
// Useful when a validation requires access
// to a context.
type ValidationFuncCtx func(ctx context.Context, fs FieldScope) (bool, error)

// turns exported func type to internal val func type
func (v ValidationFuncCtx) toValFuncInternal() valFuncInternal {
	if v == nil {
		return nil
	}

	return func(ctx context.Context, _ *Transaction, fs FieldScope) (bool, error) {
		return v(ctx, fs)
	}
}

// ValidationFuncTx is a transaction-aware
// function that's executed during a field
// validation.
//
// Useful when a validation needs to be
// executed inside a transaction.
//
// The transaction argument is nil, unless
// explicitly provided in the Options of the
// calling CollectionRef method.
type ValidationFuncTx func(tx *Transaction, fs FieldScope) (bool, error)

// turns exported func type to internal val func type
func (v ValidationFuncTx) toValFuncInternal() valFuncInternal {
	if v == nil {
		return nil
	}

	return func(_ context.Context, tx *Transaction, fs FieldScope) (bool, error) {
		return v(tx, fs)
	}
}

// ValidationFuncCtxTx is a context-aware and
// transaction-aware function that's executed
// during a field validation.
//
// Useful when a validation requires access to
// a context and needs to be executed inside a
// transaction.
//
// The transaction argument is nil, unless
// explicitly provided in the Options of the
// calling CollectionRef method.
type ValidationFuncCtxTx func(ctx context.Context, tx *Transaction, fs FieldScope) (bool, error)

// turns exported func type to internal val func type
func (v ValidationFuncCtxTx) toValFuncInternal() valFuncInternal {
	if v == nil {
		return nil
	}

	return valFuncInternal(v)
}

// Transformation interface wraps
// different transformation function types.
type Transformation interface {
	toTranFuncInternal() tranFuncInternal
}

// TransformationFunc is a function that's executed
// during a field transformation.
type TransformationFunc func(fs FieldScope) (interface{}, error)

// turns exported func type to internal val func type
func (t TransformationFunc) toTranFuncInternal() tranFuncInternal {
	if t == nil {
		return nil
	}

	return func(_ context.Context, _ *Transaction, fs FieldScope) (interface{}, error) {
		return t(fs)
	}
}

// TransformationFuncCtx is a context-aware
// function that's executed during a field
// transformation.
//
// Useful when a transformation requires access
// to a context.
type TransformationFuncCtx func(ctx context.Context, fs FieldScope) (interface{}, error)

// turns exported func type to internal val func type
func (t TransformationFuncCtx) toTranFuncInternal() tranFuncInternal {
	if t == nil {
		return nil
	}

	return func(ctx context.Context, _ *Transaction, fs FieldScope) (interface{}, error) {
		return t(ctx, fs)
	}
}

// TransformationFuncTx is a transaction-aware
// function that's executed during a field
// transformation.
//
// Useful when a transformation needs to be
// executed inside a transaction.
//
// The transaction argument is nil, unless
// explicitly provided in the Options of the
// calling CollectionRef method.
type TransformationFuncTx func(tx *Transaction, fs FieldScope) (interface{}, error)

// turns exported func type to internal val func type
func (t TransformationFuncTx) toTranFuncInternal() tranFuncInternal {
	if t == nil {
		return nil
	}

	return func(_ context.Context, tx *Transaction, fs FieldScope) (interface{}, error) {
		return t(tx, fs)
	}
}

// TransformationFuncCtxTx is a context-aware
// and transaction-aware function that's
// executed during a field transformation.
//
// Useful when a transformation requires
// access to a context and needs to be executed
// inside a transaction.
//
// The transaction argument is nil, unless
// explicitly provided in the Options of the
// calling CollectionRef method.
type TransformationFuncCtxTx func(ctx context.Context, tx *Transaction, fs FieldScope) (interface{}, error)

// turns exported func type to internal val func type
func (t TransformationFuncCtxTx) toTranFuncInternal() tranFuncInternal {
	if t == nil {
		return nil
	}

	return tranFuncInternal(t)
}
