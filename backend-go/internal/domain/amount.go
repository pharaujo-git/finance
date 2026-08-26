package domain

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

// Amount is the wire form of Money. It exists because the two libraries
// disagree about trailing zeros: System.Text.Json writes a .NET decimal with
// the scale it carries (1250.00m serialises as 1250.00, and 12.5m as 12.5),
// while shopspring's String drops trailing zeros and would render the same
// stored balance as 1250.5. Reading a value back is delegated to shopspring,
// which accepts both a JSON number and a quoted one — as MVC's web defaults do.
type Amount decimal.Decimal

// NewAmount projects a computed or stored value onto the wire.
func NewAmount(value Money) Amount { return Amount(value) }

// Money unwraps the amount for arithmetic.
func (a Amount) Money() Money { return decimal.Decimal(a) }

// String renders the value with the scale it carries.
func (a Amount) String() string {
	value := decimal.Decimal(a)
	if exponent := value.Exponent(); exponent < 0 {
		return value.StringFixed(-exponent)
	}
	return value.String()
}

// MarshalJSON writes a bare JSON number, never a string.
func (a Amount) MarshalJSON() ([]byte, error) { return []byte(a.String()), nil }

// UnmarshalJSON reads a JSON number or a quoted number.
func (a *Amount) UnmarshalJSON(data []byte) error {
	var value decimal.Decimal
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*a = Amount(value)
	return nil
}
