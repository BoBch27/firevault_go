package firevault

import "context"

// A Validation interface wraps different
// validation function types.
type Validation interface {
	toValFuncInternal() valFuncInternal
}

// A ValidationFuncCtx is a context-aware function
// that's executed during a field validation.
// Useful when a validation may depend
// dynamically on a context.
type ValidationFuncCtx func(ctx context.Context, fs FieldScope) (bool, error)

// implement method to satisfy interface
func (v ValidationFuncCtx) toValFuncInternal() valFuncInternal {
	if v == nil {
		return nil
	}

	return valFuncInternal(v)
}
