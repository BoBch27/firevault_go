package firevault

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"
)

// A ValidationFn is the function that's executed
// during a validation.
type ValidationFn func(ctx context.Context, path string, value reflect.Value, param string) (bool, error)

// A TransformationFn is the function that's executed
// during a transformation.
type TransformationFn func(ctx context.Context, path string, value reflect.Value) (interface{}, error)

type validator struct {
	validations     map[string]ValidationFn
	transformations map[string]TransformationFn
}

func newValidator() *validator {
	validator := &validator{make(map[string]ValidationFn), make(map[string]TransformationFn)}

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

	if rs.values.Kind() != reflect.Pointer && rs.values.Kind() != reflect.Ptr {
		return nil, errors.New("firevault: data must be a pointer to a struct")
	}

	rs.values = rs.values.Elem()
	rs.types = rs.types.Elem()

	if rs.values.Kind() != reflect.Struct {
		return nil, errors.New("firevault: data must be a pointer to a struct")
	}

	dataMap, err := v.validateFields(ctx, rs, "", opts)
	return dataMap, err
}

// loop through struct's fields and validate
// based on provided tags and options
func (v *validator) validateFields(
	ctx context.Context,
	rs reflectedStruct,
	path string,
	opts validationOpts,
) (map[string]interface{}, error) {
	// map which will hold all fields to pass to firestore
	dataMap := make(map[string]interface{})

	// iterate over struct fields
	for i := 0; i < rs.values.NumField(); i++ {
		fieldValue := rs.values.Field(i)
		fieldType := rs.types.Field(i)
		fieldName := fieldType.Name

		tag := fieldType.Tag.Get("firevault")

		if tag == "" || tag == "-" {
			continue
		}

		rules := v.parseTag(tag)

		// use first tag rule as new field name, if not empty
		if rules[0] != "" {
			fieldName = rules[0]
		}

		// get dot-separated field path
		fieldPath := v.getFieldPath(path, fieldName)

		// check if field is of supported type
		err := v.validateFieldType(fieldValue, fieldPath)
		if err != nil {
			return nil, err
		}

		// check if field should be skipped based on provided tags
		if v.shouldSkipField(fieldValue, fieldPath, rules, opts) {
			continue
		}

		// remove omitempty tags from rules, so no validation is attempted
		rules = v.cleanRules(rules)

		// get pointer value, only if it's not nil
		if fieldValue.Kind() == reflect.Pointer || fieldValue.Kind() == reflect.Ptr {
			if !fieldValue.IsNil() {
				fieldValue = fieldValue.Elem()
			}
		}

		// apply rules (both transformations and validations)
		// unless skipped using options
		if !opts.skipValidation {
			newFieldValue, err := v.applyRules(
				ctx,
				fieldValue,
				fieldPath,
				fieldName,
				fieldType.Name,
				rules,
				opts.method,
			)
			if err != nil {
				return nil, err
			}

			// set original struct's field value if changed
			if newFieldValue != fieldValue {
				rs.values.Field(i).Set(newFieldValue)
				fieldValue = newFieldValue
			}
		}

		// get the final value to be added to the data map
		finalValue, err := v.processFinalValue(ctx, fieldValue, fieldPath, opts)
		if err != nil {
			return nil, err
		}

		dataMap[fieldName] = finalValue
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
	fieldValue reflect.Value,
	fieldPath string,
	fieldName string,
	structFieldName string,
	rules []string,
	method methodType,
) (reflect.Value, error) {
	for _, rule := range rules {
		// skip processing if the field is empty and it's not a required rule
		requiredMethodTag := string("required" + method)
		isRequiredRule := rule == "required" || rule == requiredMethodTag
		if !hasValue(fieldValue) && !isRequiredRule {
			continue
		}

		fe := &fieldError{
			field:       fieldName,
			structField: structFieldName,
			value:       fieldValue.Interface(),
			kind:        fieldValue.Kind(),
			typ:         fieldValue.Type(),
			tag:         rule,
			param:       "",
		}

		if strings.HasPrefix(rule, "transform=") {
			transName := strings.TrimPrefix(rule, "transform=")

			if transformation, ok := v.transformations[transName]; ok {
				newValue, err := transformation(ctx, fieldPath, fieldValue)
				if err != nil {
					return reflect.Value{}, err
				}

				// check if rule returned a new value and assign it
				if newValue != nil {
					fieldValue = reflect.ValueOf(newValue)
				}
			} else {
				return reflect.Value{}, fe
			}
		} else {
			// get param value if present
			rule, param, _ := strings.Cut(rule, "=")

			if validation, ok := v.validations[rule]; ok {
				ok, err := validation(ctx, fieldPath, fieldValue, param)
				if err != nil {
					return reflect.Value{}, err
				}
				if !ok {
					fe.param = param
					return reflect.Value{}, fe
				}
			} else {
				fe.param = param
				return reflect.Value{}, fe
			}
		}
	}

	return fieldValue, nil
}

// get final field value based on field's type
func (v *validator) processFinalValue(
	ctx context.Context,
	fieldValue reflect.Value,
	fieldPath string,
	opts validationOpts,
) (interface{}, error) {
	switch fieldValue.Kind() {
	case reflect.Struct:
		return v.processStructValue(ctx, fieldValue, fieldPath, opts)
	case reflect.Map:
		return v.processMapValue(ctx, fieldValue, fieldPath, opts)
	case reflect.Array, reflect.Slice:
		return v.processSliceValue(ctx, fieldValue, fieldPath, opts)
	default:
		return fieldValue.Interface(), nil
	}
}

// get value if field is a struct
func (v *validator) processStructValue(
	ctx context.Context,
	fieldValue reflect.Value,
	fieldPath string,
	opts validationOpts,
) (interface{}, error) {
	// handle time.Time
	if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
		return fieldValue.Interface().(time.Time), nil
	}

	return v.validateFields(
		ctx,
		reflectedStruct{fieldValue.Type(), fieldValue},
		fieldPath,
		opts,
	)
}

// get value if field is a map
func (v *validator) processMapValue(
	ctx context.Context,
	fieldValue reflect.Value,
	fieldPath string,
	opts validationOpts,
) (interface{}, error) {
	newMap := make(map[string]interface{})
	iter := fieldValue.MapRange()

	for iter.Next() {
		key := iter.Key()
		val := iter.Value()

		newFieldPath := fmt.Sprintf("%s.%v", fieldPath, key.Interface())

		processedValue, err := v.processFinalValue(ctx, val, newFieldPath, opts)
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
	fieldValue reflect.Value,
	fieldPath string,
	opts validationOpts,
) (interface{}, error) {
	newSlice := make([]interface{}, fieldValue.Len())

	for i := 0; i < fieldValue.Len(); i++ {
		val := fieldValue.Index(i)
		newFieldPath := fmt.Sprintf("%s[%d]", fieldPath, i)

		processedValue, err := v.processFinalValue(ctx, val, newFieldPath, opts)
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
