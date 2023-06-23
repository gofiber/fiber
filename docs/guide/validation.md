---
id: validation
title: 🔎 Validation
sidebar_position: 5
---

## Validator package

Fiber can make _great_ use of the validator package to ensure correct validation of data to store.

* [Official validator Github page \(Installation, use, examples..\).](https://github.com/go-playground/validator)

You can find the detailed descriptions of the _validations_ used in the fields contained on the structs below:

* [Detailed docs](https://pkg.go.dev/github.com/go-playground/validator?tab=doc)

```go title="Validation Example"
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
)

type (
    User struct{
        Name          string `validate:"required,min=5,max=20"` // Required field, min 5 char long max 20
        Age           int    `validate:"required,teener"` // Required field, and client needs to implement our 'teener' tag format which we'll see later
    }

    ErrorResponse struct {
        Error       bool
        FailedField string
        Tag         string
        Value       interface{}
    }

    XValidator struct {
        validator *validator.Validate
    }

	GlobalErrorHandlerResp struct {
		Success bool 	`json:"success"`
		Message string 	`json:"message"`
	}
)

// This is the validator instance
// for more information see: https://github.com/go-playground/validator
var validate = validator.New()

func(v XValidator) Validate(data interface{}) []ErrorResponse{
	Error := []ErrorResponse{}

	err := validate.Struct(data); if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			// In this case data object is actually holding the User struct 
			var elem ErrorResponse

			elem.FailedField = err.Field() // Export struct field name
			elem.Tag = err.Tag() // Export struct tag
			elem.Value = err.Value() // Export field value, convert it to string
			elem.Error = true // When ErrorResponse.Error is true it means there was an error

			Error = append(Error, elem)	
		}
	}

	return Error
}

func main() {
	Validator := &XValidator{
		validator: validate,
	}
    app := fiber.New(fiber.Config{
		// Global custom error handler
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusBadRequest).JSON(GlobalErrorHandlerResp{
				Success: false,
				Message: err.Error(),
			})
		},
	})


	// Custom struct validation tag format
	Validator.validator.RegisterValidation("teener",func(fl validator.FieldLevel) bool {
		// User.Age needs to fit our needs, 12-18 years old.
		return fl.Field().Int() >= 12 && fl.Field().Int() <= 18
	})

    app.Get("/", func (c *fiber.Ctx) error {
		user := &User{
			Name: c.Query("name"),
			Age: c.QueryInt("age"),
		}
		
		// Validation
		if err := Validator.Validate(user); len(err) > 0 && err[0].Error {
			errMsg := make([]string,0)
			
			for _, v := range err {
				errMsg = append(errMsg, fmt.Sprintf(
					"[%s]: '%v' | Needs to implement '%s'",
					v.FailedField,
					v.Value,
					v.Tag,
				))
			}

			return &fiber.Error{
				Code: fiber.ErrBadRequest.Code,
				Message: strings.Join(errMsg, " and "),
			}
		}

		// Logic, validated with success
        return c.SendString("Hello, World!")
    })

    log.Fatal(app.Listen(":3000"))
}


/**
OUTPUT

[1]
	Request:

	GET http://127.0.0.1:3000/

	Response:

	{"success":false,"message":"[Name]: '' | Needs to implement 'required' and [Age]: '0' | Needs to implement 'required'"}

[2]
	Request:

	GET http://127.0.0.1:3000/?name=efdal&age=9

	Response:
	{"success":false,"message":"[Age]: '9' | Needs to implement 'teener'"}

[3]
	Request:

	GET http://127.0.0.1:3000/?name=efdal&age=

	Response:
	{"success":false,"message":"[Age]: '0' | Needs to implement 'required'"}

[4]
	Request:

	GET http://127.0.0.1:3000/?name=efdal&age=18

	Response:
	Hello, World!

**/

```
