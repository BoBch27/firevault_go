package firevault

import "context"

// A ValidationFunc interface wraps different
// validation function types.
type ValidationFunc interface {
	toValFuncInternal() valFuncInternal
}

// A ValFuncCtx is a context-aware function
// that's executed during a field validation.
// Useful when a validation may depend
// dynamically on a context.
type ValFuncCtx func(ctx context.Context, fs FieldScope) (bool, error)

// implement method to satisfy interface
func (v ValFuncCtx) toValFuncInternal() valFuncInternal {
	if v == nil {
		return nil
	}

	return valFuncInternal(v)
}
