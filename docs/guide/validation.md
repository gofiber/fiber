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
type Job struct{
    Type          string `validate:"required,min=3,max=32"`
    Salary        int    `validate:"required,number"`
}

type User struct{
    Name          string  `validate:"required,min=3,max=32"`
    // use `*bool` here otherwise the validation will fail for `false` values
    // Ref: https://github.com/go-playground/validator/issues/319#issuecomment-339222389
    IsActive      *bool   `validate:"required"`
    Email         string  `validate:"required,email,min=6,max=32"`
    Job           Job     `validate:"dive"`
}

type ErrorResponse struct {
    FailedField string
    Tag         string
    Value       string
}

var validate = validator.New()
func ValidateStruct(user User) []*ErrorResponse {
    var errors []*ErrorResponse
    err := validate.Struct(user)
    if err != nil {
        for _, err := range err.(validator.ValidationErrors) {
            var element ErrorResponse
            element.FailedField = err.StructNamespace()
            element.Tag = err.Tag()
            element.Value = err.Param()
            errors = append(errors, &element)
        }
    }
    return errors
}

func AddUser(c *fiber.Ctx) error {
    //Connect to database

    user := new(User)

    if err := c.BodyParser(user); err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "message": err.Error(),
        })
       
    }

    errors := ValidateStruct(*user)
    if errors != nil {
       return c.Status(fiber.StatusBadRequest).JSON(errors)
        
    }

    //Do something else here

    //Return user
   return c.JSON(user)
}

// Running a test with the following curl commands
// curl -X POST -H "Content-Type: application/json" --data "{\"name\":\"john\",\"isactive\":\"True\"}" http://localhost:8080/register/user

// Results in
// [{"FailedField":"User.Email","Tag":"required","Value":""},{"FailedField":"User.Job.Salary","Tag":"required","Value":""},{"FailedField":"User.Job.Type","Tag":"required","Value":""}]⏎
```
