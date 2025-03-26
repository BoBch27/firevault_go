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

	return func(_ context.Context, fs FieldScope) (bool, error) {
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

	return valFuncInternal(v)
}
