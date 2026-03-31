package keel

import transporthttp "github.com/lumm2509/keel/transport/http"

// Validatable is implemented by any type that can validate itself.
// Compatible with github.com/go-ozzo/ozzo-validation/v4 out of the box.
type Validatable = transporthttp.Validatable

// BindAndValidate binds the request body into dst and validates it.
// On failure it returns an ApiError ready to be returned from a handler.
//
// Example:
//
//	var body CreateUserBody
//	if err := keel.BindAndValidate(c, &body); err != nil {
//	    return err
//	}
func BindAndValidate[T Validatable](c transporthttp.BodyBinder, dst T) error {
	return transporthttp.BindAndValidate[T](c, dst)
}
