---
id: validation
title: 🔎 Validation
sidebar_position: 5
---

## Validator package

Fiber provides the [*Bind*](../api/bind.md#validation) function to validate and bind [request data](../api/bind.md#binders) to a struct.

```go title="Example"

import "github.com/go-playground/validator/v10"

type structValidator struct {
    validate *validator.Validate
}

// Validator needs to implement the Validate method
func (v *structValidator) Validate(out any) error {
    return v.validate.Struct(out)
}

// Setup your validator in the config
app := fiber.New(fiber.Config{
    StructValidator: &structValidator{validate: validator.New()},
})

type User struct {
  Name string `json:"name" form:"name" query:"name" validate:"required"`
  Age  int    `json:"age" form:"age" query:"age" validate:"gte=0,lte=100"`
}

app.Post("/", func(c fiber.Ctx) error {
  user := new(User)
  
  // Works with all bind methods - Body, Query, Form, ...
  if err := c.Bind().Body(user); err != nil { // <- here you receive the validation errors
    return err
  }
  
  return c.JSON(user)
})
```
