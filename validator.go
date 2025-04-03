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

type validator struct {
	validations     map[string]valFnWrapper
	transformations map[string]transFnWrapper
	errFormatters   []ErrorFormatterFunc
	cache           *structCache
}

func newValidator() *validator {
	validator := &validator{
		make(map[string]valFnWrapper, len(builtInValidators)),
		make(map[string]transFnWrapper, len(builtInTransformators)),
		make([]ErrorFormatterFunc, 0),
		&structCache{},
	}

	// register predefined validators
	for name, val := range builtInValidators {
		runOnNil := false
		// required rules will be called on nil values
		if strings.Contains(name, "required") {
			runOnNil = true
		}

		// no need to error check here, built in validations are always valid
		_ = validator.registerValidation(name, val.toValFuncInternal(), true, runOnNil)
	}

	// register predefined transformators
	for name, trans := range builtInTransformators {
		// no need to error check here, built in validations are always valid
		_ = validator.registerTransformation(name, trans.toTranFuncInternal(), true, false)
	}

	return validator
}

// the function that's executed during validations
type valFuncInternal func(ctx context.Context, tx *Transaction, fs FieldScope) (bool, error)

// holds val func as well as whether it can be called on nil values
type valFnWrapper struct {
	fn       valFuncInternal
	runOnNil bool
}

// register a validation
func (v *validator) registerValidation(
	name string,
	validation valFuncInternal,
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
		_, found := restrictedRules[name]
		if found || strings.ContainsAny(name, restrictedTagChars) {
			return errors.New(
				"firevault: validation rule contains restricted characters or is the same as a built-in rule",
			)
		}
	}

	if validation == nil {
		return fmt.Errorf("firevault: validation function %s cannot be empty", name)
	}

	v.validations[name] = valFnWrapper{validation, runOnNil}
	return nil
}

// the function that's executed during transformations
type tranFuncInternal func(ctx context.Context, tx *Transaction, fs FieldScope) (interface{}, error)

// holds transform func as well as whether it can be called on nil values
type transFnWrapper struct {
	fn       tranFuncInternal
	runOnNil bool
}

// register a transformation
func (v *validator) registerTransformation(
	name string,
	transformation tranFuncInternal,
	builtIn bool,
	runOnNil bool,
) error {
	if v == nil {
		return errors.New("firevault: nil validator")
	}

	if len(name) == 0 {
		return errors.New("firevault: transformation function name cannot be empty")
	}

	if !builtIn {
		_, found := restrictedRules[name]
		if found || strings.ContainsAny(name, restrictedTagChars) {
			return errors.New(
				"firevault: transformation rule contains restricted characters or is the same as a built-in rule",
			)
		}
	}

	if transformation == nil {
		return fmt.Errorf("firevault: transformation function %s cannot be empty", name)
	}

	v.transformations[name] = transFnWrapper{transformation, runOnNil}
	return nil
}

// ErrorFormatterFunc is the function that's executed
// to generate a custom, user-friendly error message,
// based on FieldError's fields.
//
// If the function returns a nil error, an instance
// of FieldError will be returned instead.
type ErrorFormatterFunc func(fe FieldError) error

// register an error formatter
func (v *validator) registerErrorFormatter(errFormatter ErrorFormatterFunc) error {
	if v == nil {
		return errors.New("firevault: nil validator")
	}

	if errFormatter == nil {
		return fmt.Errorf("firevault: error formatter function cannot be empty")
	}

	v.errFormatters = append(v.errFormatters, errFormatter)
	return nil
}

// options used during validation
type validationOpts struct {
	collPath           string
	method             methodType
	skipValidation     bool
	skipValFields      []string
	emptyFieldsAllowed []string
	modifyOriginal     bool
	deleteEmpty        bool
	tx                 *Transaction
}

// check if passed data is a struct pointer and reflect it if so
func (v *validator) validate(
	ctx context.Context,
	data interface{},
	opts validationOpts,
) (map[string]interface{}, error) {
	if v == nil {
		return nil, errors.New("firevault: nil validator")
	}

	fs := &fieldScope{
		collPath: opts.collPath,
		typ:      reflect.TypeOf(data),
		value:    reflect.ValueOf(data),
	}

	if fs.value.Kind() != reflect.Pointer {
		return nil, errors.New("firevault: data must be a pointer to a struct")
	}

	fs.typ = fs.typ.Elem()
	fs.value = fs.value.Elem()

	if fs.value.Kind() != reflect.Struct {
		return nil, errors.New("firevault: data must be a pointer to a struct")
	}

	dataMap, err := v.validateStructFields(ctx, fs, opts)
	return dataMap, err
}

// loop through struct's fields and validate
// based on provided tags and options
func (v *validator) validateStructFields(
	ctx context.Context,
	parentFs *fieldScope,
	opts validationOpts,
) (map[string]interface{}, error) {
	// map which will hold all fields to pass to firestore
	dataMap := make(map[string]interface{}, parentFs.value.NumField())

	// get cached struct data, if available
	sd, ok := v.cache.get(parentFs.typ)
	if !ok {
		// extract struct data and store in cache
		var err error
		sd, err = v.extractStructData(parentFs)
		if err != nil {
			return nil, err
		}
	}

	// iterate over struct fields
	for i := 0; i < len(sd.fields); i++ {
		cachedFs := sd.fields[i]
		newVal := parentFs.value.Field(i)

		// always use latest value
		if cachedFs.value != newVal {
			cachedFs.value = newVal
		}

		// use dynamic paths (of map/slice element as its key/index may have changed)
		if cachedFs.dynamic {
			cachedFs.path = v.getFieldPath(parentFs.path, cachedFs.field)
			cachedFs.structField = v.getFieldPath(parentFs.structPath, cachedFs.structField)
		}

		// process each individual field
		// (has side effects as it updates original struct after transformation (if allowed))
		fieldName, fieldValue, err := v.processStructField(ctx, cachedFs, opts)
		if err != nil {
			return nil, err
		}

		if fieldName != "" && fieldValue != nil {
			dataMap[fieldName] = fieldValue
		}
	}

	return dataMap, nil
}

// iterate over and collect struct fields data and store in cache
func (v *validator) extractStructData(parentFs *fieldScope) (*structData, error) {
	sd := &structData{
		name:   parentFs.typ.Name(),
		fields: make([]*fieldScope, parentFs.typ.NumField()),
	}

	for i := 0; i < parentFs.typ.NumField(); i++ {
		fieldValue := parentFs.value.Field(i)
		fieldType := parentFs.typ.Field(i)

		tag := fieldType.Tag.Get("firevault")

		// skip fields without firevault tag, or with an ignore tag
		if tag == "" || tag == "-" {
			continue
		}

		fs := &fieldScope{
			collPath:     parentFs.collPath,
			strct:        parentFs.value,
			field:        fieldType.Name,
			structField:  fieldType.Name,
			displayField: v.getDisplayName(fieldType.Name),
			value:        fieldValue,
			kind:         fieldType.Type.Kind(),
			typ:          fieldType.Type,
			dynamic:      parentFs.dynamic,
		}

		// parse tag into separate rules
		rules := v.parseTag(tag)

		// use first tag rule as new field name, if not empty
		if rules[0] != "" {
			fs.field = rules[0]
		}

		// get dot-separated field and struct path
		fs.path = v.getFieldPath(parentFs.path, fs.field)
		fs.structPath = v.getFieldPath(parentFs.structPath, fs.structField)

		// check if field is of supported type
		err := v.validateFieldType(fs.kind, fs.path)
		if err != nil {
			return nil, err
		}

		// check if and when field should be skipped based on provided rules
		fs.omitEmpty = v.shouldSkipField(rules)

		// check whether to dive into slice/map field
		fs.dive = slices.Contains(rules, "dive")

		// remove name, dive and omitempty from rules, so no validation is attempted
		rules = v.cleanRules(rules)

		// parse rules and generate rule data
		fs.rules = v.extractRuleData(rules)

		// get pointer value
		if fs.kind == reflect.Pointer {
			fs.pointer = true
			fs.value = fs.value.Elem()

			// get pointer metadata from type info, as value may be nil
			fs.typ = fs.typ.Elem()
			fs.kind = fs.typ.Kind()
		}

		// set cached struct field value
		sd.fields[i] = fs
	}

	// store in cache
	v.cache.set(parentFs.typ, sd)
	return sd, nil
}

// process individual field validations and transformations
func (v *validator) processStructField(
	ctx context.Context,
	fs *fieldScope,
	opts validationOpts,
) (string, interface{}, error) {
	// return if there's no field cache entry
	if fs == nil {
		return "", nil, nil
	}

	// skip empty field with omitempty tags
	shouldOmit := fs.omitEmpty == all || fs.omitEmpty == opts.method
	if shouldOmit && !slices.Contains(opts.emptyFieldsAllowed, fs.path) && !hasValue(fs.kind, fs.value) {
		// true if merging has been disabled (during an update)
		if opts.deleteEmpty {
			return fs.field, Delete, nil
		}

		return "", nil, nil
	}

	// handle pointers
	if fs.pointer {
		fs.value = fs.value.Elem()
	}

	// apply rules (both transformations and validations)
	// unless skipped using options
	if !opts.skipValidation || (len(opts.skipValFields) > 0 && !slices.Contains(opts.skipValFields, fs.path)) {
		// store field value in order to compare if it has changed after transformations
		fieldValue := fs.value

		// apply validations and transformations
		err := v.applyRules(ctx, fs, opts)
		if err != nil {
			return "", nil, err
		}

		// check if original struct value should be changed (can be thread-unsafe, hence option)
		if opts.modifyOriginal {
			// set original struct's field value if changed
			if fieldValue != fs.value {
				// use fieldValue as that stores the original field value
				fieldValue.Set(fs.value)
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

// parse rule tags
func (v *validator) parseTag(tag string) []string {
	rules := strings.Split(tag, ",")
	validRules := make([]string, len(rules))

	for idx, rule := range rules {
		trimmedRule := strings.TrimSpace(rule)
		validRules[idx] = trimmedRule
	}

	return validRules
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
	fn = lowerUpperBoundaryRegex().ReplaceAllStringFunc(fn, func(ns string) string {
		return string(ns[0]) + " " + string(ns[1])
	})

	// check if string contains a number
	if digitInstanceRegex().MatchString(fn) {
		fn = upperDigitBoundaryRegex().ReplaceAllStringFunc(fn, func(ns string) string {
			return string(ns[0]) + " " + string(ns[1])
		})
		fn = lowerDigitBoundaryRegex().ReplaceAllStringFunc(fn, func(ns string) string {
			return string(ns[0]) + " " + string(ns[1])
		})
	}

	return fn
}

// check if field is of supported type and return error if not
func (v *validator) validateFieldType(fieldKind reflect.Kind, fieldPath string) error {
	if !isSupported(fieldKind) {
		return errors.New("firevault: unsupported field type - " + fieldPath)
	}

	return nil
}

// return if and when to skip empty field, based on rules
func (v *validator) shouldSkipField(rules []string) methodType {
	if slices.Contains(rules, "omitempty") {
		return all
	}

	if slices.Contains(rules, string("omitempty_"+create)) {
		return create
	}

	if slices.Contains(rules, string("omitempty_"+update)) {
		return update
	}

	if slices.Contains(rules, string("omitempty_"+validate)) {
		return validate
	}

	return none
}

// remove name, dive and omitempty from rules
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

// parse specified rules and extract data for each
func (v *validator) extractRuleData(rules []string) []*ruleData {
	rulesData := make([]*ruleData, 0, len(rules))

	for _, rule := range rules {
		isTransform := strings.HasPrefix(rule, "transform=")
		var valFn valFuncInternal
		var transFn tranFuncInternal
		var param string
		var runOnNil bool
		var methodOnly methodType

		if strings.HasSuffix(rule, string("_"+create)) {
			methodOnly = create
		} else if strings.HasSuffix(rule, string("_"+update)) {
			methodOnly = update
		} else if strings.HasSuffix(rule, string("_"+validate)) {
			methodOnly = validate
		}

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

		rulesData = append(rulesData, &ruleData{
			name:        rule,
			valFn:       valFn,
			transFn:     transFn,
			isTransform: isTransform,
			param:       param,
			runOnNil:    runOnNil,
			methodOnly:  methodOnly,
		})
	}

	return rulesData
}

// validate field based on rules
func (v *validator) applyRules(
	ctx context.Context,
	fs *fieldScope,
	opts validationOpts,
) error {
	for _, rule := range fs.rules {
		// skip method specific rules which don't match current method
		if rule.methodOnly != "" && rule.methodOnly != opts.method {
			continue
		}

		if rule.isTransform {
			err := v.applyTransformation(ctx, opts.tx, fs, rule)
			if err != nil {
				return err
			}

			continue
		}

		err := v.applyValidation(ctx, opts.tx, fs, rule)
		if err != nil {
			return err
		}
	}

	return nil
}

// apply transformation rule
func (v *validator) applyTransformation(
	ctx context.Context,
	tx *Transaction,
	fs *fieldScope,
	rule *ruleData,
) error {
	fs.rule = rule.name

	// skip processing if field is zero, unless stated otherwise during rule registration
	if !hasValue(fs.kind, fs.value) && !rule.runOnNil {
		return nil
	}

	newValue, err := rule.transFn(ctx, tx, fs)
	if err != nil {
		return err
	}

	// check if transformation returns a new value and assign it, only if it isn't a different kind
	if newValue != fs.value.Interface() {
		newReflectedVal := reflect.ValueOf(newValue)
		if newReflectedVal.Kind() != fs.kind || newReflectedVal.Type() != fs.typ {
			return errors.New("firevault: transformed value cannot be of a different kind/type")
		}

		fs.value = newReflectedVal
	}

	return nil
}

// apply validation rule
func (v *validator) applyValidation(
	ctx context.Context,
	tx *Transaction,
	fs *fieldScope,
	rule *ruleData,
) error {
	fs.rule = rule.name
	fs.param = rule.param

	// skip processing if field is zero, unless stated otherwise during rule registration
	if !hasValue(fs.kind, fs.value) && !rule.runOnNil {
		return nil
	}

	valid, err := rule.valFn(ctx, tx, fs)
	if err != nil {
		return err
	}

	if !valid {
		return v.generateFieldErr(fs)
	}

	return nil
}

// get final field value based on field's type
func (v *validator) processFinalValue(
	ctx context.Context,
	fs *fieldScope,
	opts validationOpts,
) (interface{}, error) {
	// return nil if Value is invalid (nil pointer)
	if !fs.value.IsValid() {
		return nil, nil
	}

	switch fs.kind {
	case reflect.Struct:
		// handle time.Time
		if fs.typ == reflect.TypeOf(time.Time{}) {
			return fs.value.Interface().(time.Time), nil
		}

		return v.validateStructFields(ctx, fs, opts)
	case reflect.Map:
		// return map directly, without validating nested fields
		if !fs.dive {
			return fs.value.Interface(), nil
		}

		return v.processMapValue(ctx, fs, opts)
	case reflect.Array, reflect.Slice:
		// return slice/array directly, without validating nested fields
		if !fs.dive {
			return fs.value.Interface(), nil
		}

		return v.processSliceValue(ctx, fs, opts)
	default:
		return fs.value.Interface(), nil
	}
}

// process map's nested fields
func (v *validator) processMapValue(
	ctx context.Context,
	parentFs *fieldScope,
	opts validationOpts,
) (map[string]interface{}, error) {
	newMap := make(map[string]interface{}, parentFs.value.Len())
	iter := parentFs.value.MapRange()

	for iter.Next() {
		key := iter.Key()
		val := iter.Value()
		kind := val.Kind()

		if kind == reflect.Pointer {
			val = val.Elem()
			kind = val.Kind()
		}

		fs := &fieldScope{
			collPath:    parentFs.collPath,
			strct:       parentFs.strct,
			field:       key.String(),
			structField: key.String(),
			path:        fmt.Sprintf("%s.%v", parentFs.path, key.Interface()),
			structPath:  fmt.Sprintf("%s.%v", parentFs.structPath, key.Interface()),
			value:       val,
			kind:        kind,
			typ:         val.Type(),
			dynamic:     true,
		}

		processedValue, err := v.processFinalValue(ctx, fs, opts)
		if err != nil {
			return nil, err
		}

		newMap[key.String()] = processedValue
	}

	return newMap, nil
}

// process slice/array's nested fields
func (v *validator) processSliceValue(
	ctx context.Context,
	parentFs *fieldScope,
	opts validationOpts,
) ([]interface{}, error) {
	newSlice := make([]interface{}, parentFs.value.Len())

	for i := 0; i < parentFs.value.Len(); i++ {
		val := parentFs.value.Index(i)
		kind := val.Kind()

		if kind == reflect.Pointer {
			val = val.Elem()
			kind = val.Kind()
		}

		fs := &fieldScope{
			collPath:    parentFs.collPath,
			strct:       parentFs.strct,
			field:       fmt.Sprintf("[%d]", i),
			structField: fmt.Sprintf("[%d]", i),
			path:        fmt.Sprintf("%s[%d]", parentFs.path, i),
			structPath:  fmt.Sprintf("%s[%d]", parentFs.structPath, i),
			value:       val,
			kind:        kind,
			typ:         val.Type(),
			dynamic:     true,
		}

		processedValue, err := v.processFinalValue(ctx, fs, opts)
		if err != nil {
			return nil, err
		}

		newSlice[i] = processedValue
	}

	return newSlice, nil
}

// generate fieldError
func (v *validator) generateFieldErr(fs *fieldScope) error {
	fe := &fieldError{
		collPath:     fs.collPath,
		field:        fs.field,
		structField:  fs.structField,
		displayField: fs.displayField,
		path:         fs.path,
		structPath:   fs.structPath,
		value:        fs.value,
		kind:         fs.kind,
		typ:          fs.typ,
		rule:         fs.rule,
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
