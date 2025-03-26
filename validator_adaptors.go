package firevault

import "context"

// A ValidationFunc interface wraps different
// validation function types.
type ValidationFunc interface {
	toValFuncInternal() valFuncInternal
}

// A ValFunc is a function that's executed
// during a field validation.
type ValFunc func(fs FieldScope) (bool, error)

// turns exported func type to internal val func type
func (v ValFunc) toValFuncInternal() valFuncInternal {
	if v == nil {
		return nil
	}

	return func(_ context.Context, _ *Transaction, fs FieldScope) (bool, error) {
		return v(fs)
	}
}

// A ValFuncCtx is a context-aware function
// that's executed during a field validation.
// Useful when a validation may depend
// dynamically on a context.
type ValFuncCtx func(ctx context.Context, fs FieldScope) (bool, error)

// turns exported func type to internal val func type
func (v ValFuncCtx) toValFuncInternal() valFuncInternal {
	if v == nil {
		return nil
	}

	return func(ctx context.Context, _ *Transaction, fs FieldScope) (bool, error) {
		return v(ctx, fs)
	}
}

// A ValFuncTx is a transaction-aware function
// that's executed during a field validation.
// Useful when a validation may need to be
// executed inside a transaction.
type ValFuncTx func(tx *Transaction, fs FieldScope) (bool, error)

// turns exported func type to internal val func type
func (v ValFuncTx) toValFuncInternal() valFuncInternal {
	if v == nil {
		return nil
	}

	return func(_ context.Context, tx *Transaction, fs FieldScope) (bool, error) {
		return v(tx, fs)
	}
}

// A TransformationFunc interface wraps
// different transformation function types.
type TransformationFunc interface {
	toTranFuncInternal() tranFuncInternal
}

// A TranFunc is a function that's executed
// during a field transformation.
type TranFunc func(fs FieldScope) (interface{}, error)

// turns exported func type to internal val func type
func (t TranFunc) toTranFuncInternal() tranFuncInternal {
	if t == nil {
		return nil
	}

	return func(_ context.Context, fs FieldScope) (interface{}, error) {
		return t(fs)
	}
}

// A TranFuncCtx is a context-aware function
// that's executed during a field transformation.
// Useful when a transformation may depend
// dynamically on a context.
type TranFuncCtx func(ctx context.Context, fs FieldScope) (interface{}, error)

// turns exported func type to internal val func type
func (t TranFuncCtx) toTranFuncInternal() tranFuncInternal {
	if t == nil {
		return nil
	}

	return tranFuncInternal(t)
}
