package firevault

import (
	"fmt"
	"reflect"
)

// validates if field is zero and returns error if so
func validateRequired(fieldName string, fieldValue reflect.Value) error {
	if fieldValue.IsZero() {
		return fmt.Errorf("firevault: field %s is required", fieldName)
	}

	return nil
}
