# Firevault
Firevault is a [Firestore](https://cloud.google.com/firestore/) ODM for Go, providing robust data modelling and document handling.

Installation
------------
Use go get to install Firevault.

```go
go get github.com/bobch27/firevault_go
```

Importing
------------
Import the package in your code.

```go
import "github.com/bobch27/firevault_go"
```

Connection
------------
You can connect to Firevault using the `Connect` method, providing a project ID. A Firevault `Connection` is designed to be thread-safe and used as a singleton instance. A cache is used under the hood to store struct validation metadata, parsing validation tags once per struct type. Using multiple instances defeats the purpose of caching.

```go
import (
	"log"

	"github.com/bobch27/firevault_go"
)

// Sets your Google Cloud Platform project ID.
projectID := "YOUR_PROJECT_ID"
ctx := context.Background()

connection, err := firevault.Connect(ctx, projectID)
if err != nil {
	log.Fatalln("Firevault initialisation failed:", err)
}
```

To close the connection, when it's no longer needed, you can call the `Close` method. It need not be called at program exit.

```go
defer connection.Close()
```

Models
------------
Defining a model is as simple as creating a struct with Firevault tags.

```go
type User struct {
	Name     string   `firevault:"name,required,omitempty"`
	Email    string   `firevault:"email,required,email,is_unique,omitempty"`
	Password string   `firevault:"password,required,min=6,transform=hash_pass,omitempty"`
	Address  *Address `firevault:"address,omitempty"`
	Age      int      `firevault:"age,required,min=18,omitempty"`
}

type Address struct {
	Line1 string `firevault:",omitempty"`
	City  string `firevault:"-"`
}
```

Tags
------------
When defining a new struct type with a Firevault tag, note that the rules' order matters (apart from the different `omitempty` rules, which can be used anywhere). 

The first rule is always the **field name** which will be used in Firestore. You can skip that by just using a comma, before adding the others.

After that, each rule is a different validation, and they will be parsed in order.

Other than the validation rules, Firevault supports the following built-in ones:
- `omitempty` - If the field is set to it’s default value (e.g. `0` for `int`, or `""` for `string`), the field will be omitted from validation and Firestore.
- `omitempty_create` - Works the same way as `omitempty`, but only for the `Create` method. Ignored during `Update` and `Validate` methods.
- `omitempty_update` - Works the same way as `omitempty`, but only for the `Update` method. Ignored during `Create` and `Validate` methods.
- `omitempty_validate` - Works the same way as `omitempty`, but only for the `Validate` method. Ignored during `Create` and `Update` methods.
- `dive` - If the field is an array/slice or a map, this rule allows to recursively loop through and validate inner fields. Useful when the inner fields are structs with custom validation tags. Ignored for fields that are not arrays/slices or maps.
- `-` - Ignores the field.

Validations
------------
Firevault validates fields' values based on the defined rules. There are built-in validations, with support for adding **custom** ones. 

*Again, the order in which they are executed depends on the tag order.*

*Built-in validations:*
- `required` - Validates whether the field's value is not the default type value (i.e. `nil` for `pointer`, `""` for `string`, `0` for `int` etc.). Fails when it is the default.
- `required_create` - Works the same way as `required`, but only for the `Create` method. Ignored during `Update` and `Validate` methods.
- `required_update` - Works the same way as `required`, but only for the `Update` method. Ignored during `Create` and `Validate` methods.
- `required_validate` - Works the same way as `required`, but only for the `Validate` method. Ignored during `Create` and `Update` methods.
- `max` - Validates whether the field's value, or length, is less than or equal to the param's value. Requires a param (e.g. `max=20`). For numbers, it checks the value, for strings, maps and slices, it checks the length.
- `min` - Validates whether the field's value, or length, is greater than or equal to the param's value. Requires a param (e.g. `min=20`). For numbers, it checks the value, for strings, maps and slices, it checks the length.
- `email` - Validates whether the field's string value is a valid email address.

*Custom validations:*
- To define a custom validation, use `Connection`'s `RegisterValidation` method.
	- *Expects*:
		- name: A `string` defining the validation name. If the name includes a method-specific suffix ("_create", "_update", or "_validate"), the rule will be applied exclusively during calls to the corresponding method type and ignored for others.
		- func: A function that satisfies the `Validation` interface. Available types can be found in [validator_adaptors.go](https://github.com/bobch27/firevault_go/blob/main/validator_adaptors.go). The different types have different params, but the same return values.
			- *Expects*:
				- ctx: A context (only in `ValidationFuncCtx` and `ValidationFuncCtxTx`).
				- tx: A `Transaction` instance (only in `ValidationFuncTx` and `ValidationFuncCtxTx`).
				- fs: A value that implements the `FieldScope` interface, which gives access to different field data, useful during the validation. Available methods for `FieldScope` can be found in [field_scope.go](https://github.com/bobch27/firevault_go/blob/main/field_scope.go).
			- *Returns*:
				- result: A `bool` which returns `true` if check has passed, and `false` if it hasn't.
				- error: An `error` in case something went wrong during the check.
		- runOnNil *(optional)*: An optional `bool` indicating whether the validation should be executed on nil values. The default is `false`.

*Registering custom validations is not thread-safe. It is intended that all rules be registered, prior to any validation. Also, if a rule with the same name already exists, the previous one will be replaced.*

```go
connection.RegisterValidation(
	"is_upper", 
	ValidationFunc(func(fs FieldScope) (bool, error) {
		if fs.Kind() != reflect.String {
			return false, nil
		}

		s := fs.Value().String()
		return s == strings.toUpper(s), nil
	}),
)
```
```go
connection.RegisterValidation(
	"is_unique", 
	ValidationFuncCtxTx(func(ctx context.Context, tx *Transaction, fs FieldScope) (bool, error) {
		if tx == nil {
			return false, errors.New("is_unique validation should be done in a transaction")
		}

		// check DB to see if field exists (this read will be executed in a transaction)
		doc, err := Collection[User](connection, fs.Collection()).FindOne(
			ctx,
			NewQuery().Where(fs.Field(), "==", fs.Value().Interface()),
			NewOptions().Transaction(tx),
		)
		if err != nil {
			return false, err
		}
		if doc.ID != "" {
			return false, nil
		}

		return true, nil
	}),
)
```

You can then chain the rule like a normal one.

```go
type User struct {
	Name string `firevault:"name,required,is_upper,is_unique,omitempty"`
}
```

Transformations
------------
Firevault also supports rules that transform the field's value. There are built-in transformations, with support for adding **custom** ones. To use them, it's as simple as adding a prefix to the rule.

*Again, the order in which they are executed depends on the tag order.*

*Built-in transformations:*
- `uppercase` - Converts the field's string value to upper case. If the field is not a string, it simply returns its original value and no error.
- `lowercase` - Converts the field's string value to lower case. If the field is not a string, it simply returns its original value and no error.
- `trim_space` - Removes all leading and trailing white space around the field's string value. If the field is not a string, it simply returns its original value and no error.

*Custom transformations:*
- To define a transformation, use `Connection`'s `RegisterTransformation` method.
	- *Expects*:
		- name: A `string` defining the transformation name. If the name includes a method-specific suffix ("_create", "_update", or "_validate"), the rule will be applied exclusively during calls to the corresponding method type and ignored for others.
		- func: A function that satisfies the `Transformation` interface. Available types can be found in [validator_adaptors.go](https://github.com/bobch27/firevault_go/blob/main/validator_adaptors.go). The different types have different params, but the same return values.
			- *Expects*:
				- ctx: A context (only in `TransformationFuncCtx` and `TransformationFuncCtxTx`).
				- tx: A `Transaction` instance (only in `TransformationFuncTx` and `TransformationFuncCtxTx`).
				- fs: A value that implements the `FieldScope` interface, which gives access to different field data, useful during the transformation. Available methods for `FieldScope` can be found in [field_scope.go](https://github.com/bobch27/firevault_go/blob/main/field_scope.go).
			- *Returns*:
				- result: An `interface{}` with the new, transformed, value.
				- error: An `error` in case something went wrong during the transformation.
		- runOnNil *(optional)*: An optional `bool` indicating whether the transformation should be executed on nil values. The default is `false`.

*Registering custom transformations is not thread-safe. It is intended that all rules be registered, prior to any validation. Also, if a rule with the same name already exists, the previous one will be replaced.*

```go
connection.RegisterTransformation(
	"to_lower", 
	TransformationFunc(func(fs FieldScope) (interface{}, error) {
		if fs.Kind() != reflect.String {
			return fs.Value().Interface(), errors.New(fs.StructField() + " must be a string")
		}

		return strings.ToLower(fs.Value().String()), nil
	}),
)
```

You can then chain the rule like a normal one, but don't forget to use the `transform=` prefix.

*Again, the tag order matters. Defining a transformation at the end, means the value will be updated **after** the validations, whereas a definition at the start, means the field will be updated and **then** validated.*

```go
type User struct {
	// transformation will take place after all validations have passed
	Email string `firevault:"email,required,email,transform=to_lower,omitempty"`
}
```
```go
type User struct {
	// the "email" validation will be executed on the new value
	Email string `firevault:"email,required,transform=to_lower,email,omitempty"`
}
```

Collections
------------
A Firevault `CollectionRef` instance allows for interacting with Firestore, through various read and write methods.

These instances are lightweight and safe to create repeatedly. They can be freely used as needed, without concern for maintaining a singleton instance, as each instance independently references the specified Firestore collection.

To create a `CollectionRef` instance, call the `Collection` function, using the struct type parameter, and passing in the `Connection` instance, as well as a collection **path**.

```go
collection := firevault.Collection[User](connection, "users")
```

### Methods
The `CollectionRef` instance has **7** built-in methods to support interaction with Firestore.

- `Create` - A method which validates passed in data and adds it as a document to Firestore.
	- *Expects*:
		- ctx: A context.
		- data: A `pointer` of a `struct` with populated fields which will be added to Firestore after validation.
		- options *(optional)*: An instance of `Options` with the following chainable methods having an effect. 
			- SkipValidation: When used, it means all validation tags will be ingored (the `name` and `omitempty` rules will be acknowledged). If no field paths are provided, validation will be skipped for all fields. Otherwise, validation will only be skipped for the specified field paths.
			- AllowEmptyFields: When invoked with a variable number of `string` params, the fields that match the provided field paths will ignore the `omitempty` and `omitempty_create` rules. This can be useful when a field must be set to its zero value only on certain method calls. If not used, or called with no params, all fields will honour the two rules.
			- ModifyOriginal: When used, if there are transformations which alter field values, the original, passed in struct data will also be updated in place. Note: when used, this will make the entire method call thread-unsafe, so should be used with caution.
			- CustomID: When invoked with a `string` param, that value will be used as an ID when adding the document to Firestore. If not used, or called with no params, an auto generated ID will be used.
			- Transaction: When called with a `Transaction` instance, it ensures the operation is run as part of a transaction.
	- *Returns*:
		- id: A `string` with the new document's ID.
		- error: An `error` in case something goes wrong during validation or interaction with Firestore.
```go
user := User{
	Name: 	  "Bobby Donev",
	Email:    "hello@bobbydonev.com",
	Password: "12356",
	Age:      26,
	Address:  &Address{
		Line1: "1 High Street",
		City:  "London",
	},
}
id, err := collection.Create(ctx, &user)
if err != nil {
	fmt.Println(err)
} 
fmt.Println(id) // "6QVHL46WCE680ZG2Xn3X"
```
```go
id, err := collection.Create(
	ctx, 
	&user, 
	NewOptions().CustomID("custom-id"),
)
if err != nil {
	fmt.Println(err)
} 
fmt.Println(id) // "custom-id"
```
```go
user := User{
	Name: 	  "Bobby Donev",
	Email:    "hello@bobbydonev.com",
	Password: "12356",
	Age:      0,
	Address:  &Address{
		Line1: "1 High Street",
		City:  "London",
	},
}
id, err := collection.Create(
	ctx, 
	&user, 
	NewOptions().AllowEmptyFields("age"),
)
if err != nil {
	fmt.Println(err)
} 
fmt.Println(id) // "6QVHL46WCE680ZG2Xn3X"
```
- `Update` - A method which validates passed in data and updates all Firestore documents which match provided `Query`. The method uses Firestore's `BulkWriter` under the hood, meaning the operation is not atomic.
	- *Expects*:
		- ctx: A context.
		- query: A `Query` instance to filter which documents to update.
		- data: A `pointer` of a `struct` with populated fields which will be used to update the documents after validation.
		- options *(optional)*: An instance of `Options` with the following chainable methods having an effect. 
			- SkipValidation: When used, it means all validation tags will be ingored (the `name` and `omitempty` rules will be acknowledged). If no field paths are provided, validation will be skipped for all fields. Otherwise, validation will only be skipped for the specified field paths.
			- AllowEmptyFields: When invoked with a variable number of `string` params, the fields that match the provided field paths will ignore the `omitempty` and `omitempty_update` rules. This can be useful when a field must be set to its zero value only on certain method calls. If not used, or called with no params, all fields will honour the two rules.
			- ModifyOriginal: When used, if there are transformations which alter field values, the original, passed in struct data will also be updated in place. Note: when used, this will make the entire method call thread-unsafe, so should be used with caution.
			- ReplaceAll: When used, the merging of fields will be disabled, meaning the entire document will be replaced - no existing fields will be preserved. The deletion of fields is based on the provided struct, not the Firestore document itself. If the struct has changed since the document was created, some fields may not be deleted.
			- ReplaceFields: When invoked with a variable number of `string` params, the fields which match the provided field paths will be fully overwritten. Other fields on the document will be untouched. If not used, or called with no params, all the fields given in the data argument will be overwritten (unless `ReplaceAll` was used). If a provided field path does not refer to a value in the data passed, it'll be ignored.
			- RequireLastUpdateTime: When invoked with a `time.Time` timestamp, the operation will only proceed if the document's last update time matches the given timestamp exactly. Else, the operation fails with an error.
			- Transaction: When called with a `Transaction` instance, it ensures the operation is run as part of a transaction.
	- *Returns*:
		- error: An `error` in case something goes wrong during validation or interaction with Firestore.
	- ***Important***: 
		- If neither `omitempty`, nor `omitempty_update` rules have been used, non-specified field values in the passed in data will be set to Go's default values, thus updating all document fields. To prevent that behaviour, please use one of the two rules. 
```go
user := User{
	Password: "123567",
}
err := collection.Update(
	ctx, 
	NewQuery().ID("6QVHL46WCE680ZG2Xn3X"), 
	&user,
)
if err != nil {
	fmt.Println(err)
} 
fmt.Println("Success")
```
```go
user := User{
	Address:  &Address{
		Line1: "1 Main Road",
		City:  "New York",
	}
}
err := collection.Update(
	ctx, 
	NewQuery().ID("6QVHL46WCE680ZG2Xn3X"), 
	&user, 
	NewOptions().ReplaceFields("address.Line1"),
)
if err != nil {
	fmt.Println(err)
} 
fmt.Println("Success") // only the address.Line1 field will be updated
```
```go
user := User{
	Address:  &Address{
		Line1: "1 Main Road",
		City:  "New York",
	}
}
err := collection.Update(
	ctx, 
	NewQuery().ID("6QVHL46WCE680ZG2Xn3X"), 
	&user, 
	NewOptions().SkipValidation("address.Line1"),
)
if err != nil {
	fmt.Println(err)
} 
fmt.Println("Success") // no validation will be performed on the address.Line1 field
```
- `Validate` - A method which validates and transforms passed in data. 
	- *Expects*:
		- ctx: A context.
		- data: A `pointer` of a `struct` with populated fields which will be validated.
		- options *(optional)*: An instance of `Options` with the following chainable methods having an effect. 
			- SkipValidation: When used, it means all validation tags will be ingored (the `name` and `omitempty` rules will be acknowledged). If no field paths are provided, validation will be skipped for all fields. Otherwise, validation will only be skipped for the specified field paths.
			- AllowEmptyFields: When invoked with a variable number of `string` params, the fields that match the provided field paths will ignore the `omitempty` and `omitempty_validate` rules. This can be useful when a field must be set to its zero value only on certain method calls. If not used, or called with no params, all fields will honour the two rules.
			- ModifyOriginal: When used, if there are transformations which alter field values, the original, passed in struct data will also be updated in place. Note: when used, this will make the entire method call thread-unsafe, so should be used with caution.
			- AsCreate: When used, it allows the application of the same rules as if performing a `Create` operation (e.g. `required_create`), i.e. it performs the same validation as the one before document creation.
			- AsUpdate: When used, it allows the application of the same rules as if performing an `Update` operation (e.g. `required_update`), i.e. it performs the same validation as the one before document updating.
	- *Returns*:
		- error: An `error` in case something goes wrong during validation.
	- ***Important***: 
		- If neither `omitempty`, nor `omitempty_validate` rules have been used, non-specified field values in the passed in data will be set to Go's default values. 
```go
user := User{
	Email: "HELLO@BOBBYDONEV.COM",
}
err := collection.Validate(ctx, &user)
if err != nil {
	fmt.Println(err)
} 
fmt.Println(user) // {hello@bobbydonev.com}
```
```go
user := User{
	Email: "HELLO@BOBBYDONEV.COM",
}
// this will run the same validation as the one in collection.Update()
// so rules like "required_update" will be applied instead of "required_validate"
err := collection.Validate(
	ctx, 
	&user,
	NewOptions().AsUpdate()
)
if err != nil {
	fmt.Println(err)
} 
fmt.Println(user) // {hello@bobbydonev.com}
```
- `Delete` - A method which deletes all Firestore documents which match provided `Query`. The method uses Firestore's `BulkWriter` under the hood, meaning the operation is not atomic.
	- *Expects*:
		- ctx: A context.
		- query: A `Query` instance to filter which documents to delete.
		- options *(optional)*: An instance of `Options` with the following chainable methods having an effect.
			- RequireExists: When used, the operation will only proceed if the document exists. Else, the operation fails with an error. This option overrides any previous calls to RequireLastUpdateTime.
			- RequireLastUpdateTime: When invoked with a `time.Time` timestamp, the operation will only proceed if the document's last update time matches the given timestamp exactly. Else, the operation fails with an error. This option overrides any previous calls to RequireExists.
			- Transaction: When called with a `Transaction` instance, it ensures the operation is run as part of a transaction.
	- *Returns*:
		- error: An `error` in case something goes wrong during interaction with Firestore.
```go
err := collection.Delete(
	ctx, 
	NewQuery().ID("6QVHL46WCE680ZG2Xn3X"),
)
if err != nil {
	fmt.Println(err)
} 
fmt.Println("Success")
```
```go
err := collection.Delete(
	ctx, 
	NewQuery().ID("6QVHL46WCE680ZG2Xn3X"),
	NewOptions().RequireExists(),
)
if err != nil {
	fmt.Println(err)
} 
fmt.Println("Success")
```
- `Find` - A method which gets the Firestore documents which match the provided `Query`.
	- *Expects*:
		- ctx: A context.
		- query: A `Query` to filter and order documents.
		- options *(optional)*: An instance of `Options` with the following chainable methods having an effect.
 			- Transaction: When called with a `Transaction` instance, it ensures the operation is run as part of a transaction.
	- *Returns*: 
		- docs: A `slice` containing the results of type `Document[T]` (where `T` is the type used when initiating the collection instance). `Document[T]` has three properties.
			- ID: A `string` which holds the document's ID.
			- Data: The document's data of type `T`.
			- Metadata: The document's read-only metadata:
				- CreateTime: The time at which the document was created.
				- UpdateTime: The time at which the document was last changed.
				- ReadTime: The time at which the document was read.
		- error: An `error` in case something goes wrong during interaction with Firestore.
```go
users, err := collection.Find(
	ctx, 
	NewQuery().
		Where("email", "==", "hello@bobbydonev").
		Limit(1),
)
if err != nil {
	fmt.Println(err)
} 
fmt.Println(users) // []Document[User]
fmt.Println(users[0].ID) // 6QVHL46WCE680ZG2Xn3X
```
- `FindOne` - A method which gets the first Firestore document which matches the provided `Query`.
	- *Expects*:
		- ctx: A context.
		- query: A `Query` to filter and order documents.
		- options *(optional)*: An instance of `Options` with the following chainable methods having an effect.
 			- Transaction: When called with a `Transaction` instance, it ensures the operation is run as part of a transaction.
	- *Returns*:
		- doc: Returns the document with type `Document[T]` (where `T` is the type used when initiating the collection instance). `Document[T]` has three properties.
			- ID: A `string` which holds the document's ID.
			- Data: The document's data of type `T`.
			- Metadata: The document's read-only metadata:
				- CreateTime: The time at which the document was created.
				- UpdateTime: The time at which the document was last changed.
				- ReadTime: The time at which the document was read.
		- error: An `error` in case something goes wrong during interaction with Firestore.
```go
user, err := collection.FindOne(
	ctx, 
	NewQuery().ID("6QVHL46WCE680ZG2Xn3X"),
)
if err != nil {
	fmt.Println(err)
} 
fmt.Println(user.Data) // {Bobby Donev hello@bobbydonev.com asdasdkjahdks 26 0xc0001d05a0}
```
```go
 user, err := collection.FindOne(
 	ctx, 
 	NewQuery().ID("6QVHL46WCE680ZG2Xn3X"),
 	NewOptions().Transaction(tx),
 )
 if err != nil {
 	fmt.Println(err)
 }
 fmt.Println(user.Data) // {Bobby Donev hello@bobbydonev.com asdasdkjahdks 26 0xc0001d05a0}
 ```
- `Count` - A method which gets the number of Firestore documents which match the provided `Query`.
	- *Expects*:
		- ctx: A context.
		- query: An instance of `Query` to filter documents.
	- *Returns*: 
		- count: An `int64` representing the number of documents which meet the criteria.
		- error: An `error` in case something goes wrong during interaction with Firestore.
```go
count, err := collection.Count(
	ctx, 
	NewQuery().Where("email", "==", "hello@bobbydonev"),
)
if err != nil {
	fmt.Println(err)
} 
fmt.Println(count) // 1
```

Queries
------------
A Firevault `Query` instance allows querying Firestore, by chaining various methods. The query can have multiple filters.

To create a `Query` instance, call the `NewQuery` method.

```go
query := firevault.NewQuery()
```

### Methods
The `Query` instance has **10** built-in methods to support filtering and ordering Firestore documents.

- `ID` - Returns a new `Query` that that exclusively filters the set of results based on provided IDs.
	- *Expects*:
		- ids: A varying number of `string` values used to filter out results.
	- *Returns*:
		- A new `Query` instance.
	- ***Important***:
		- ID takes precedence over and completely overrides any previous or subsequent calls to other Query methods, including Where. To filter by ID as well as other criteria, use the Where method with the special DocumentID field, instead of calling ID.
```go
newQuery := query.ID("6QVHL46WCE680ZG2Xn3X")
```
- `Where` - Returns a new `Query` that filters the set of results. 
	- *Expects*:
		- path: A `string` which can be a single field or a dot-separated sequence of fields.
		- operator: A `string` which must be one of `==`, `!=`, `<`, `<=`, `>`, `>=`, `array-contains`, `array-contains-any`, `in` or `not-in`.
		- value: An `interface{}` value used to filter out the results.
	- *Returns*:
		- A new `Query` instance.
```go
newQuery := query.Where("name", "==", "Bobby Donev")
```
- `OrderBy` - Returns a new `Query` that specifies the order in which results are returned. 
	- *Expects*:
		- path: A `string` which can be a single field or a dot-separated sequence of fields. To order by document name, use the special field path `DocumentID`.
		- direction: A `Direction` used to specify whether results are returned in ascending or descending order.
	- *Returns*:
		- A new `Query` instance.
```go
newQuery := query.Where("name", "==", "Bobby Donev").OrderBy("age", Asc)
```
- `Limit` - Returns a new `Query` that specifies the maximum number of first results to return. 
	- *Expects*:
		- num: An `int` which indicates the max number of results to return.
	- *Returns*:
		- A new `Query` instance.
```go
newQuery := query.Where("name", "==", "Bobby Donev").Limit(1)
```
- `LimitToLast` - Returns a new `Query` that specifies the maximum number of last results to return. 
	- *Expects*:
		- num: An `int` which indicates the max number of results to return.
	- *Returns*:
		- A new `Query` instance.
```go
newQuery := query.Where("name", "==", "Bobby Donev").LimitToLast(1)
```
- `Offset` - Returns a new `Query` that specifies the number of initial results to skip. 
	- *Expects*:
		- num: An `int` which indicates the number of results to skip.
	- *Returns*:
		- A new `Query` instance.
```go
newQuery := query.Where("name", "==", "Bobby Donev").Offset(1)
```
- `StartAt` - Returns a new `Query` that specifies that results should start at the document with the given field values. Should be called with one field value for each OrderBy clause, in the order that they appear.
	- *Expects*:
		- values: A varying number of `interface{}` values used to filter out results.
	- *Returns*:
		- A new `Query` instance.
```go
newQuery := query.Where("name", "==", "Bobby Donev").OrderBy("age", Asc).StartAt(25)
```
- `StartAfter` - Returns a new `Query` that specifies that results should start just after the document with the given field values. Should be called with one field value for each OrderBy clause, in the order that they appear.
	- *Expects*:
		- values: A varying number of `interface{}` values used to filter out results.
	- *Returns*:
		- A new `Query` instance.
```go
newQuery := query.Where("name", "==", "Bobby Donev").OrderBy("age", Asc).StartAfter(25)
```
- `EndBefore` - Returns a new `Query` that specifies that results should end just before the document with the given field values. Should be called with one field value for each OrderBy clause, in the order that they appear.
	- *Expects*:
		- values: A varying number of `interface{}` values used to filter out results.
	- *Returns*:
		- A new `Query` instance.
```go
newQuery := query.Where("name", "==", "Bobby Donev").OrderBy("age", Asc).EndBefore(25)
```
- `EndAt` - Returns a new `Query` that specifies that results should end at the document with the given field values. Should be called with one field value for each OrderBy clause, in the order that they appear.
	- *Expects*:
		- values: A varying number of `interface{}` values used to filter out results.
	- *Returns*:
		- A new `Query` instance.
```go
newQuery := query.Where("name", "==", "Bobby Donev").OrderBy("age", Asc).EndAt(25)
```

Options
------------
A Firevault `Options` instance allows for the overriding of default options for validation, creation and updating methods, by chaining various methods.

To create a new `Options` instance, call the `NewOptions` method.

```go
options := firevault.NewOptions()
```

### Methods
The `Options` instance has **11** built-in methods to support overriding default `CollectionRef` method options. Some options only apply to specific `CollectionRef` methods.

- `SkipValidation` - Returns a new `Options` instance that allows to skip the data validation during `Create`, `Update` and `Validate` methods. The "name" rule, "omitempty" rules and "ignore" rule will still be honoured. If no field paths are provided, validation will be skipped for all fields. Otherwise, validation will only be skipped for the specified field paths.
	- *Expects*:
		- path: A varying number of `string` values (using dot separation) used to select field paths.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.SkipValidation("name")
```
- `AllowEmptyFields` - Returns a new `Options` instance that allows to specify which field paths should ignore the "omitempty" rules. Only applies to the `Validate`, `Create` and `Update` methods.
	- *Expects*:
		- path: A varying number of `string` values (using dot separation) used to select field paths.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.AllowEmptyFields("age")
```
- `ModifyOriginal` - Returns a new `Options` instance that allows the updating of field values in the original passed in data struct after transformations. Note, this will make the operation thread-unsafe, so should be used with caution. Only applies to the `Validate`, `Create` and `Update` methods.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.ModifyOriginal()
```
- `AsCreate` - Returns a new `Options` instance that allows the application of the same rules as if performing a `Create` operation (e.g. `required_create`). Only applies to the `Validate` method.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.AsCreate()
```
- `AsUpdate` - Returns a new `Options` instance that allows the application of the same rules as if performing an `Update` operation (e.g. `required_update`). Only applies to the `Validate` method.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.AsUpdate()
```
- `CustomID` - Returns a new `Options` instance that allows to specify a custom document ID to be used when creating a Firestore document. Only applies to the `Create` method.
	- *Expects*:
		- id: A `string` specifying the custom ID.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.CustomID("custom-id")
```
- `ReplaceAll` - Returns a new `Options` instance that allows to disable the merging of fields, meaning the entire document will be replaced (i.e. no existing fields will be preserved). Only applies to the `Update` method.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.ReplaceAll()
```
- `ReplaceFields` - Returns a new `Options` instance that allows to specify which field paths to be fully overwritten. Other fields on the existing document will be untouched. Only applies to the `Update` method.
	- *Expects*:
		- path: A varying number of `string` values (using dot separation) used to select field paths.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.ReplaceFields("address.Line1")
```
- `RequireLastUpdateTime` - Returns a new `Options` instance that allows to add a precondition that the document must exist and have the specified last update timestamp before proceeding with the operation. Else, the operation fails with an error. Only applies to the `Update` and `Delete` methods.
	- *Expects*:
		- timestamp: A `time.Time` value to compare against the document's last update time. Must be microsecond aligned.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.RequireLastUpdateTime(time.Now())
```
- `RequireExists` - Returns a new `Options` instance that allows to add a precondition that the document must exist before proceeding with the operation. Else, the operation fails with an error. Only applies to the `Delete` method.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.RequireExists()
```
- `Transaction` - Returns a new `Options` instance that allows to add a transaction instance, ensuring the operation is executed as part of a transaction. Does not apply to the `Validate` method.
 	- *Expects*:
 		- tx: A `Transaction` instance to ensure operation is run within a transaction.
 	- *Returns*:
 		- A new `Options` instance.
 ```go
 newOptions := options.Transaction(tx)
 ```

Transactions
------------
Firevault supports **Firestore transactions** for atomic read-write operations, allowing you to interact with documents within a single, consistent operation. Transactions ensure that all changes are either fully committed or fully rolled back in case of an error, making your database interactions more reliable.
 
To perform operations within a transaction, pass a transaction instance as an option when calling methods like `Update`, `Delete`, `Create`, etc. Firevault will automatically handle the transaction logic for you.

```go
func runSimpleTransaction(ctx context.Context, connection *Connection) error {
	// ...

	collection := Collection[User](connection, "users")
	query := NewQuery().Where("age", "==", 18)
	updates := &User{Age: 19}

	// Run a new transaction
	err := connection.RunTransaction(ctx, func(ctx context.Context, tx *Transaction) error {
		// Create new Options instance with Transaction and make sure to pass it into each 
		// CollectionRef method inside the transaction
		opts := NewOptions().Transaction(tx)

		// Read documents
		docs, err := collection.Find(ctx, query, opts)
		if err != nil {
			fmt.Println("Failed to read documents: %v", err)
			return err
		}

		// Create new query for updating docs
		updatedQuery := NewQuery()

		// Add all found doc IDs
		for _, doc := range docs {
			updatedQuery = updatedQuery.ID(doc.ID)
		}

		// Update found docs
		return collection.Update(ctx, updatedQuery, updates, opts)
	})
	if err != nil {
		// Handle any errors appropriately in this section.
		log.Printf("An error has occurred: %s", err)
	}

	return err
}
```
 
***Transactional Behavior:***
- ID Query Clause: When using a transaction for `Update` or `Delete`, only the `ID` clause in the `Query` is considered. If you need to update or delete documents based on other criteria, use the `Find` method first to retrieve the document IDs, and then pass them to `Update` or `Delete`.
- Atomicity: The operations performed within a transaction are **atomic**. If any error occurs during the transaction, all changes will be rolled back, ensuring that your Firestore data remains consistent.
- Multiple Collections: Transactions support operations across multiple collections within the same transaction, as long as the total number of operations does not exceed Firestore's transaction limits (~500 operations per transaction).

Custom Errors
------------
During collection methods which require validation (i.e. `Create`, `Update` and `Validate`), Firevault may return an error that implements the `FieldError` interface, which can aid in presenting custom error messages to users. All other errors are of the usual `error` type, and do not satisfy the the interface. Available methods for `FieldError` can be found in [field_error.go](https://github.com/bobch27/firevault_go/blob/main/field_error.go). 

Firevault supports the creation of custom, user-friendly, error messages, through `ErrorFormatterFunc`. These are run whenever a `FieldError` is created (i.e. whenever a validation rule fails). All registered formatters are executed on the `FieldError` and if all return a nil error (or there's no registered formatters), a `FieldError` is returned instead. Otherwise, the first custom error is returned.

*Error formatters:*
- To define an error formatter, use `Connection`'s `RegisterErrorFormatter` method.
	- *Expects*:
		- func: A function of type `ErrorFormatterFunc`. The passed in function accepts one parameter.
			- *Expects*:
				- fe: An error that complies with the `FieldError` interface.
			- *Returns*:
				- error: An `error` with a custom message, using `FieldError`'s field validation details.

*Registering custom error formatters is not thread-safe. It is intended that all functions be registered, prior to any validation.*

```go
connection.RegisterErrorFormatter(func(fe FieldError) error {
	if err.Rule() == "min" {
		return fmt.Errorf("%s must be at least %s characters long.", fe.DisplayField(), fe.Param())
	}

	return nil
})
```

This is how it can be used.

```go
id, err := collection.Create(ctx, &User{
	Name: "Bobby Donev",
	Email: "hello@bobbydonev.com",
	Password: "12345",
	Age: 26,
	Address: &Address{
		Line1: "1 High Street",
		City:  "London",
	},
})
if err != nil {
	fmt.Println(err) // "Password must be at least 6 characters long."
} else {
	fmt.Println(id)
}
```
```go
id, err := collection.Create(ctx, &User{
	Name: "Bobby Donev",
	Email: "hello@.com", // will fail on the "email" rule
	Password: "123456",
	Age: 25,
	Address: &Address{
		Line1: "1 High Street",
		City:  "London",
	},
})
if err != nil {
	fmt.Println(err) // error isn't catched by formatter; complies with FieldError
} else {
	fmt.Println(id)
}
```

You can also directly handle and parse returned `FieldError`, without registering an error formatter. Here is an example of parsing a returned `FieldError`, in cases where an error formatter doesn't catch it, or in cases where no formatters are registered.

```go
func parseError(fe FieldError) {
	if fe.StructField() == "Password" { // or fe.Field() == "password"
		if fe.Rule() == "min" {
			fmt.Printf("Password must be at least %s characters long.", fe.Param())
		} else {
			fmt.Println(fe.Error())
		}
	} else {
		fmt.Println(fe.Error())
	}
}

id, err := collection.Create(ctx, &User{
	Name: "Bobby Donev",
	Email: "hello@bobbydonev.com",
	Password: "12345",
	Age: 26,
	Address: &Address{
		Line1: "1 High Street",
		City:  "London",
	},
})
if err != nil {
	var fErr FieldError
	if errors.As(err, &fErr) {
		parseError(fErr) // "Password must be at least 6 characters long."
	} else {
		fmt.Println(err.Error())
	}
} else {
	fmt.Println(id)
}
```

Performance
------------
Firevault's built-in validation is designed to be both robust and efficient. Benchmarks indicate that it performs comparably to industry-leading libraries like [go-playground/validator](https://github.com/go-playground/validator), both with and without caching.

For detailed benchmark results, see [BENCHMARKS.md](https://github.com/bobch27/firevault_go/blob/main/BENCHMARKS.md).

Contributing
------------
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

License
------------
[MIT](https://choosealicense.com/licenses/mit/)