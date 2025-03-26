package firevault

import (
	"context"
	"strings"
	"testing"
)

func BenchmarkValidateSimpleStruct(b *testing.B) {
	type SimpleStruct struct {
		Name  string `firevault:"name,required,min=3,max=50"`
		Email string `firevault:"email,required,email"`
		Age   int    `firevault:"age,min=18,max=120"`
	}

	v := newValidator()
	ctx := context.Background()
	opts := validationOpts{method: create}

	// Prepare a valid struct for benchmarking
	validData := &SimpleStruct{
		Name:  "John Doe",
		Email: "john.doe@example.com",
		Age:   30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := v.validate(ctx, validData, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateWithCustomRules(b *testing.B) {
	// Register a custom validation and transformation
	v := newValidator()
	err := v.registerValidation(
		"is_thirty",
		func(ctx context.Context, tx *Transaction, fs FieldScope) (bool, error) {
			return fs.Value().Int() == 30, nil
		},
		false,
		false,
	)
	if err != nil {
		b.Fatal(err)
	}

	err = v.registerTransformation(
		"lowercase",
		func(ctx context.Context, fs FieldScope) (interface{}, error) {
			return strings.ToLower(fs.Value().String()), nil
		},
		false,
		false,
	)
	if err != nil {
		b.Fatal(err)
	}

	type CustomStruct struct {
		Name  string `firevault:"name,required,min=3"`
		Email string `firevault:"email,required,email,transform=lowercase"`
		Age   int    `firevault:"age,min=18,max=120,is_thirty"`
	}

	ctx := context.Background()
	opts := validationOpts{method: create}

	// Prepare a valid struct with custom rules
	validData := &CustomStruct{
		Name:  "John Doe",
		Email: "john.doe@example.com",
		Age:   30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := v.validate(ctx, validData, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateStructWithSlice(b *testing.B) {
	type SimpleStruct struct {
		Name  string   `firevault:"name,required,min=3,max=50"`
		Email string   `firevault:"email,required,email"`
		Age   int      `firevault:"age,min=18,max=120"`
		Tags  []string `firevault:"tags,min=1,max=5"`
	}

	v := newValidator()
	ctx := context.Background()
	opts := validationOpts{method: create}

	// Prepare a valid struct for benchmarking
	validData := &SimpleStruct{
		Name:  "John Doe",
		Email: "john.doe@example.com",
		Age:   30,
		Tags:  []string{"tag1", "tag2"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := v.validate(ctx, validData, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateNestedStructWithSlice(b *testing.B) {
	type Address struct {
		Street string `firevault:"street,required"`
		City   string `firevault:"city,required"`
		Zip    string `firevault:"zip,required"`
	}

	type ComplexStruct struct {
		Name    string   `firevault:"name,required,min=3,max=50"`
		Email   string   `firevault:"email,required,email"`
		Age     int      `firevault:"age,min=18,max=120"`
		Address Address  `firevault:"address,required"`
		Tags    []string `firevault:"tags,min=1,max=5"`
	}

	v := newValidator()
	ctx := context.Background()
	opts := validationOpts{method: create}

	// Prepare a valid nested struct for benchmarking
	validData := &ComplexStruct{
		Name:  "John Doe",
		Email: "john.doe@example.com",
		Age:   30,
		Address: Address{
			Street: "123 Main St",
			City:   "Anytown",
			Zip:    "12345",
		},
		Tags: []string{"tag1", "tag2"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := v.validate(ctx, validData, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateSliceOfSimpleStructs(b *testing.B) {
	type User struct {
		Name  string `firevault:"name,required,min=3,max=50"`
		Email string `firevault:"email,required,email"`
		Age   int    `firevault:"age,min=18,max=120"`
	}

	v := newValidator()
	ctx := context.Background()
	opts := validationOpts{method: create}

	// Prepare a slice of valid structs for benchmarking
	validData := []*User{
		{
			Name:  "John Doe",
			Email: "john.doe@example.com",
			Age:   30,
		},
		{
			Name:  "Jane Smith",
			Email: "jane.smith@example.com",
			Age:   25,
		},
		{
			Name:  "Bob Johnson",
			Email: "bob.johnson@example.com",
			Age:   45,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, user := range validData {
			_, err := v.validate(ctx, user, opts)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
