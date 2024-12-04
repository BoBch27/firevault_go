package firevault

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
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
	fieldScopePool  sync.Pool
	fieldErrorPool  sync.Pool
	cache           *structCache
}

func newValidator() *validator {
	validator := &validator{
		make(map[string]valFnWrapper, len(builtInValidators)),
		make(map[string]transFnWrapper, 0),
		make([]ErrorFormatterFn, 0),
		sync.Pool{
			New: func() interface{} {
				return &fieldScope{}
			},
		},
		sync.Pool{
			New: func() interface{} {
				return &fieldError{}
			},
		},
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

	var err error
	sm, ok := v.cache.get(rs.types)
	if !ok {
		sm, err = v.extractStructCache(rs, path, structPath, opts) // create cache
		if err != nil {
			return nil, err
		}
	}

	// iterate over struct fields
	for i := 0; i < rs.values.NumField(); i++ {
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
		fields: make([]*fieldMetadata, rs.types.NumField()),
	}

	for i := 0; i < rs.values.NumField(); i++ {
		fieldValue := rs.values.Field(i)
		fieldType := rs.types.Field(i)

		// skip fields without firevault tag, or with an ignore tag
		tag := fieldType.Tag.Get("firevault")
		if tag == "" || tag == "-" {
			continue
		}

		fs := v.fieldScopePool.Get().(*fieldScope)
		defer v.fieldScopePool.Put(fs)
		*fs = fieldScope{
			strct:       rs.values,
			field:       fieldType.Name,
			structField: fieldType.Name,
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

		fm := &fieldMetadata{
			fieldScope: *fs,
			idx:        i,
		}

		// check if field should be skipped based on provided tags
		if v.shouldSkipField(fs, rules, opts) {
			fm.ignore = true
		}

		// remove omitempty tags from rules, so no validation is attempted
		rules = v.cleanRules(rules)
		fm.rules = v.extractTagCache(rules)

		// get pointer value, only if it's not nil
		if fs.kind == reflect.Pointer && !fs.value.IsNil() {
			fs.value = fs.value.Elem()
			fs.kind = fs.value.Kind()
			fs.typ = fs.value.Type()
		}

		if fs.kind == reflect.Struct {
			fm.nested = true
		}

		sm.fields[i] = fm
	}

	v.cache.set(rs.types, sm)
	return sm, nil
}

// process individual field validations and transformations
func (v *validator) processField(
	ctx context.Context,
	rs reflectedStruct,
	val reflect.Value,
	fm *fieldMetadata,
	opts validationOpts,
) (string, interface{}, error) {
	// update cache to use new value
	fieldValue := val
	fm.fieldScope.value = val

	// apply rules (both transformations and validations)
	// unless skipped using options
	if !opts.skipValidation {
		// (has side effects as it updates fs values)
		err := v.applyRules(ctx, fm)
		if err != nil {
			return "", nil, err
		}

		if opts.modifyOriginal {
			// set original struct's field value if changed
			if fieldValue != (&fm.fieldScope).value {
				rs.values.Field(fm.idx).Set(fm.value)
			}
		}
	}

	// get the final value to be added to the data map
	finalValue, err := v.processFinalValue(ctx, &fm.fieldScope, opts)
	if err != nil {
		return "", nil, err
	}

	return fm.field, finalValue, nil
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
func (v *validator) shouldSkipField(
	fs *fieldScope,
	rules []string,
	opts validationOpts,
) bool {
	omitEmptyMethodTag := string("omitempty_" + opts.method)
	shouldOmitEmpty := slices.Contains(rules, "omitempty") || slices.Contains(rules, omitEmptyMethodTag)

	if shouldOmitEmpty && !slices.Contains(opts.emptyFieldsAllowed, fs.path) {
		return !hasValue(fs.kind, fs.value)
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
	fm *fieldMetadata,
) error {
	for _, rule := range fm.rules {
		fe := v.fieldErrorPool.Get().(*fieldError)
		defer v.fieldErrorPool.Put(fe)
		*fe = fieldError{
			field:       fm.field,
			structField: fm.structField,
			path:        fm.path,
			structPath:  fm.structPath,
			value:       fm.value.Interface(),
			kind:        fm.kind,
			typ:         fm.typ,
		}

		if rule.isTransform {
			err := v.applyTransformation(ctx, fm, fe, rule)
			if err != nil {
				return err
			}

			continue
		}

		err := v.applyValidation(ctx, fm, fe, rule)
		if err != nil {
			return err
		}
	}

	return nil
}

// apply transformation rule
func (v *validator) applyTransformation(
	ctx context.Context,
	fm *fieldMetadata,
	fe *fieldError,
	rule *tagMetadata,
) error {
	fe.tag = rule.rule
	fm.fieldScope.tag = rule.rule

	// skip processing if field is zero, unless stated otherwise during rule registration
	if !hasValue(fm.kind, fm.value) && !rule.runOnNil {
		return nil
	}

	newValue, err := rule.transFn(ctx, &fm.fieldScope)
	if err != nil {
		return err
	}

	// check if transformation returns a new value and assign it
	if newValue != fm.value.Interface() {
		fm.fieldScope.value = reflect.ValueOf(newValue)
		fm.fieldScope.kind = fm.value.Kind()
		fm.fieldScope.typ = fm.value.Type()
	}

	return nil
}

// apply validation rule
func (v *validator) applyValidation(
	ctx context.Context,
	fm *fieldMetadata,
	fe *fieldError,
	rule *tagMetadata,
) error {
	fm.fieldScope.tag = rule.rule
	fe.tag = rule.rule
	fm.fieldScope.param = rule.param
	fe.param = rule.param

	// skip processing if field is zero, unless stated otherwise during rule registration
	if !hasValue(fm.kind, fm.value) && !rule.runOnNil {
		return nil
	}

	valid, err := rule.valFn(ctx, &fm.fieldScope)
	if err != nil {
		return err
	}

	if !valid {
		return v.formatErr(fe)
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
) (map[string]interface{}, error) {
	newMap := make(map[string]interface{}, fs.value.Len())
	iter := fs.value.MapRange()

	for iter.Next() {
		key := iter.Key()
		val := iter.Value()

		newFs := v.fieldScopePool.Get().(*fieldScope)
		defer v.fieldScopePool.Put(newFs)
		*newFs = fieldScope{
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
) ([]interface{}, error) {
	newSlice := make([]interface{}, fs.value.Len())

	for i := 0; i < fs.value.Len(); i++ {
		val := fs.value.Index(i)

		newFs := v.fieldScopePool.Get().(*fieldScope)
		defer v.fieldScopePool.Put(newFs)
		*newFs = fieldScope{
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
	validRules := make([]string, 0, len(rules))

	for _, rule := range rules {
		trimmedRule := strings.TrimSpace(rule)
		validRules = append(validRules, trimmedRule)
	}

	return validRules
}

// format fieldError
func (v *validator) formatErr(fe *fieldError) error {
	// set display field
	// done here so expensive regex matching is only done when an error must be returned
	fe.displayField = v.getDisplayName(fe.structField)

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
			transWrapper, ok := v.transformations[rule]
			if !ok {
				continue
			}

			rule = strings.TrimPrefix(rule, "transform=")
			transFn = transWrapper.fn
			runOnNil = transWrapper.runOnNil
		} else {
			valWrapper, ok := v.validations[rule]
			if !ok {
				continue
			}

			rule, param, _ = strings.Cut(rule, "=")
			valFn = valWrapper.fn
			runOnNil = valWrapper.runOnNil
		}

		tags = append(tags, &tagMetadata{
			rule:     rule,
			valFn:    valFn,
			transFn:  transFn,
			param:    param,
			runOnNil: runOnNil,
		})
	}

	return tags
}
