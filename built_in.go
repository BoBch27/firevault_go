package firevault

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"time"
)

var builtInValidators = map[string]ValidationFn{
	"required":          validateRequired,
	"required_create":   validateRequired,
	"required_update":   validateRequired,
	"required_validate": validateRequired,
	"email":             validateEmail,
	"max":               validateMax,
	"min":               validateMin,
}

// validates if field is of supported type
func isSupported(fieldValue reflect.Value) bool {
	switch fieldValue.Kind() {
	case reflect.Invalid, reflect.Chan, reflect.Func:
		return false
	}

	return true
}

// validates if field's value is not the default static value
func hasValue(fieldValue reflect.Value) bool {
	switch fieldValue.Kind() {
	case reflect.Slice, reflect.Map, reflect.Ptr, reflect.Interface:
		return !fieldValue.IsNil()
	default:
		return fieldValue.IsValid() && !fieldValue.IsZero()
	}
}

// validates if field is zero
func validateRequired(_ context.Context, fs FieldScope) (bool, error) {
	return hasValue(fs.Value()), nil
}

// validates if field is a valid email address
func validateEmail(_ context.Context, fs FieldScope) (bool, error) {
	emailRegex := regexp.MustCompile("^(?:(?:(?:(?:[a-zA-Z]|\\d|[!#\\$%&'\\*\\+\\-\\/=\\?\\^_`{\\|}~]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])+(?:\\.([a-zA-Z]|\\d|[!#\\$%&'\\*\\+\\-\\/=\\?\\^_`{\\|}~]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])+)*)|(?:(?:\\x22)(?:(?:(?:(?:\\x20|\\x09)*(?:\\x0d\\x0a))?(?:\\x20|\\x09)+)?(?:(?:[\\x01-\\x08\\x0b\\x0c\\x0e-\\x1f\\x7f]|\\x21|[\\x23-\\x5b]|[\\x5d-\\x7e]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])|(?:(?:[\\x01-\\x09\\x0b\\x0c\\x0d-\\x7f]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}]))))*(?:(?:(?:\\x20|\\x09)*(?:\\x0d\\x0a))?(\\x20|\\x09)+)?(?:\\x22))))@(?:(?:(?:[a-zA-Z]|\\d|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])|(?:(?:[a-zA-Z]|\\d|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])(?:[a-zA-Z]|\\d|-|\\.|~|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])*(?:[a-zA-Z]|\\d|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])))\\.)+(?:(?:[a-zA-Z]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])|(?:(?:[a-zA-Z]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])(?:[a-zA-Z]|\\d|-|\\.|~|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])*(?:[a-zA-Z]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])))\\.?$")
	return emailRegex.MatchString(fs.Value().String()), nil
}

// validates if field's value is less than or equal to param's value
func validateMax(
	_ context.Context,
	fs FieldScope,
) (bool, error) {
	if fs.Param() == "" {
		return false, errors.New("firevault: provide a max param - " + fs.Path())
	}

	switch fs.Kind() {
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		i, err := asInt(fs.Param())
		if err != nil {
			return false, err
		}

		return fs.Value().Len() <= int(i), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := asInt(fs.Param())
		if err != nil {
			return false, err
		}

		return fs.Value().Int() <= i, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := asUint(fs.Param())
		if err != nil {
			return false, err
		}

		return fs.Value().Uint() <= u, nil
	case reflect.Float32, reflect.Float64:
		f, err := asFloat(fs.Param())
		if err != nil {
			return false, err
		}

		return fs.Value().Float() <= f, nil
	case reflect.Struct:
		timeType := reflect.TypeOf(time.Time{})

		if fs.Type().ConvertibleTo(timeType) {
			max, err := asTime(fs.Param())
			if err != nil {
				return false, nil
			}

			t := fs.Value().Convert(timeType).Interface().(time.Time)

			return t.Before(max), nil
		}
	}

	return false, errors.New("firevault: invalid field type - " + fs.Path())
}

// validates if field's value is greater than or equal to param's value
func validateMin(
	_ context.Context,
	fs FieldScope,
) (bool, error) {
	if fs.Param() == "" {
		return false, errors.New("firevault: provide a min param - " + fs.Path())
	}

	switch fs.Kind() {
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		i, err := asInt(fs.Param())
		if err != nil {
			return false, err
		}

		return fs.Value().Len() >= int(i), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := asInt(fs.Param())
		if err != nil {
			return false, err
		}

		return fs.Value().Int() >= i, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := asUint(fs.Param())
		if err != nil {
			return false, err
		}

		return fs.Value().Uint() >= u, nil
	case reflect.Float32, reflect.Float64:
		f, err := asFloat(fs.Param())
		if err != nil {
			return false, err
		}

		return fs.Value().Float() >= f, nil
	case reflect.Struct:
		timeType := reflect.TypeOf(time.Time{})

		if fs.Type().ConvertibleTo(timeType) {
			min, err := asTime(fs.Param())
			if err != nil {
				return false, err
			}

			t := fs.Value().Convert(timeType).Interface().(time.Time)

			return t.After(min), nil
		}
	}

	return false, errors.New("firevault: invalid field type - " + fs.Path())
}
