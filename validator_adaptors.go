package firevault

import "context"

// A Validation interface wraps different
// validation function types.
type Validation interface {
	toValFuncInternal() valFuncInternal
}

// A ValidationFunc is a function that's executed
// during a field validation.
type ValidationFunc func(fs FieldScope) (bool, error)

// turns exported func type to internal val func type
func (v ValidationFunc) toValFuncInternal() valFuncInternal {
	if v == nil {
		return nil
	}

	return func(_ context.Context, fs FieldScope) (bool, error) {
		return v(fs)
	}
}

// A ValidationFuncCtx is a context-aware function
// that's executed during a field validation.
// Useful when a validation may depend
// dynamically on a context.
type ValidationFuncCtx func(ctx context.Context, fs FieldScope) (bool, error)

// turns exported func type to internal val func type
func (v ValidationFuncCtx) toValFuncInternal() valFuncInternal {
	if v == nil {
		return nil
	}

	return valFuncInternal(v)
}
