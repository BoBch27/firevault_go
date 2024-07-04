package firevault

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

type validator struct {
	validations     map[string]ValidationFn
	transformations map[string]TransformationFn
}

type ValidationFn func(reflect.Value, string) bool

type TransformationFn func(reflect.Value) interface{}

type reflectedStruct struct {
	types  reflect.Type
	values reflect.Value
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

func (v *validator) registerValidation(name string, validation ValidationFn) error {
	if len(name) == 0 {
		return errors.New("firevault: validation function Name cannot be empty")
	}

	if validation == nil {
		return fmt.Errorf("firevault: validation function %s cannot be empty", name)
	}

	v.validations[name] = validation
	return nil
}

func (v *validator) registerTransformation(name string, transformation TransformationFn) error {
	if len(name) == 0 {
		return errors.New("firevault: transformation function Name cannot be empty")
	}

	if transformation == nil {
		return fmt.Errorf("firevault: transformation function %s cannot be empty", name)
	}

	v.transformations[name] = transformation
	return nil
}

func (v *validator) validate(data interface{}, opts validationOpts) (map[string]interface{}, error) {
	rs := reflectedStruct{reflect.TypeOf(data), reflect.ValueOf(data)}

	if rs.values.Kind() != reflect.Pointer && rs.values.Kind() != reflect.Ptr {
		return nil, errors.New("firevault: data must be a pointer to a struct")
	}

	rs.values = rs.values.Elem()
	rs.types = rs.types.Elem()

	if rs.values.Kind() != reflect.Struct {
		return nil, errors.New("firevault: data must be a pointer to a struct")
	}

	dataMap, err := v.validateFields(rs, opts, "")
	return dataMap, err
}

func (v *validator) validateFields(
	rs reflectedStruct,
	opts validationOpts,
	path string,
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

		// get field path based on name tag and trim leading dot (if exists)
		fieldPath := strings.TrimPrefix(fmt.Sprintf("%s.%s", path, rules[0]), ".")

		// skip validation if value is zero and an omitempty tag is present
		// unless tags are skipped using options
		omitEmptyMethodTag := fmt.Sprintf("omitempty_%s", opts.method)
		shouldOmitEmpty := slices.Contains(rules, "omitempty") ||
			slices.Contains(rules, omitEmptyMethodTag)

		if shouldOmitEmpty {
			if !slices.Contains(opts.emptyFieldsAllowed, fieldPath) {
				if !hasValue(fieldValue) {
					continue
				}
			}
		}

		// remove omitempty tags from rules, so no validation is attempted
		rules = delSliceItem(rules, "omitempty")
		rules = delSliceItem(rules, fmt.Sprintf("omitempty_%s", create))
		rules = delSliceItem(rules, fmt.Sprintf("omitempty_%s", update))
		rules = delSliceItem(rules, fmt.Sprintf("omitempty_%s", validate))

		// get pointer value, only if it's not nil
		if fieldValue.Kind() == reflect.Pointer || fieldValue.Kind() == reflect.Ptr {
			if !fieldValue.IsNil() {
				fieldValue = fieldValue.Elem()
			}
		}

		// validate field based on rules
		for ruleIndex, rule := range rules {

			// use first tag rule as new field name, rather than having a "name=" prefix
			if ruleIndex == 0 && rule != "" {
				fieldName = rule
				continue
			}

			// skip validations depending on the passed in options
			if opts.skipValidation {
				continue
			}

			fe := &fieldError{
				code:        "",
				tag:         rule,
				field:       fieldName,
				structField: fieldType.Name,
				value:       fieldValue.Interface(),
				param:       "",
				kind:        fieldValue.Kind(),
				typ:         fieldValue.Type(),
			}

			if strings.HasPrefix(rule, "transform=") {
				// skip rule if value is zero
				if !hasValue(fieldValue) {
					continue
				}

				transName := strings.TrimPrefix(rule, "transform=")

				if transformation, ok := v.transformations[transName]; ok {
					newValue := transformation(fieldValue)

					// check if rule returned a new value and assign it
					if newValue != nil {
						fieldValue = reflect.ValueOf(newValue)
						rs.values.Field(i).Set(fieldValue)
					}
				} else {
					fe.code = "unknown-transformation"
					return nil, fe
				}
			} else {
				// skip rules (apart from "required") if value is zero
				requiredMethodTag := fmt.Sprintf("required_%s", opts.method)
				if rule != "required" && rule != requiredMethodTag && !hasValue(fieldValue) {
					continue
				}

				// get param value if present
				param := ""
				params := strings.Split(rule, "=")
				if len(params) > 1 {
					param = params[1]
					rule = params[0]
				}

				if validation, ok := v.validations[rule]; ok {
					ok := validation(fieldValue, param)
					if !ok {
						fe.code = "failed-validation"
						fe.param = param
						return nil, fe
					}
				} else {
					fe.code = "unknown-validation"
					fe.param = param
					return nil, fe
				}
			}
		}

		finalValue := fieldValue.Interface()

		// If the field is a nested struct, recursively validate it and add to map
		if fieldValue.Kind() == reflect.Struct {
			newStruct, err := v.validateFields(
				reflectedStruct{fieldValue.Type(), fieldValue},
				opts,
				fieldPath,
			)
			if err != nil {
				return nil, err
			}

			finalValue = newStruct
			// If the field is a nested struct in map, recursively validate it and add to map
		} else if fieldValue.Kind() == reflect.Map {
			iter := fieldValue.MapRange()
			newMap := make(map[string]interface{})

			for iter.Next() {
				val := iter.Value()
				key := iter.Key()

				if val.Kind() == reflect.Struct {
					newVal, err := v.validateFields(
						reflectedStruct{val.Type(), val},
						opts,
						fieldPath,
					)
					if err != nil {
						return nil, err
					}

					newMap[key.String()] = newVal
				} else {
					newMap[key.String()] = val.Interface()
				}
			}

			finalValue = newMap

			// If the field is a nested struct in slice, recursively validate it and add to map
		} else if fieldValue.Kind() == reflect.Array || fieldValue.Kind() == reflect.Slice {
			newSlice := make([]interface{}, 0)

			for idx := 0; idx < fieldValue.Len(); idx++ {
				val := fieldValue.Index(idx)

				if val.Kind() == reflect.Struct {
					newVal, err := v.validateFields(
						reflectedStruct{val.Type(), val},
						opts,
						fieldPath,
					)
					if err != nil {
						return nil, err
					}

					newSlice = append(newSlice, newVal)
				} else {
					newSlice = append(newSlice, val.Interface())
				}
			}

			finalValue = newSlice
		}

		dataMap[fieldName] = finalValue
	}

	return dataMap, nil
}

func (v *validator) parseTag(tag string) []string {
	rules := strings.Split(tag, ",")

	var validatedRules []string

	for _, rule := range rules {
		trimmedRule := strings.TrimSpace(rule)
		if trimmedRule != "" {
			validatedRules = append(validatedRules, trimmedRule)
		}
	}

	return validatedRules
}
