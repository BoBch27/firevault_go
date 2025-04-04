package firevault

import (
	"errors"
	"reflect"
	"strings"
	"time"
)

const restrictedTagChars = ".[],|=+()`~!@#$%^&*\\\"/?<>{}"

var (
	restrictedRules = map[string]struct{}{
		"dive":               {},
		"omitempty":          {},
		"omitempty_create":   {},
		"omitempty_update":   {},
		"omitempty_validate": {},
	}

	builtInValidators = map[string]ValidationFunc{
		"required":          validateRequired,
		"required_create":   validateRequired,
		"required_update":   validateRequired,
		"required_validate": validateRequired,
		"email":             validateEmail,
		"max":               validateMax,
		"min":               validateMin,
	}

	builtInTransformators = map[string]TransformationFunc{
		"uppercase":  transformUppercase,
		"lowercase":  transformLowercase,
		"trim_space": transformTrimSpace,
	}
)

// validates if field is of supported type
func isSupported(fieldKind reflect.Kind) bool {
	switch fieldKind {
	case reflect.Invalid, reflect.Chan, reflect.Func:
		return false
	}

	return true
}

// validates if field's value is not the default static value
func hasValue(fieldKind reflect.Kind, fieldValue reflect.Value) bool {
	switch fieldKind {
	case reflect.Slice, reflect.Map, reflect.Ptr, reflect.Interface:
		return !fieldValue.IsNil()
	default:
		return fieldValue.IsValid() && !fieldValue.IsZero()
	}
}

// validates if field is zero
func validateRequired(fs FieldScope) (bool, error) {
	return hasValue(fs.Kind(), fs.Value()), nil
}

// validates if field is a valid email address
func validateEmail(fs FieldScope) (bool, error) {
	return emailRegex().MatchString(fs.Value().String()), nil
}

// validates if field's value is less than or equal to param's value
func validateMax(fs FieldScope) (bool, error) {
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
func validateMin(fs FieldScope) (bool, error) {
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

// transforms a field of string type to upper case
func transformUppercase(fs FieldScope) (interface{}, error) {
	if fs.Kind() != reflect.String {
		return fs.Value().Interface(), nil
	}

	return strings.ToUpper(fs.Value().String()), nil
}

// transforms a field of string type to lower case
func transformLowercase(fs FieldScope) (interface{}, error) {
	if fs.Kind() != reflect.String {
		return fs.Value().Interface(), nil
	}

	return strings.ToLower(fs.Value().String()), nil
}

// transforms a field of string type by removing all white space
func transformTrimSpace(fs FieldScope) (interface{}, error) {
	if fs.Kind() != reflect.String {
		return fs.Value().Interface(), nil
	}

	return strings.TrimSpace(fs.Value().String()), nil
}
