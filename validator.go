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
type ValidationFn func(ctx context.Context, fs FieldScope) (bool, error)

// holds val func as well as whether it can be called on nil values
type valFnWrapper struct {
	fn       ValidationFn
	runOnNil bool
}

// A TransformationFn is the function that's executed
// during a transformation.
type TransformationFn func(ctx context.Context, fs FieldScope) (interface{}, error)

// holds trans func as well as whether it can be called on nil values
type transFnWrapper struct {
	fn       TransformationFn
	runOnNil bool
}

// An ErrorFormatterFn is the function that's executed
// to generate a custom, user-friendly error message,
// based on FieldError's fields.
//
// If the function returns a nil error, an instance
// of FieldError will be returned instead.
type ErrorFormatterFn func(fe FieldError) error

type validator struct {
	validations     map[string]valFnWrapper
	transformations map[string]transFnWrapper
	errFormatters   []ErrorFormatterFn
	cache           *structCache
}

func newValidator() *validator {
	validator := &validator{
		make(map[string]valFnWrapper, len(builtInValidators)),
		make(map[string]transFnWrapper, 0),
		make([]ErrorFormatterFn, 0),
		newStructCache(),
	}

	// Register predefined validators
	for name, val := range builtInValidators {
		runOnNil := false
		// required tags will be called on nil values
		if strings.Contains(name, "required") {
			runOnNil = true
		}

		// no need to error check here, built in validations are always valid
		_ = validator.registerValidation(name, val, true, runOnNil)
	}

	return validator
}

// register a validation
func (v *validator) registerValidation(
	name string,
	validation ValidationFn,
	builtIn bool,
	runOnNil bool,
) error {
	if v == nil {
		return errors.New("firevault: nil validator")
	}

	if len(name) == 0 {
		return errors.New("firevault: validation function name cannot be empty")
	}

	if !builtIn {
		_, exists := builtInValidators[name]
		if exists {
			return errors.New(
				"firevault: validation rule name is taken by a built-in validation function",
			)
		}
	}

	if validation == nil {
		return fmt.Errorf("firevault: validation function %s cannot be empty", name)
	}

	v.validations[name] = valFnWrapper{validation, runOnNil}
	return nil
}

// register a transformation
func (v *validator) registerTransformation(
	name string,
	transformation TransformationFn,
	runOnNil bool,
) error {
	if v == nil {
		return errors.New("firevault: nil validator")
	}

	if len(name) == 0 {
		return errors.New("firevault: transformation function name cannot be empty")
	}

	if transformation == nil {
		return fmt.Errorf("firevault: transformation function %s cannot be empty", name)
	}

	v.transformations[name] = transFnWrapper{transformation, runOnNil}
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

// options used during validation
type validationOpts struct {
	method             methodType
	skipValidation     bool
	emptyFieldsAllowed []string
	modifyOriginal     bool
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
	dataMap := make(map[string]interface{}, rs.values.NumField())

	sm, ok := v.cache.get(rs.types)
	if !ok {
		var err error
		sm, err = v.extractStructCache(rs, path, structPath, opts) // create cache
		if err != nil {
			return nil, err
		}
	}

	// iterate over struct fields
	for i := 0; i < len(sm.fields); i++ {
		// process individual field
		// (has side effects as it updates original struct after transformation)
		fieldName, fieldValue, err := v.processField(ctx, rs, rs.values.Field(i), sm.fields[i], opts)
		if err != nil {
			return nil, err
		}

		if fieldName != "" && fieldValue != nil {
			dataMap[fieldName] = fieldValue
		}
	}

	return dataMap, nil
}

func (v *validator) extractStructCache(
	rs reflectedStruct,
	path string,
	structPath string,
	opts validationOpts,
) (*structMetadata, error) {
	sm := &structMetadata{
		name:   rs.types.Name(),
		fields: make([]*fieldScope, rs.types.NumField()),
	}

	for i := 0; i < rs.values.NumField(); i++ {
		fieldValue := rs.values.Field(i)
		fieldType := rs.types.Field(i)

		// skip fields without firevault tag, or with an ignore tag
		tag := fieldType.Tag.Get("firevault")
		if tag == "" || tag == "-" {
			continue
		}

		fs := &fieldScope{
			strct:       rs.values,
			field:       fieldType.Name,
			structField: fieldType.Name,
			idx:         i,
			value:       fieldValue,
			kind:        fieldValue.Kind(),
			typ:         fieldType.Type,
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
		err := v.validateFieldType(fs.kind, fs.path)
		if err != nil {
			return nil, err
		}

		// check if field should be skipped based on provided tags
		fs.omitEmpty = v.shouldSkipField(rules, opts.method)
		fs.dive = slices.Contains(rules, "dive")

		// remove name, dive and omitempty tags from rules, so no validation is attempted
		rules = v.cleanRules(rules)
		fs.rules = v.extractTagCache(rules)

		// get pointer value, only if it's not nil
		if fs.kind == reflect.Pointer && !fs.value.IsNil() {
			fs.value = fs.value.Elem()
			fs.kind = fs.value.Kind()
			fs.typ = fs.value.Type()
			fs.pointer = true
		}

		sm.fields[i] = fs
	}

	v.cache.set(rs.types, sm)
	return sm, nil
}

// process individual field validations and transformations
func (v *validator) processField(
	ctx context.Context,
	rs reflectedStruct,
	val reflect.Value,
	fs *fieldScope,
	opts validationOpts,
) (string, interface{}, error) {
	// skip empty field with omitempty tags
	shouldOmit := fs.omitEmpty == "always" || fs.omitEmpty == string(opts.method)
	if shouldOmit && !slices.Contains(opts.emptyFieldsAllowed, fs.path) && !hasValue(fs.kind, val) {
		return "", nil, nil
	}

	// update cache to use new value
	fieldValue := val
	fs.value = val

	// handle pointers
	if fs.pointer {
		fs.value = fs.value.Elem()
	}

	// apply rules (both transformations and validations)
	// unless skipped using options
	if !opts.skipValidation {
		// (has side effects as it updates fs values)
		err := v.applyRules(ctx, fs)
		if err != nil {
			return "", nil, err
		}

		if opts.modifyOriginal {
			// set original struct's field value if changed
			if fieldValue != fs.value {
				rs.values.Field(fs.idx).Set(fs.value)
			}
		}
	}

	// get the final value to be added to the data map
	finalValue, err := v.processFinalValue(ctx, fs, opts)
	if err != nil {
		return "", nil, err
	}

	return fs.field, finalValue, nil
}

// get dot-separated field path
func (v *validator) getFieldPath(path string, fieldName string) string {
	if path == "" {
		return fieldName
	}

	return path + "." + fieldName
}

// check if field is of supported type and return error if not
func (v *validator) validateFieldType(fieldKind reflect.Kind, fieldPath string) error {
	if !isSupported(fieldKind) {
		return errors.New("firevault: unsupported field type - " + fieldPath)
	}

	return nil
}

// skip field validation if value is zero and an omitempty tag is present
// (unless tags are skipped using options)
func (v *validator) shouldSkipField(rules []string, method methodType) string {
	if slices.Contains(rules, "omitempty") {
		return "always"
	}

	if slices.Contains(rules, string("omitempty_"+method)) {
		return string(method)
	}

	return ""
}

// remove name, dive and omitempty tags from rules
func (v *validator) cleanRules(rules []string) []string {
	cleanedRules := make([]string, 0, len(rules))

	for index, rule := range rules {
		if index != 0 && rule != "omitempty" && rule != string("omitempty_"+create) &&
			rule != string("omitempty_"+update) && rule != string("omitempty_"+validate) &&
			rule != "dive" {
			cleanedRules = append(cleanedRules, rule)
		}
	}

	return cleanedRules
}

// validate field based on rules
func (v *validator) applyRules(
	ctx context.Context,
	fs *fieldScope,
) error {
	for _, rule := range fs.rules {
		if rule.isTransform {
			err := v.applyTransformation(ctx, fs, rule)
			if err != nil {
				return err
			}

			continue
		}

		err := v.applyValidation(ctx, fs, rule)
		if err != nil {
			return err
		}
	}

	return nil
}

// apply transformation rule
func (v *validator) applyTransformation(
	ctx context.Context,
	fs *fieldScope,
	rule *tagMetadata,
) error {
	fs.tag = rule.rule

	// skip processing if field is zero, unless stated otherwise during rule registration
	if !hasValue(fs.kind, fs.value) && !rule.runOnNil {
		return nil
	}

	newValue, err := rule.transFn(ctx, fs)
	if err != nil {
		return err
	}

	// check if transformation returns a new value and assign it
	if newValue != fs.value.Interface() {
		fs.value = reflect.ValueOf(newValue)
		fs.kind = fs.value.Kind()
		fs.typ = fs.value.Type()
	}

	return nil
}

// apply validation rule
func (v *validator) applyValidation(
	ctx context.Context,
	fs *fieldScope,
	rule *tagMetadata,
) error {
	fs.tag = rule.rule
	fs.param = rule.param

	// skip processing if field is zero, unless stated otherwise during rule registration
	if !hasValue(fs.kind, fs.value) && !rule.runOnNil {
		return nil
	}

	valid, err := rule.valFn(ctx, fs)
	if err != nil {
		return err
	}

	if !valid {
		return v.formatErr(fs)
	}

	return nil
}

// get final field value based on field's type
func (v *validator) processFinalValue(
	ctx context.Context,
	fs *fieldScope,
	opts validationOpts,
) (interface{}, error) {
	switch fs.kind {
	case reflect.Struct:
		// handle time.Time
		if fs.typ == reflect.TypeOf(time.Time{}) {
			return fs.value.Interface().(time.Time), nil
		}

		return v.validateFields(ctx, reflectedStruct{fs.typ, fs.value}, fs.path, fs.structPath, opts)
	case reflect.Map:
		if fs.dive {
			return v.processMapValue(ctx, fs, opts)
		}

		return fs.value.Interface(), nil
	case reflect.Array, reflect.Slice:
		if fs.dive {
			return v.processSliceValue(ctx, fs, opts)
		}

		return fs.value.Interface(), nil
	default:
		return fs.value.Interface(), nil
	}
}

// get value if field is a map
func (v *validator) processMapValue(
	ctx context.Context,
	fs *fieldScope,
	opts validationOpts,
) (map[string]interface{}, error) {
	newMap := make(map[string]interface{}, fs.value.Len())
	iter := fs.value.MapRange()

	for iter.Next() {
		key := iter.Key()
		val := iter.Value()
		kind := val.Kind()

		if kind == reflect.Pointer {
			val = val.Elem()
			kind = val.Kind()
		}

		newFs := &fieldScope{
			strct:       fs.strct,
			field:       key.String(),
			structField: key.String(),
			path:        fmt.Sprintf("%s.%v", fs.path, key.Interface()),
			structPath:  fmt.Sprintf("%s.%v", fs.structPath, key.Interface()),
			value:       val,
			kind:        kind,
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
) ([]interface{}, error) {
	newSlice := make([]interface{}, fs.value.Len())

	for i := 0; i < fs.value.Len(); i++ {
		val := fs.value.Index(i)
		kind := val.Kind()

		if kind == reflect.Pointer {
			val = val.Elem()
			kind = val.Kind()
		}

		newFs := &fieldScope{
			strct:       fs.strct,
			field:       fmt.Sprintf("[%d]", i),
			structField: fmt.Sprintf("[%d]", i),
			path:        fmt.Sprintf("%s[%d]", fs.path, i),
			structPath:  fmt.Sprintf("%s[%d]", fs.structPath, i),
			value:       val,
			kind:        kind,
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
	validRules := make([]string, 0, len(rules))

	for _, rule := range rules {
		trimmedRule := strings.TrimSpace(rule)
		validRules = append(validRules, trimmedRule)
	}

	return validRules
}

// format fieldError
func (v *validator) formatErr(fs *fieldScope) error {
	// set display field
	// done here so expensive regex matching is only done when an error must be returned
	fe := &fieldError{
		field:        fs.field,
		structField:  fs.structField,
		displayField: v.getDisplayName(fs.structField),
		path:         fs.path,
		structPath:   fs.structPath,
		value:        fs.value.Interface(),
		kind:         fs.kind,
		typ:          fs.typ,
		tag:          fs.tag,
		param:        fs.param,
	}

	for _, formatter := range v.errFormatters {
		err := formatter(fe)
		if err != nil {
			return err
		}
	}

	return fe
}

// get field struct name in a human-readable form
func (v *validator) getDisplayName(fieldName string) string {
	// handle snake case - replace underscores with spaces
	fn := strings.ReplaceAll(fieldName, "_", " ")

	// split camel and pascal case
	fn = lowerUpperBoundary.ReplaceAllStringFunc(fn, func(ns string) string {
		return string(ns[0]) + " " + string(ns[1])
	})

	// check if string contains a number
	if digitBoundary.MatchString(fn) {
		fn = upperDigitBoundary.ReplaceAllStringFunc(fn, func(ns string) string {
			return string(ns[0]) + " " + string(ns[1])
		})
		fn = lowerDigitBoundary.ReplaceAllStringFunc(fn, func(ns string) string {
			return string(ns[0]) + " " + string(ns[1])
		})
	}

	return fn
}

func (v *validator) extractTagCache(rules []string) []*tagMetadata {
	tags := make([]*tagMetadata, 0, len(rules))

	for _, rule := range rules {
		isTransform := strings.HasPrefix(rule, "transform=")
		var valFn ValidationFn
		var transFn TransformationFn
		var param string
		var runOnNil bool

		if isTransform {
			rule = strings.TrimPrefix(rule, "transform=")

			transWrapper, ok := v.transformations[rule]
			if !ok {
				continue
			}

			transFn = transWrapper.fn
			runOnNil = transWrapper.runOnNil
		} else {
			rule, param, _ = strings.Cut(rule, "=")

			valWrapper, ok := v.validations[rule]
			if !ok {
				continue
			}

			valFn = valWrapper.fn
			runOnNil = valWrapper.runOnNil
		}

		tags = append(tags, &tagMetadata{
			rule:        rule,
			valFn:       valFn,
			transFn:     transFn,
			isTransform: isTransform,
			param:       param,
			runOnNil:    runOnNil,
		})
	}

	return tags
}
