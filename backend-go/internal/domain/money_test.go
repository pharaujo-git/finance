package domain_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

func TestRoundMoneyMatchesNumeric18_2(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"1.005":   "1.01",
		"-1.005":  "-1.01",
		"2.344":   "2.34",
		"2.345":   "2.35",
		"10":      "10",
		"0.1":     "0.1",
		"-0.0049": "0",
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			value, err := decimal.NewFromString(input)
			if err != nil {
				t.Fatalf("parsing %q: %v", input, err)
			}
			expected, err := decimal.NewFromString(want)
			if err != nil {
				t.Fatalf("parsing %q: %v", want, err)
			}
			if got := domain.RoundMoney(value); !got.Equal(expected) {
				t.Errorf("RoundMoney(%s) = %s, want %s", input, got, want)
			}
		})
	}

	if !domain.Zero().IsZero() {
		t.Error("Zero() is not zero")
	}
}
