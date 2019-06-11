Package validator
================
<img align="right" src="https://raw.githubusercontent.com/go-playground/validator/v8/logo.png">[![Join the chat at https://gitter.im/bluesuncorp/validator](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/go-playground/validator?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
![Project status](https://img.shields.io/badge/version-8.18.2-green.svg)
[![Build Status](https://semaphoreci.com/api/v1/projects/ec20115f-ef1b-4c7d-9393-cc76aba74eb4/530054/badge.svg)](https://semaphoreci.com/joeybloggs/validator)
[![Coverage Status](https://coveralls.io/repos/go-playground/validator/badge.svg?branch=v8&service=github)](https://coveralls.io/github/go-playground/validator?branch=v8)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-playground/validator)](https://goreportcard.com/report/github.com/go-playground/validator)
[![GoDoc](https://godoc.org/gopkg.in/go-playground/validator.v8?status.svg)](https://godoc.org/gopkg.in/go-playground/validator.v8)
![License](https://img.shields.io/dub/l/vibe-d.svg)

Package validator implements value validations for structs and individual fields based on tags.

It has the following **unique** features:

-   Cross Field and Cross Struct validations by using validation tags or custom validators.  
-   Slice, Array and Map diving, which allows any or all levels of a multidimensional field to be validated.  
-   Handles type interface by determining it's underlying type prior to validation.
-   Handles custom field types such as sql driver Valuer see [Valuer](https://golang.org/src/database/sql/driver/types.go?s=1210:1293#L29)
-   Alias validation tags, which allows for mapping of several validations to a single tag for easier defining of validations on structs
-   Extraction of custom defined Field Name e.g. can specify to extract the JSON name while validating and have it available in the resulting FieldError

Installation
------------

Use go get.

	go get gopkg.in/go-playground/validator.v8

or to update

	go get -u gopkg.in/go-playground/validator.v8

Then import the validator package into your own code.

	import "gopkg.in/go-playground/validator.v8"

Error Return Value
-------

Validation functions return type error

They return type error to avoid the issue discussed in the following, where err is always != nil:

* http://stackoverflow.com/a/29138676/3158232
* https://github.com/go-playground/validator/issues/134

validator only returns nil or ValidationErrors as type error; so in you code all you need to do
is check if the error returned is not nil, and if it's not type cast it to type ValidationErrors
like so:

```go
err := validate.Struct(mystruct)
validationErrors := err.(validator.ValidationErrors)
 ```

Usage and documentation
------

Please see http://godoc.org/gopkg.in/go-playground/validator.v8 for detailed usage docs.

##### Examples:

Struct & Field validation
```go
package main

import (
	"fmt"

	"gopkg.in/go-playground/validator.v8"
)

// User contains user information
type User struct {
	FirstName      string     `validate:"required"`
	LastName       string     `validate:"required"`
	Age            uint8      `validate:"gte=0,lte=130"`
	Email          string     `validate:"required,email"`
	FavouriteColor string     `validate:"hexcolor|rgb|rgba"`
	Addresses      []*Address `validate:"required,dive,required"` // a person can have a home and cottage...
}

// Address houses a users address information
type Address struct {
	Street string `validate:"required"`
	City   string `validate:"required"`
	Planet string `validate:"required"`
	Phone  string `validate:"required"`
}

var validate *validator.Validate

func main() {

	config := &validator.Config{TagName: "validate"}

	validate = validator.New(config)

	validateStruct()
	validateField()
}

func validateStruct() {

	address := &Address{
		Street: "Eavesdown Docks",
		Planet: "Persphone",
		Phone:  "none",
	}

	user := &User{
		FirstName:      "Badger",
		LastName:       "Smith",
		Age:            135,
		Email:          "Badger.Smith@gmail.com",
		FavouriteColor: "#000",
		Addresses:      []*Address{address},
	}

	// returns nil or ValidationErrors ( map[string]*FieldError )
	errs := validate.Struct(user)

	if errs != nil {

		fmt.Println(errs) // output: Key: "User.Age" Error:Field validation for "Age" failed on the "lte" tag
		//	                         Key: "User.Addresses[0].City" Error:Field validation for "City" failed on the "required" tag
		err := errs.(validator.ValidationErrors)["User.Addresses[0].City"]
		fmt.Println(err.Field) // output: City
		fmt.Println(err.Tag)   // output: required
		fmt.Println(err.Kind)  // output: string
		fmt.Println(err.Type)  // output: string
		fmt.Println(err.Param) // output:
		fmt.Println(err.Value) // output:

		// from here you can create your own error messages in whatever language you wish
		return
	}

	// save user to database
}

func validateField() {
	myEmail := "joeybloggs.gmail.com"

	errs := validate.Field(myEmail, "required,email")

	if errs != nil {
		fmt.Println(errs) // output: Key: "" Error:Field validation for "" failed on the "email" tag
		return
	}

	// email ok, move on
}
```

Custom Field Type
```go
package main

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"

	"gopkg.in/go-playground/validator.v8"
)

// DbBackedUser User struct
type DbBackedUser struct {
	Name sql.NullString `validate:"required"`
	Age  sql.NullInt64  `validate:"required"`
}

func main() {

	config := &validator.Config{TagName: "validate"}

	validate := validator.New(config)

	// register all sql.Null* types to use the ValidateValuer CustomTypeFunc
	validate.RegisterCustomTypeFunc(ValidateValuer, sql.NullString{}, sql.NullInt64{}, sql.NullBool{}, sql.NullFloat64{})

	x := DbBackedUser{Name: sql.NullString{String: "", Valid: true}, Age: sql.NullInt64{Int64: 0, Valid: false}}
	errs := validate.Struct(x)

	if len(errs.(validator.ValidationErrors)) > 0 {
		fmt.Printf("Errs:\n%+v\n", errs)
	}
}

// ValidateValuer implements validator.CustomTypeFunc
func ValidateValuer(field reflect.Value) interface{} {
	if valuer, ok := field.Interface().(driver.Valuer); ok {
		val, err := valuer.Value()
		if err == nil {
			return val
		}
		// handle the error how you want
	}
	return nil
}
```

Struct Level Validation
```go
package main

import (
	"fmt"
	"reflect"

	"gopkg.in/go-playground/validator.v8"
)

// User contains user information
type User struct {
	FirstName      string     `json:"fname"`
	LastName       string     `json:"lname"`
	Age            uint8      `validate:"gte=0,lte=130"`
	Email          string     `validate:"required,email"`
	FavouriteColor string     `validate:"hexcolor|rgb|rgba"`
	Addresses      []*Address `validate:"required,dive,required"` // a person can have a home and cottage...
}

// Address houses a users address information
type Address struct {
	Street string `validate:"required"`
	City   string `validate:"required"`
	Planet string `validate:"required"`
	Phone  string `validate:"required"`
}

var validate *validator.Validate

func main() {

	config := &validator.Config{TagName: "validate"}

	validate = validator.New(config)
	validate.RegisterStructValidation(UserStructLevelValidation, User{})

	validateStruct()
}

// UserStructLevelValidation contains custom struct level validations that don't always
// make sense at the field validation level. For Example this function validates that either
// FirstName or LastName exist; could have done that with a custom field validation but then
// would have had to add it to both fields duplicating the logic + overhead, this way it's
// only validated once.
//
// NOTE: you may ask why wouldn't I just do this outside of validator, because doing this way
// hooks right into validator and you can combine with validation tags and still have a
// common error output format.
func UserStructLevelValidation(v *validator.Validate, structLevel *validator.StructLevel) {

	user := structLevel.CurrentStruct.Interface().(User)

	if len(user.FirstName) == 0 && len(user.LastName) == 0 {
		structLevel.ReportError(reflect.ValueOf(user.FirstName), "FirstName", "fname", "fnameorlname")
		structLevel.ReportError(reflect.ValueOf(user.LastName), "LastName", "lname", "fnameorlname")
	}

	// plus can to more, even with different tag than "fnameorlname"
}

func validateStruct() {

	address := &Address{
		Street: "Eavesdown Docks",
		Planet: "Persphone",
		Phone:  "none",
		City:   "Unknown",
	}

	user := &User{
		FirstName:      "",
		LastName:       "",
		Age:            45,
		Email:          "Badger.Smith@gmail.com",
		FavouriteColor: "#000",
		Addresses:      []*Address{address},
	}

	// returns nil or ValidationErrors ( map[string]*FieldError )
	errs := validate.Struct(user)

	if errs != nil {

		fmt.Println(errs) // output: Key: 'User.LastName' Error:Field validation for 'LastName' failed on the 'fnameorlname' tag
		//	                         Key: 'User.FirstName' Error:Field validation for 'FirstName' failed on the 'fnameorlname' tag
		err := errs.(validator.ValidationErrors)["User.FirstName"]
		fmt.Println(err.Field) // output: FirstName
		fmt.Println(err.Tag)   // output: fnameorlname
		fmt.Println(err.Kind)  // output: string
		fmt.Println(err.Type)  // output: string
		fmt.Println(err.Param) // output:
		fmt.Println(err.Value) // output:

		// from here you can create your own error messages in whatever language you wish
		return
	}

	// save user to database
}
```

Benchmarks
------
###### Run on MacBook Pro (Retina, 15-inch, Late 2013) 2.6 GHz Intel Core i7 16 GB 1600 MHz DDR3 using Go version go1.5.3 darwin/amd64
```go
PASS
BenchmarkFieldSuccess-8                            	20000000	       118 ns/op	       0 B/op	       0 allocs/op
BenchmarkFieldFailure-8                            	 2000000	       758 ns/op	     432 B/op	       4 allocs/op
BenchmarkFieldDiveSuccess-8                        	  500000	      2471 ns/op	     464 B/op	      28 allocs/op
BenchmarkFieldDiveFailure-8                        	  500000	      3172 ns/op	     896 B/op	      32 allocs/op
BenchmarkFieldCustomTypeSuccess-8                  	 5000000	       300 ns/op	      32 B/op	       2 allocs/op
BenchmarkFieldCustomTypeFailure-8                  	 2000000	       775 ns/op	     432 B/op	       4 allocs/op
BenchmarkFieldOrTagSuccess-8                       	 1000000	      1122 ns/op	       4 B/op	       1 allocs/op
BenchmarkFieldOrTagFailure-8                       	 1000000	      1167 ns/op	     448 B/op	       6 allocs/op
BenchmarkStructLevelValidationSuccess-8            	 3000000	       548 ns/op	     160 B/op	       5 allocs/op
BenchmarkStructLevelValidationFailure-8            	 3000000	       558 ns/op	     160 B/op	       5 allocs/op
BenchmarkStructSimpleCustomTypeSuccess-8           	 2000000	       623 ns/op	      36 B/op	       3 allocs/op
BenchmarkStructSimpleCustomTypeFailure-8           	 1000000	      1381 ns/op	     640 B/op	       9 allocs/op
BenchmarkStructPartialSuccess-8                    	 1000000	      1036 ns/op	     272 B/op	       9 allocs/op
BenchmarkStructPartialFailure-8                    	 1000000	      1734 ns/op	     730 B/op	      14 allocs/op
BenchmarkStructExceptSuccess-8                     	 2000000	       888 ns/op	     250 B/op	       7 allocs/op
BenchmarkStructExceptFailure-8                     	 1000000	      1036 ns/op	     272 B/op	       9 allocs/op
BenchmarkStructSimpleCrossFieldSuccess-8           	 2000000	       773 ns/op	      80 B/op	       4 allocs/op
BenchmarkStructSimpleCrossFieldFailure-8           	 1000000	      1487 ns/op	     536 B/op	       9 allocs/op
BenchmarkStructSimpleCrossStructCrossFieldSuccess-8	 1000000	      1261 ns/op	     112 B/op	       7 allocs/op
BenchmarkStructSimpleCrossStructCrossFieldFailure-8	 1000000	      2055 ns/op	     576 B/op	      12 allocs/op
BenchmarkStructSimpleSuccess-8                     	 3000000	       519 ns/op	       4 B/op	       1 allocs/op
BenchmarkStructSimpleFailure-8                     	 1000000	      1429 ns/op	     640 B/op	       9 allocs/op
BenchmarkStructSimpleSuccessParallel-8             	10000000	       146 ns/op	       4 B/op	       1 allocs/op
BenchmarkStructSimpleFailureParallel-8             	 2000000	       551 ns/op	     640 B/op	       9 allocs/op
BenchmarkStructComplexSuccess-8                    	  500000	      3269 ns/op	     244 B/op	      15 allocs/op
BenchmarkStructComplexFailure-8                    	  200000	      8436 ns/op	    3609 B/op	      60 allocs/op
BenchmarkStructComplexSuccessParallel-8            	 1000000	      1024 ns/op	     244 B/op	      15 allocs/op
BenchmarkStructComplexFailureParallel-8            	  500000	      3536 ns/op	    3609 B/op	      60 allocs/op
```

Complimentary Software
----------------------

Here is a list of software that compliments using this library either pre or post validation.

* [form](https://github.com/go-playground/form) - Decodes url.Values into Go value(s) and Encodes Go value(s) into url.Values. Dual Array and Full map support.
* [Conform](https://github.com/leebenson/conform) - Trims, sanitizes & scrubs data based on struct tags.

How to Contribute
------

There will always be a development branch for each version i.e. `v1-development`. In order to contribute, 
please make your pull requests against those branches.

If the changes being proposed or requested are breaking changes, please create an issue, for discussion
or create a pull request against the highest development branch for example this package has a
v1 and v1-development branch however, there will also be a v2-development branch even though v2 doesn't exist yet.

I strongly encourage everyone whom creates a custom validation function to contribute them and
help make this package even better.

License
------
Distributed under MIT License, please see license file in code for more details.
