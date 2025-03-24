---
id: validation
title: 🔎 Validation
sidebar_position: 5
---

## Validator package

Fiber provides the [Bind](../api/bind.md#validation) function to validate and bind [request data](../api/bind.md#binders) to a struct.

```go title="Basic Example"
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

```go title="Advanced Validation Example"
type User struct {
    Name     string `json:"name" validate:"required,min=3,max=32"`
    Email    string `json:"email" validate:"required,email"`
    Age      int    `json:"age" validate:"gte=0,lte=100"`
    Password string `json:"password" validate:"required,min=8"`
    Website  string `json:"website" validate:"url"`
}

// Custom validation error messages
type UserWithCustomMessages struct {
    Name     string `json:"name" validate:"required,min=3,max=32" message:"Name is required and must be between 3 and 32 characters"`
    Email    string `json:"email" validate:"required,email" message:"Valid email is required"`
    Age      int    `json:"age" validate:"gte=0,lte=100" message:"Age must be between 0 and 100"`
}

app.Post("/user", func(c fiber.Ctx) error {
    user := new(User)
    
    if err := c.Bind().Body(user); err != nil {
        // Handle validation errors
        if validationErrors, ok := err.(validator.ValidationErrors); ok {
            for _, e := range validationErrors {
                // e.Field() - field name
                // e.Tag() - validation tag
                // e.Value() - invalid value
                // e.Param() - validation parameter
                return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
                    "field": e.Field(),
                    "error": e.Error(),
                })
            }
        }
        return err
    }
    
    return c.JSON(user)
})
```

```go title="Custom Validator Example"
// Custom validator for password strength
type PasswordValidator struct {
    validate *validator.Validate
}

func (v *PasswordValidator) Validate(out any) error {
    if err := v.validate.Struct(out); err != nil {
        return err
    }
    
    // Custom password validation logic
    if user, ok := out.(*User); ok {
        if len(user.Password) < 8 {
            return errors.New("password must be at least 8 characters")
        }
        // Add more password validation rules here
    }
    
    return nil
}

// Usage
app := fiber.New(fiber.Config{
    StructValidator: &PasswordValidator{validate: validator.New()},
})
```
