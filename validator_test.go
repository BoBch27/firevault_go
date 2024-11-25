package firevault

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	type Address struct {
		Street string `firevault:",required"`
		City   string `firevault:",required"`
	}

	type TestStruct struct {
		Name      string    `firevault:"name,required,min=3,max=50"`
		Age       int       `firevault:"age,min=18,max=120"`
		Email     string    `firevault:"email,required,email"`
		CreatedAt time.Time `firevault:"created_at,omitempty"`
		Address   Address   `firevault:"address"`
		Tags      []string  `firevault:"tags,min=1,max=5"`
	}

	tests := []struct {
		name    string
		data    interface{}
		opts    validationOpts
		wantErr bool
	}{
		{
			name: "Valid struct",
			data: &TestStruct{
				Name:    "John Doe",
				Age:     30,
				Email:   "john@example.com",
				Address: Address{Street: "123 Main St", City: "Anytown"},
				Tags:    []string{"tag1", "tag2"},
			},
			opts:    validationOpts{method: create},
			wantErr: false,
		},
		{
			name: "Invalid name (too short)",
			data: &TestStruct{
				Name:    "Jo",
				Age:     30,
				Email:   "john@example.com",
				Address: Address{Street: "123 Main St", City: "Anytown"},
			},
			opts:    validationOpts{method: create},
			wantErr: true,
		},
		{
			name: "Invalid age (too young)",
			data: &TestStruct{
				Name:    "John Doe",
				Age:     17,
				Email:   "john@example.com",
				Address: Address{Street: "123 Main St", City: "Anytown"},
			},
			opts:    validationOpts{method: create},
			wantErr: true,
		},
		{
			name: "Invalid email",
			data: &TestStruct{
				Name:    "John Doe",
				Age:     30,
				Email:   "not-an-email",
				Address: Address{Street: "123 Main St", City: "Anytown"},
			},
			opts:    validationOpts{method: create},
			wantErr: true,
		},
		{
			name: "Missing required field",
			data: &TestStruct{
				Name:    "John Doe",
				Age:     30,
				Address: Address{Street: "123 Main St", City: "Anytown"},
			},
			opts:    validationOpts{method: create},
			wantErr: true,
		},
		{
			name: "Invalid nested struct",
			data: &TestStruct{
				Name:    "John Doe",
				Age:     30,
				Email:   "john@example.com",
				Address: Address{Street: "123 Main St"},
			},
			opts:    validationOpts{method: create},
			wantErr: true,
		},
		{
			name: "Invalid tags (too many)",
			data: &TestStruct{
				Name:    "John Doe",
				Age:     30,
				Email:   "john@example.com",
				Address: Address{Street: "123 Main St", City: "Anytown"},
				Tags:    []string{"tag1", "tag2", "tag3", "tag4", "tag5", "tag6"},
			},
			opts:    validationOpts{method: create},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newValidator()
			_, err := v.validate(context.Background(), tt.data, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("validator.validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				_, ok := err.(*fieldError)
				if !ok {
					t.Errorf("Expected *fieldError, got %T", err)
				}
			}
		})
	}
}

func TestIndividualValidations(t *testing.T) {
	v := newValidator()

	tests := []struct {
		name      string
		rule      string
		value     interface{}
		param     string
		wantValid bool
	}{
		{"Required valid", "required", "test", "", true},
		{"Required invalid", "required", "", "", false},
		{"Min length valid", "min", "test", "3", true},
		{"Min length invalid", "min", "te", "3", false},
		{"Max length valid", "max", "test", "5", true},
		{"Max length invalid", "max", "testing", "5", false},
		{"Email valid", "email", "test@example.com", "", true},
		{"Email invalid", "email", "not-an-email", "", false},
		{"Min value valid", "min", 20, "18", true},
		{"Min value invalid", "min", 17, "18", false},
		{"Max value valid", "max", 100, "120", true},
		{"Max value invalid", "max", 121, "120", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, ok := v.validations[tt.rule]
			if !ok {
				t.Fatalf("Validation rule %s not found", tt.rule)
			}

			valid, err := validator(context.Background(), "test", reflect.ValueOf(tt.value), tt.param)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if valid != tt.wantValid {
				t.Errorf(
					"validator.%s() with value %v and param %s: got %v, want %v",
					tt.rule,
					tt.value,
					tt.param,
					valid,
					tt.wantValid,
				)
			}
		})
	}
}

func TestCustomRules(t *testing.T) {
	v := newValidator()

	// Custom validation rule
	err := v.registerValidation(
		"custom",
		func(ctx context.Context, path string, value reflect.Value, param string) (bool, error) {
			return value.String() == "custom", nil
		},
	)
	if err != nil {
		t.Fatalf("Failed to register custom validation: %v", err)
	}

	// Custom transformation rule
	err = v.registerTransformation(
		"uppercase",
		func(ctx context.Context, path string, value reflect.Value) (interface{}, error) {
			return strings.ToUpper(value.String()), nil
		},
	)
	if err != nil {
		t.Fatalf("Failed to register custom transformation: %v", err)
	}

	type CustomStruct struct {
		CustomField    string `firevault:"custom_field,custom"`
		UppercaseField string `firevault:"uppercase_field,transform=uppercase"`
	}

	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
		want    map[string]interface{}
	}{
		{
			name: "Valid custom validation and transformation",
			data: &CustomStruct{
				CustomField:    "custom",
				UppercaseField: "test",
			},
			wantErr: false,
			want: map[string]interface{}{
				"custom_field":    "custom",
				"uppercase_field": "TEST",
			},
		},
		{
			name: "Invalid custom validation",
			data: &CustomStruct{
				CustomField:    "not custom",
				UppercaseField: "test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.validate(context.Background(), tt.data, validationOpts{method: create})
			if (err != nil) != tt.wantErr {
				t.Errorf("validator.validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(result, tt.want) {
				t.Errorf("validator.validate() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestRegisterValidation(t *testing.T) {
	v := newValidator()

	tests := []struct {
		name       string
		valName    string
		validation ValidationFn
		wantErr    bool
	}{
		{
			name:    "Valid registration",
			valName: "test_validation",
			validation: func(
				ctx context.Context,
				path string,
				value reflect.Value,
				param string,
			) (bool, error) {
				return true, nil
			},
			wantErr: false,
		},
		{
			name:    "Empty name",
			valName: "",
			validation: func(
				ctx context.Context,
				path string,
				value reflect.Value,
				param string,
			) (bool, error) {
				return true, nil
			},
			wantErr: true,
		},
		{
			name:       "Nil validation function",
			valName:    "nil_validation",
			validation: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.registerValidation(tt.valName, tt.validation)
			if (err != nil) != tt.wantErr {
				t.Errorf("validator.registerValidation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegisterTransformation(t *testing.T) {
	v := newValidator()

	tests := []struct {
		name           string
		transName      string
		transformation TransformationFn
		wantErr        bool
	}{
		{
			name:      "Valid registration",
			transName: "test_transformation",
			transformation: func(
				ctx context.Context,
				path string,
				value reflect.Value,
			) (interface{}, error) {
				return value.Interface(), nil
			},
			wantErr: false,
		},
		{
			name:      "Empty name",
			transName: "",
			transformation: func(
				ctx context.Context,
				path string,
				value reflect.Value,
			) (interface{}, error) {
				return value.Interface(), nil
			},
			wantErr: true,
		},
		{
			name:           "Nil transformation function",
			transName:      "nil_transformation",
			transformation: nil,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.registerTransformation(tt.transName, tt.transformation)
			if (err != nil) != tt.wantErr {
				t.Errorf("validator.registerTransformation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
