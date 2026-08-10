---
id: validation
title: 🔎 Validation
sidebar_position: 5
---

## Validator package

Fiber does not bundle a validation library. Instead, [`Bind`](../api/bind.md#validation) accepts any validator you plug into `fiber.Config.StructValidator` and runs it automatically whenever request data is bound onto a struct. The core stays dependency-free, and validation becomes a one-time setup instead of per-handler boilerplate.

The steps below use [go-playground/validator](https://github.com/go-playground/validator), the most common choice in the Go ecosystem, but any library works as long as you wrap it in the small adapter from step 2.

### Step 1: Install a validator

```bash
go get github.com/go-playground/validator/v10
```

### Step 2: Wire it into the app config

`StructValidator` expects a single `Validate(out any) error` method, so wrap the library in a tiny adapter and register it once:

```go
import "github.com/go-playground/validator/v10"

type structValidator struct {
    validate *validator.Validate
}

// Validator needs to implement the Validate method
func (v *structValidator) Validate(out any) error {
    return v.validate.Struct(out)
}

// Set up your validator in the config
app := fiber.New(fiber.Config{
    StructValidator: &structValidator{validate: validator.New()},
})
```

:::note
`StructValidator` runs only for struct destinations (or pointers to structs). Binding into maps and other non-struct types skips validation.
:::

### Step 3: Bind and validate in one call

Tag your structs with the validator's rules. Every bind method (`Body`, `Query`, `Form`, and the rest) now triggers validation and returns its errors alongside binding errors:

```go
type User struct {
    Name string `json:"name" form:"name" query:"name" validate:"required"`
    Age  int    `json:"age" form:"age" query:"age" validate:"gte=0,lte=100"`
}

app.Post("/", func(c fiber.Ctx) error {
    user := new(User)

    // Works with all bind methods: Body, Query, Form, ...
    if err := c.Bind().Body(user); err != nil { // validation errors are returned here
        return err
    }

    return c.JSON(user)
})
```

Returned as-is, a validation error surfaces through Fiber's [error handling](./error-handling.md), so you can also shape it globally in a custom error handler.

### Step 4: Shape the error response

For field-level feedback, unwrap `validator.ValidationErrors` and answer with a structured body instead of the default text:

```go
type User struct {
    Name     string `json:"name" validate:"required,min=3,max=32"`
    Email    string `json:"email" validate:"required,email"`
    Age      int    `json:"age" validate:"gte=0,lte=100"`
    Password string `json:"password" validate:"required,min=8"`
    Website  string `json:"website" validate:"url"`
}

app.Post("/user", func(c fiber.Ctx) error {
    user := new(User)

    if err := c.Bind().Body(user); err != nil {
        var validationErrors validator.ValidationErrors
        if errors.As(err, &validationErrors) {
            out := make([]fiber.Map, 0, len(validationErrors))
            for _, e := range validationErrors {
                // e.Field() - field name, e.Tag() - failed rule,
                // e.Param() - rule parameter, e.Value() - invalid value
                out = append(out, fiber.Map{
                    "field": e.Field(),
                    "rule":  e.Tag(),
                })
            }
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": out})
        }
        return err
    }

    return c.JSON(user)
})
```

### Step 5: Add your own rules

Because the adapter is yours, you can layer custom checks on top of the tag rules:

```go
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
