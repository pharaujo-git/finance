package application

import (
	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// Field names double as the ModelState keys of a validation 400. They are the
// PascalCase property names of the .NET request DTOs, not the camelCase JSON
// names, because that is what the .NET API emits.
const (
	FieldEmail    = "Email"
	FieldPassword = "Password"
	FieldName     = "Name"
	FieldCurrency = "Currency"
)

// Limits mirror the DataAnnotations attributes on FinanceTracker's AuthDtos and
// the column widths in db/migrations/0001_baseline.sql.
const (
	emailMaxLength    = 256
	passwordMinLength = 8
	passwordMaxLength = 128
	nameMaxLength     = 200
	currencyMaxLength = 8
)

// UserDto is the profile shape both APIs return. The JSON names are camelCase
// because the .NET API applies JsonNamingPolicy.CamelCase to responses.
type UserDto struct {
	ID       uuid.UUID `json:"id"`
	Email    string    `json:"email"`
	Name     string    `json:"name"`
	Currency string    `json:"currency"`
}

// NewUserDto projects a user onto the wire shape.
func NewUserDto(user *domain.User) UserDto {
	return UserDto{
		ID:       user.Id,
		Email:    user.Email,
		Name:     user.Name,
		Currency: user.Currency,
	}
}

// AuthResponse is what register and login return.
type AuthResponse struct {
	Token string  `json:"token"`
	User  UserDto `json:"user"`
}

// RegisterRequest is the body of POST /api/auth/register.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// Validate mirrors the attributes on FinanceTracker's RegisterRequest:
// [Required][EmailAddress][MaxLength(256)] on Email, [Required][MinLength(8)]
// [MaxLength(128)] on Password, [Required][MaxLength(200)] on Name. Every rule
// runs, so one field can report several messages, exactly as MVC does.
func (r RegisterRequest) Validate() error {
	errs := domain.NewValidationError()

	required(errs, FieldEmail, r.Email)
	emailAddress(errs, FieldEmail, r.Email)
	maxLength(errs, FieldEmail, r.Email, emailMaxLength)

	required(errs, FieldPassword, r.Password)
	minLength(errs, FieldPassword, r.Password, passwordMinLength)
	maxLength(errs, FieldPassword, r.Password, passwordMaxLength)

	required(errs, FieldName, r.Name)
	maxLength(errs, FieldName, r.Name, nameMaxLength)

	return errs.OrNil()
}

// LoginRequest is the body of POST /api/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Validate mirrors FinanceTracker's LoginRequest. Note the missing
// [EmailAddress]: sign-in accepts anything shaped like a string, so a wrong
// address fails as bad credentials rather than as a validation error.
func (r LoginRequest) Validate() error {
	errs := domain.NewValidationError()

	required(errs, FieldEmail, r.Email)
	maxLength(errs, FieldEmail, r.Email, emailMaxLength)

	required(errs, FieldPassword, r.Password)
	maxLength(errs, FieldPassword, r.Password, passwordMaxLength)

	return errs.OrNil()
}

// UpdateProfileRequest is the body of PUT /api/auth/me.
type UpdateProfileRequest struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
}

// Validate mirrors FinanceTracker's UpdateProfileRequest.
func (r UpdateProfileRequest) Validate() error {
	errs := domain.NewValidationError()

	required(errs, FieldName, r.Name)
	maxLength(errs, FieldName, r.Name, nameMaxLength)

	required(errs, FieldCurrency, r.Currency)
	maxLength(errs, FieldCurrency, r.Currency, currencyMaxLength)

	return errs.OrNil()
}
