package firevault

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
)

// A ValidationFn is the function that's executed
// during a validation.
type ValidationFn func(ctx context.Context, fs FieldScope) (bool, error)

// A TransformationFn is the function that's executed
// during a transformation.
type TransformationFn func(ctx context.Context, fs FieldScope) (interface{}, error)

// An ErrorFormatterFn is the function that's executed
// to generate a custom, user-friendly error message,
// based on FieldError's fields.
//
// If the function returns a nil error, an instance
// of FieldError will be returned instead.
type ErrorFormatterFn func(fe FieldError) error

type validator struct {
	validations     map[string]ValidationFn
	transformations map[string]TransformationFn
	errFormatters   []ErrorFormatterFn
}

func newValidator() *validator {
	validator := &validator{
		make(map[string]ValidationFn),
		make(map[string]TransformationFn),
		make([]ErrorFormatterFn, 0),
	}

	// Register predefined validators
	for k, v := range builtInValidators {
		// no need to error check here, built in validations are always valid
		_ = validator.registerValidation(k, v)
	}

	return validator
}

// register a validation
func (v *validator) registerValidation(name string, validation ValidationFn) error {
	if v == nil {
		return errors.New("firevault: nil validator")
	}

	if len(name) == 0 {
		return errors.New("firevault: validation function name cannot be empty")
	}

	if validation == nil {
		return fmt.Errorf("firevault: validation function %s cannot be empty", name)
	}

	v.validations[name] = validation
	return nil
}

// register a transformation
func (v *validator) registerTransformation(name string, transformation TransformationFn) error {
	if v == nil {
		return errors.New("firevault: nil validator")
	}

	if len(name) == 0 {
		return errors.New("firevault: transformation function name cannot be empty")
	}

	if transformation == nil {
		return fmt.Errorf("firevault: transformation function %s cannot be empty", name)
	}

	v.transformations[name] = transformation
	return nil
}

// register an error formatter
func (v *validator) registerErrorFormatter(errFormatter ErrorFormatterFn) error {
	if v == nil {
		return errors.New("firevault: nil validator")
	}

	if errFormatter == nil {
		return fmt.Errorf("firevault: error formatter function cannot be empty")
	}

	v.errFormatters = append(v.errFormatters, errFormatter)
	return nil
}

// the reflected struct
type reflectedStruct struct {
	types  reflect.Type
	values reflect.Value
}

// check if passed data is a pointer and reflect it if so
func (v *validator) validate(
	ctx context.Context,
	data interface{},
	opts validationOpts,
) (map[string]interface{}, error) {
	if v == nil {
		return nil, errors.New("firevault: nil validator")
	}

	rs := reflectedStruct{reflect.TypeOf(data), reflect.ValueOf(data)}
	rsValKind := rs.values.Kind()

	if rsValKind != reflect.Pointer && rsValKind != reflect.Ptr {
		return nil, errors.New("firevault: data must be a pointer to a struct")
	}

	rs.values = rs.values.Elem()
	rs.types = rs.types.Elem()

	if rs.values.Kind() != reflect.Struct {
		return nil, errors.New("firevault: data must be a pointer to a struct")
	}

	dataMap, err := v.validateFields(ctx, rs, "", "", opts)
	return dataMap, err
}

// loop through struct's fields and validate
// based on provided tags and options
func (v *validator) validateFields(
	ctx context.Context,
	rs reflectedStruct,
	path string,
	structPath string,
	opts validationOpts,
) (map[string]interface{}, error) {
	// map which will hold all fields to pass to firestore
	dataMap := make(map[string]interface{})

	// iterate over struct fields
	for i := 0; i < rs.values.NumField(); i++ {
		fieldValue := rs.values.Field(i)
		fieldType := rs.types.Field(i)

		fs := &fieldScope{
			strct:        rs.values,
			field:        fieldType.Name,
			structField:  fieldType.Name,
			displayField: v.getDisplayName(fieldType.Name),
			value:        fieldValue,
			kind:         fieldValue.Kind(),
			typ:          fieldType.Type,
		}

		tag := fieldType.Tag.Get("firevault")

		if tag == "" || tag == "-" {
			continue
		}

		rules := v.parseTag(tag)

		// use first tag rule as new field name, if not empty
		if rules[0] != "" {
			fs.field = rules[0]
		}

		// get dot-separated field and struct path
		fs.path = v.getFieldPath(path, fs.field)
		fs.structPath = v.getFieldPath(structPath, fs.structField)

		// check if field is of supported type
		err := v.validateFieldType(fs.value, fs.path)
		if err != nil {
			return nil, err
		}

		// check if field should be skipped based on provided tags
		if v.shouldSkipField(fs.value, fs.path, rules, opts) {
			continue
		}

		// remove omitempty tags from rules, so no validation is attempted
		rules = v.cleanRules(rules)

		// get pointer value, only if it's not nil
		if fs.kind == reflect.Pointer || fs.kind == reflect.Ptr {
			if !fs.value.IsNil() {
				fs.value = fs.value.Elem()
				fs.kind = fs.value.Kind()
				fs.typ = fs.value.Type()
			}
		}

		// apply rules (both transformations and validations)
		// unless skipped using options
		if !opts.skipValidation {
			newFieldValue, err := v.applyRules(ctx, fs, rules, opts.method)
			if err != nil {
				return nil, err
			}

			// set original struct's field value if changed
			if newFieldValue != fs.value {
				rs.values.Field(i).Set(newFieldValue)
				fs.value = newFieldValue
				fs.kind = fs.value.Kind()
				fs.typ = fs.value.Type()
			}
		}

		// get the final value to be added to the data map
		finalValue, err := v.processFinalValue(ctx, fs, opts)
		if err != nil {
			return nil, err
		}

		dataMap[fs.field] = finalValue
	}

	return dataMap, nil
}

// get dot-separated field path
func (v *validator) getFieldPath(path string, fieldName string) string {
	if path == "" {
		return fieldName
	}

	return path + "." + fieldName
}

// get field struct name in a human-readable form
func (v *validator) getDisplayName(fieldName string) string {
	// handle snake case - replace underscores with spaces
	fn := strings.ReplaceAll(fieldName, "_", " ")

	// split camel and pascal case
	fn = regexp.MustCompile(`([a-z])([A-Z])`).ReplaceAllStringFunc(fn, func(ns string) string {
		return string(ns[0]) + " " + string(ns[1])
	})

	// check if string contains a number
	if regexp.MustCompile(`\d`).MatchString(fn) {
		fn = regexp.MustCompile(`([A-Z])([0-9])`).ReplaceAllStringFunc(fn, func(ns string) string {
			return string(ns[0]) + " " + string(ns[1])
		})
		fn = regexp.MustCompile(`([a-z])([0-9])`).ReplaceAllStringFunc(fn, func(ns string) string {
			return string(ns[0]) + " " + string(ns[1])
		})
	}

	return fn
}

// check if field is of supported type and return error if not
func (v *validator) validateFieldType(fieldValue reflect.Value, fieldPath string) error {
	if !isSupported(fieldValue) {
		return errors.New("firevault: unsupported field type - " + fieldPath)
	}

	return nil
}

// skip field validation if value is zero and an omitempty tag is present
// (unless tags are skipped using options)
func (v *validator) shouldSkipField(
	fieldValue reflect.Value,
	fieldPath string,
	rules []string,
	opts validationOpts,
) bool {
	omitEmptyMethodTag := string("omitempty_" + opts.method)
	shouldOmitEmpty := slices.Contains(rules, "omitempty") || slices.Contains(rules, omitEmptyMethodTag)

	if shouldOmitEmpty && !slices.Contains(opts.emptyFieldsAllowed, fieldPath) {
		return !hasValue(fieldValue)
	}

	return false
}

// remove omitempty tags from rules
func (v *validator) cleanRules(rules []string) []string {
	cleanedRules := make([]string, 0, len(rules))

	for index, rule := range rules {
		if index != 0 && rule != "omitempty" && rule != string("omitempty_"+create) &&
			rule != string("omitempty_"+update) && rule != string("omitempty_"+validate) {
			cleanedRules = append(cleanedRules, rule)
		}
	}

	return cleanedRules
}

// validate field based on rules
func (v *validator) applyRules(
	ctx context.Context,
	fs *fieldScope,
	rules []string,
	method methodType,
) (reflect.Value, error) {
	fieldValue := fs.value

	for _, rule := range rules {
		// skip processing if the field is empty and it's not a required rule
		requiredMethodTag := string("required" + method)
		isRequiredRule := rule == "required" || rule == requiredMethodTag
		if !hasValue(fieldValue) && !isRequiredRule {
			continue
		}

		fe := &fieldError{
			field:        fs.field,
			structField:  fs.structField,
			displayField: fs.displayField,
			path:         fs.path,
			structPath:   fs.structPath,
			value:        fs.value.Interface(),
			kind:         fs.kind,
			typ:          fs.typ,
		}

		if strings.HasPrefix(rule, "transform=") {
			transName := strings.TrimPrefix(rule, "transform=")

			fe.tag = transName
			fs.tag = transName

			if transformation, ok := v.transformations[transName]; ok {
				newValue, err := transformation(ctx, fs)
				if err != nil {
					return reflect.Value{}, err
				}

				// check if rule returned a new value and assign it
				if newValue != nil {
					fieldValue = reflect.ValueOf(newValue)
				}
			} else {
				return reflect.Value{}, v.formatErr(fe)
			}
		} else {
			// get param value if present
			rule, param, _ := strings.Cut(rule, "=")

			fe.tag = rule
			fs.tag = rule
			fe.param = param
			fs.param = param

			if validation, ok := v.validations[rule]; ok {
				ok, err := validation(ctx, fs)
				if err != nil {
					return reflect.Value{}, err
				}
				if !ok {
					return reflect.Value{}, v.formatErr(fe)
				}
			} else {
				return reflect.Value{}, v.formatErr(fe)
			}
		}
	}

	return fieldValue, nil
}

// get final field value based on field's type
func (v *validator) processFinalValue(
	ctx context.Context,
	fs *fieldScope,
	opts validationOpts,
) (interface{}, error) {
	switch fs.kind {
	case reflect.Struct:
		return v.processStructValue(ctx, fs, opts)
	case reflect.Map:
		return v.processMapValue(ctx, fs, opts)
	case reflect.Array, reflect.Slice:
		return v.processSliceValue(ctx, fs, opts)
	default:
		return fs.value.Interface(), nil
	}
}

// get value if field is a struct
func (v *validator) processStructValue(
	ctx context.Context,
	fs *fieldScope,
	opts validationOpts,
) (interface{}, error) {
	// handle time.Time
	if fs.typ == reflect.TypeOf(time.Time{}) {
		return fs.value.Interface().(time.Time), nil
	}

	return v.validateFields(
		ctx,
		reflectedStruct{fs.typ, fs.value},
		fs.path,
		fs.structPath,
		opts,
	)
}

// get value if field is a map
func (v *validator) processMapValue(
	ctx context.Context,
	fs *fieldScope,
	opts validationOpts,
) (interface{}, error) {
	newMap := make(map[string]interface{})
	iter := fs.value.MapRange()

	for iter.Next() {
		key := iter.Key()
		val := iter.Value()

		newFs := &fieldScope{
			strct:       fs.strct,
			field:       key.String(),
			structField: key.String(),
			path:        fmt.Sprintf("%s.%v", fs.path, key.Interface()),
			structPath:  fmt.Sprintf("%s.%v", fs.structPath, key.Interface()),
			value:       val,
			kind:        val.Kind(),
			typ:         val.Type(),
		}

		processedValue, err := v.processFinalValue(ctx, newFs, opts)
		if err != nil {
			return nil, err
		}

		newMap[key.String()] = processedValue
	}

	return newMap, nil
}

// get value if field is a slice/array
func (v *validator) processSliceValue(
	ctx context.Context,
	fs *fieldScope,
	opts validationOpts,
) (interface{}, error) {
	newSlice := make([]interface{}, fs.value.Len())

	for i := 0; i < fs.value.Len(); i++ {
		val := fs.value.Index(i)

		newFs := &fieldScope{
			strct:       fs.strct,
			field:       fmt.Sprintf("[%d]", i),
			structField: fmt.Sprintf("[%d]", i),
			path:        fmt.Sprintf("%s[%d]", fs.path, i),
			structPath:  fmt.Sprintf("%s[%d]", fs.structPath, i),
			value:       val,
			kind:        val.Kind(),
			typ:         val.Type(),
		}

		processedValue, err := v.processFinalValue(ctx, newFs, opts)
		if err != nil {
			return nil, err
		}

		newSlice[i] = processedValue
	}

	return newSlice, nil
}

// parse rule tags
func (v *validator) parseTag(tag string) []string {
	rules := strings.Split(tag, ",")

	var validatedRules []string

	for _, rule := range rules {
		trimmedRule := strings.TrimSpace(rule)
		validatedRules = append(validatedRules, trimmedRule)
	}

	return validatedRules
}

// format fieldError
func (v *validator) formatErr(fe *fieldError) error {
	for _, formatter := range v.errFormatters {
		err := formatter(fe)
		if err != nil {
			return err
		}
	}

	return fe
}
