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
You can connect to Firevault using the `Connect` method, providing a project ID.

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
When defining a new struct type with Firevault tags, note that the tags' order matters (apart from the different `omitempty` tags, which can be used anywhere). 

The first tag is always the **field name** which will be used in Firestore. You can skip that by just using a comma, before adding further tags.

After that, each tag is a different validation rule, and they will be parsed in order.

Other than the validation tags, Firevault supports the following built-in tags:
- `omitempty` - If the field is set to it’s default value (e.g. `0` for `int`, or `""` for `string`), the field will be omitted from validation and Firestore.
- `omitempty_create` - Works the same way as `omitempty`, but only for the `Create` method. Ignored during `Update` and `Validate` methods.
- `omitempty_update` - Works the same way as `omitempty`, but only for the `Update` method. Ignored during `Create` and `Validate` methods.
- `omitempty_validate` - Works the same way as `omitempty`, but only for the `Validate` method. Ignored during `Create` and `Update` methods.
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
		- name: A `string` defining the validation name
		- func: A function of type `ValidationFn`. The passed in function accepts four parameters.
			- *Expects*:
				- ctx: A context.
				- path: A `string` which contains the field's path (using dot-separation).
				- value: A `reflect.Value` of the field.
				- param: A `string` which will be validated against.
			- *Returns*:
				- result: A `bool` which returns `true` if check has passed, and `false` if it hasn't.
				- error: An `error` in case something went wrong during the check.

*Registering custom validations is not thread-safe. It is intended that all rules be registered, prior to any validation. Also, if a rule with the same name already exists, the previous one will be replaced.*

```go
connection.RegisterValidation(
	"is_upper", 
	func(_ context.Context, _ string, value reflect.Value, _ string) (bool, error) {
		if value.Kind() != reflect.String {
			return false, nil
		}

		s := value.String()
		return s == strings.toUpper(s), nil
	},
)
```

You can then chain the tag like a normal one.

```go
type User struct {
	Name string `firevault:"name,required,is_upper,omitempty"`
}
```

Transformations
------------
Firevault also supports rules that transform the field's value. To use them, it's as simple as registering a transformation and adding a prefix to the tag.

- To define a transformation, use `Connection`'s `RegisterTransformation` method.
	- *Expects*:
		- name: A `string` defining the validation name
		- func: A function of type `TransformationFn`. The passed in function accepts three parameters.
			- *Expects*: 
				- ctx: A context.
				- path: A `string` which contains the field's path (using dot-separation).
				- value: A `reflect.Value` of the field.
			- *Returns*:
				- result: An `interface{}` with the new value.
				- error: An `error` in case something went wrong during the transformation.

*Registering custom transformations is not thread-safe. It is intended that all rules be registered, prior to any validation. Also, if a rule with the same name already exists, the previous one will be replaced.*

```go
connection.RegisterTransformation(
	"to_lower", 
	func(_ context.Context, path string, value reflect.Value) (interface{}, error) {
		if value.Kind() != reflect.String {
			return value.Interface(), errors.New(path + " must be a string")
		}

		if value.String() != "" {
			return strings.ToLower(value.String()), nil
		}

		return value.String(), nil
	},
)
```

You can then chain the tag like a normal one, but don't forget to use the `transform=` prefix.

*Again, the tag order matters. Defining a transformation at the end, means the value will be updated **after** the validations, whereas a definition at the start, means the field will be updated and **then** validated.*

```go
type User struct {
	Email string `firevault:"email,required,email,transform=to_lower,omitempty"`
}
```

Collections
------------
A Firevault `CollectionRef` instance allows for interacting with Firestore, through various read and write methods.

To create a `CollectionRef` instance, call the `Collection` function, using the struct type parameter, and passing in the `Connection` instance, as well as a collection **path**.

```go
collection := firevault.Collection[User](connection, "users")
```

Methods
------------
The `CollectionRef` instance has **7** built-in methods to support interaction with Firestore.

- `Create` - A method which validates passed in data and adds it as a document to Firestore. 
	- *Expects*:
		- ctx: A context.
		- data: A `pointer` of a `struct` with populated fields which will be added to Firestore after validation.
		- options *(optional)*: An instance of `Options` with the following properties having an
		effect. 
			- SkipValidation: A `bool` which when `true`, means all validation tags will be ingored (the `name` and `omitempty` tags will be acknowledged). Default is `false`.
			- ID: A `string` which will add a document to Firestore with the specified ID.
			- AllowEmptyFields: An optional `string` `slice`, which is used to specify which fields can ignore the `omitempty` and `omitempty_create` tags. This can be useful when a field must be set to its zero value only on certain method calls. If left empty, all fields will honour the two tags.
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
- `Update` - A method which validates passed in data and updates all Firestore documents which match provided `Query`. If `Query` contains an `ID` clause and no documents are matched, new ones will be created for each provided ID. The method uses Firestore's `BulkWriter` under the hood, meaning the operation is not atomic.
	- *Expects*:
		- ctx: A context.
		- query: A `Query` instance to filter which documents to update.
		- data: A `pointer` of a `struct` with populated fields which will be used to update the documents after validation.
		- options *(optional)*: An instance of `Options` with the following properties having an
		effect.
			- SkipValidation: A `bool` which when `true`, means all validation tags will be ingored (the `name` and `omitempty` tags will be acknowledged). Default is `false`.
			- MergeFields: An optional `string` `slice`, which is used to specify which fields to be overwritten. Other fields on the document will be untouched. If left empty, all the fields given in the data argument will be overwritten. If a field is specified, but is not present in the data passed, the field will be deleted from the document (using `firestore.Delete`).
			- AllowEmptyFields: An optional `string` `slice`, which is used to specify which fields can ignore the `omitempty` and `omitempty_update` tags. This can be useful when a field must be set to its zero value only on certain updates. If left empty, all fields will honour the two tags.
	- *Returns*:
		- error: An `error` in case something goes wrong during validation or interaction with Firestore.
	- ***Important***: 
		- If neither `omitempty`, nor `omitempty_update` tags have been used, non-specified field values in the passed in data will be set to Go's default values, thus updating all document fields. To prevent that behaviour, please use one of the two tags. 
		- If no documents match the provided `Query`, the operation will do nothing and will not return an error.
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
	Password: "123567",
}
err := collection.Update(
	ctx, 
	NewQuery().ID("6QVHL46WCE680ZG2Xn3X"), 
	&user, 
	NewOptions().SkipValidation(),
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
	NewOptions().MergeFields("address.Line1"),
)
if err != nil {
	fmt.Println(err)
} 
fmt.Println("Success") // only the address.Line1 field will be updated
```
```go
user := User{
	Address:  &Address{
		City:  "New York",
	}
}
err := collection.Update(
	ctx, 
	NewQuery().ID("6QVHL46WCE680ZG2Xn3X"), 
	&user, 
	NewOptions().MergeFields("address.Line1"),
)
if err != nil {
	fmt.Println(err)
} 
fmt.Println("Success") // address.Line1 field will be deleted from document, since it's not present in data
```
- `Validate` - A method which validates and transforms passed in data. 
	- *Expects*:
		- ctx: A context.
		- data: A `pointer` of a `struct` with populated fields which will be validated.
		- options *(optional)*: An instance of `Options` with the following properties having an
		effect.
			- SkipValidation: A `bool` which when `true`, means all validation tags will be ingored (the `name` and `omitempty` tags will be acknowledged). Default is `false`.
			- AllowEmptyFields: An optional `string` `slice`, which is used to specify which fields can ignore the `omitempty` and `omitempty_validate` tags. This can be useful when a field must be set to its zero value only on certain method calls. If left empty, all fields will honour the two tags.
	- *Returns*:
		- error: An `error` in case something goes wrong during validation.
	- ***Important***: 
		- If neither `omitempty`, nor `omitempty_validate` tags have been used, non-specified field values in the passed in data will be set to Go's default values. 
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
- `Delete` - A method which deletes all Firestore documents which match provided `Query`. The method uses Firestore's `BulkWriter` under the hood, meaning the operation is not atomic. 
	- *Expects*:
		- ctx: A context.
		- query: A `Query` instance to filter which documents to delete.
	- *Returns*:
		- error: An `error` in case something goes wrong during interaction with Firestore.
	- If no documents match the provided `Query`, the method does nothing and `error` is `nil`.
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
- `Find` - A method which gets the Firestore documents which match the provided query.
	- *Expects*:
		- ctx: A context.
		- query: A `Query` to filter and order documents.
	- *Returns*: 
		- docs: A `slice` containing the results of type `Document[T]` (where `T` is the type used when initiating the collection instance). `Document[T]` has two properties.
			- ID: A `string` which holds the document's ID.
			- Data: The document's data of type `T`.
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
- `FindOne` - A method which gets the first Firestore document which matches the provided `Query`. Returns an empty Document[T] (empty ID string and zero-value T Data), and no error if no documents are found.
	- *Expects*:
		- ctx: A context.
		- query: A `Query` to filter and order documents.
	- *Returns*:
		- doc: Returns the document with type `Document[T]` (where `T` is the type used when initiating the collection instance). `Document[T]` has two properties.
			- ID: A `string` which holds the document's ID.
			- Data: The document's data of type `T`.
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
- `Count` - A method which gets the number of Firestore documents which match the provided query.
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

Methods
------------
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
		- value: An `interface{}` used to filter out the results.
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

Methods
------------
The `Options` instance has **4** built-in methods to support overriding default `CollectionRef` method options.

- `SkipValidation` - Returns a new `Options` instance that allows to skip the data validation during creation, updating and validation methods. The "name" tag, "omitempty" tags and "ignore" tag will still be honoured.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.SkipValidation()
```
- `AllowEmptyFields` - Returns a new `Options` instance that allows to specify which field paths should ignore the "omitempty" tags. This can be useful when zero values are needed only during a specific method call. If left empty, those tags will be honoured for all fields.
	- *Expects*:
		- path: A varying number of `string` values (using dot separation) used to select field paths.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.AllowEmptyFields("age")
```
- `MergeFields` - Returns a new `Options` instance that allows to specify which field paths to be overwritten. Other fields on the existing document will be untouched. If a provided field path does not refer to a value in the data passed, that field will be deleted from the document. Only used for updating method.
	- *Expects*:
		- path: A varying number of `string` values (using dot separation) used to select field paths.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.MergeFields("address.Line1")
```
- `CustomID` - Returns a new `Options` instance that allows to specify a custom document ID to be used when creating a Firestore document. Only used for creation method.
	- *Expects*:
		- id: A `string` specifying the custom ID.
	- *Returns*:
		- A new `Options` instance.
```go
newOptions := options.CustomID("custom-id")
```

Custom Errors
------------
During collection methods which require validation (i.e. `Create`, `Update` and `Validate`), Firevault may return an error of a `FieldError` interface, which can aid in presenting custom error messages to users. All other errors are of the usual `error` type. Available methods for `FieldError` can be found in the `field_error.go` file. 

Firevault supports the creation of custom, user-friendly, error messages, through `ErrorFormatterFn`. These are run whenever a `FieldError` is created (i.e. whenever a validation rule fails). All registered formatters are executed on the `FieldError` and if all return a nil error (or there's no registered formatters), a `FieldError` is returned instead. Otherwise, the first custom error is returned.

*Error formatters:*
- To define an error formatter, use `Connection`'s `RegisterErrorFormatter` method.
	- *Expects*:
		- func: A function of type `ErrorFormatterFn`. The passed in function accepts one parameter.
			- *Expects*:
				- fe: An error that complies with the `FieldError` interface.
			- *Returns*:
				- error: An `error` with a custom message, using `FieldError`'s field validation details.

*Registering custom error formatters is not thread-safe. It is intended that all functions be registered, prior to any validation.*

```go
connection.RegisterErrorFormatter(func(fe FieldError) error {
	if err.Tag() == "min" {
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
	Email: "hello@.com", // will fail on the "email" tag
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
		if fe.Tag() == "min" {
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

Contributing
------------
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

License
------------
[MIT](https://choosealicense.com/licenses/mit/)