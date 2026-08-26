package application

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// Message templates copied from the System.ComponentModel.DataAnnotations
// resources, confirmed against bodies emitted by the running .NET API. Note
// that Required and EmailAddress name the field first while MinLength and
// MaxLength put "The field" first; that asymmetry is theirs, not a typo.
const (
	requiredMessage     = "The %s field is required."
	emailAddressMessage = "The %s field is not a valid e-mail address."
	minLengthMessage    = "The field %s must be a string or array type with a minimum length of '%d'."
	maxLengthMessage    = "The field %s must be a string or array type with a maximum length of '%d'."
)

// required mirrors RequiredAttribute with AllowEmptyStrings left at false: a
// string is missing when it is empty or entirely whitespace.
func required(errs *domain.ValidationError, field, value string) {
	if strings.TrimSpace(value) == "" {
		errs.Add(field, fmt.Sprintf(requiredMessage, field))
	}
}

// emailAddress mirrors EmailAddressAttribute, whose whole check is: exactly one
// '@', neither first nor last. It is applied to the raw value, before the
// service trims and lowercases it, because MVC validates the bound model.
func emailAddress(errs *domain.ValidationError, field, value string) {
	first := strings.IndexByte(value, '@')
	if first <= 0 || first == len(value)-1 || first != strings.LastIndexByte(value, '@') {
		errs.Add(field, fmt.Sprintf(emailAddressMessage, field))
	}
}

// minLength mirrors MinLengthAttribute. Lengths are counted in runes to match
// .NET's string.Length semantics closely enough for the ASCII inputs these
// fields hold; a UTF-16 surrogate pair would count as one here and two there.
func minLength(errs *domain.ValidationError, field, value string, limit int) {
	if utf8.RuneCountInString(value) < limit {
		errs.Add(field, fmt.Sprintf(minLengthMessage, field, limit))
	}
}

// maxLength mirrors MaxLengthAttribute.
func maxLength(errs *domain.ValidationError, field, value string, limit int) {
	if utf8.RuneCountInString(value) > limit {
		errs.Add(field, fmt.Sprintf(maxLengthMessage, field, limit))
	}
}

// The money bounds every [Range(typeof(decimal), ...)] in FinanceTracker's DTOs
// uses. They are strings because that is how the attribute spells them, and
// because the message quotes them verbatim.
const (
	// MoneyMinPositive is the lower bound of an amount that must be paid.
	MoneyMinPositive = "0.01"
	// MoneyMinZero is the lower bound of an amount that may be nothing.
	MoneyMinZero = "0.00"
	// MoneyMax is the largest value numeric(18,2) holds at two decimal places.
	MoneyMax = "999999999999.99"
)

const (
	rangeMessage = "The field %s must be between %s and %s."

	// missingMembersMessage stands in for the JsonException System.Text.Json
	// raises for an absent `required` member. MVC surfaces that under the "$"
	// key with the CLR type name in the text; the wording here is this API's,
	// the status (400) and the body shape are the .NET API's.
	missingMembersMessage = "The JSON payload was missing required properties, including the following: %s"
)

// decimalRange mirrors RangeAttribute over a decimal. Every money field in the
// DTOs shares MoneyMax as its upper bound, so only the lower one varies. An
// absent value is not its business: `required` members are checked by
// requiredMembers first.
func decimalRange(errs *domain.ValidationError, field string, value *domain.Money, minimum string) {
	if value == nil {
		return
	}

	low := decimal.RequireFromString(minimum)
	high := decimal.RequireFromString(MoneyMax)
	if value.LessThan(low) || value.GreaterThan(high) {
		errs.Add(field, fmt.Sprintf(rangeMessage, field, minimum, MoneyMax))
	}
}

// intRange mirrors RangeAttribute over an int, as used by the paging query.
func intRange(errs *domain.ValidationError, field string, value *int, minimum, maximum int) {
	if value == nil {
		return
	}
	if *value < minimum || *value > maximum {
		errs.Add(field, fmt.Sprintf(rangeMessage, field,
			strconv.Itoa(minimum), strconv.Itoa(maximum)))
	}
}

// requiredMembers reports the `required` members the payload left out. .NET
// fails deserialization for these, so nothing else about the body is checked;
// callers return as soon as this reports something.
func requiredMembers(errs *domain.ValidationError, missing []string) bool {
	if len(missing) == 0 {
		return false
	}
	errs.Add(domain.JSONBodyField, fmt.Sprintf(missingMembersMessage, strings.Join(missing, ", ")))
	return true
}
