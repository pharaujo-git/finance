package domain

import "github.com/shopspring/decimal"

// MoneyScale is the number of decimal places the schema stores: every money
// column in db/migrations is numeric(18,2).
const MoneyScale int32 = 2

// Money is the amount type carried by the entities. A decimal keeps the
// arithmetic exact, which float64 would not.
type Money = decimal.Decimal

func init() {
	// shopspring/decimal writes a quoted string by default. The .NET API writes
	// decimals as JSON numbers, so the whole binary opts into numbers; there is
	// no per-value knob, and every decimal this program serialises is money or
	// a rate that the .NET API renders the same way.
	decimal.MarshalJSONWithoutQuotes = true
}

// Zero is the additive identity, handy as a fold seed. It renders as `0`, which
// is what System.Text.Json writes for `default(decimal)`.
func Zero() Money { return decimal.Zero }

// RoundMoney matches decimal.Round(value, 2, MidpointRounding.AwayFromZero).
// The guard matters for JSON parity rather than for arithmetic: .NET's Round
// never lengthens a value's scale, so an amount posted as 12.5 comes back as
// 12.5, while shopspring's Round on its own would render it as 12.50.
func RoundMoney(value Money) Money {
	if value.Exponent() >= -MoneyScale {
		return value
	}
	return value.Round(MoneyScale)
}

// TrimMoney drops trailing fractional zeros, which is how a .NET decimal
// division reports its result: 250m/500m is 0.5m, not 0.5000m. Only the
// computed savings rate needs it; stored amounts keep the scale they were
// written with.
func TrimMoney(value Money) Money {
	for value.Exponent() < 0 {
		shorter := value.Truncate(-value.Exponent() - 1)
		if !shorter.Equal(value) {
			break
		}
		value = shorter
	}
	return value
}
