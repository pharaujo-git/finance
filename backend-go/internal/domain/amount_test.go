package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// System.Text.Json writes a .NET decimal with the scale it carries, so a stored
// balance of 1250.00 must not render as 1250.5's shorter cousin.
func TestAmountKeepsItsScale(t *testing.T) {
	t.Parallel()

	cases := []struct {
		literal string
		want    string
	}{
		{"1250.00", "1250.00"},
		{"12.5", "12.5"},
		{"0.6667", "0.6667"},
		{"-75.50", "-75.50"},
		{"0", "0"},
		{"1000", "1000"},
	}

	for _, testCase := range cases {
		amount := domain.NewAmount(decimal.RequireFromString(testCase.literal))

		encoded, err := json.Marshal(amount)
		if err != nil {
			t.Fatalf("Marshal(%s): %v", testCase.literal, err)
		}
		if string(encoded) != testCase.want {
			t.Errorf("Marshal(%s) = %s, want %s", testCase.literal, encoded, testCase.want)
		}
	}
}

func TestZeroAmountRendersAsZero(t *testing.T) {
	t.Parallel()

	// The zero value stands in for `default(decimal)`, which the .NET API writes
	// as 0 whenever a lookup found nothing.
	encoded, err := json.Marshal(domain.Amount{})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(encoded) != "0" {
		t.Errorf("Marshal = %s, want 0", encoded)
	}
}

func TestAmountReadsNumbersAndStrings(t *testing.T) {
	t.Parallel()

	var fromNumber domain.Amount
	if err := json.Unmarshal([]byte("12.50"), &fromNumber); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if fromNumber.String() != "12.50" {
		t.Errorf("amount = %s, want 12.50", fromNumber.String())
	}

	// MVC's web defaults allow reading a number from a string.
	var fromString domain.Amount
	if err := json.Unmarshal([]byte(`"12.50"`), &fromString); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !fromString.Money().Equal(fromNumber.Money()) {
		t.Errorf("string form = %s, want the same value", fromString.String())
	}
}

// RoundMoney is decimal.Round(value, 2, MidpointRounding.AwayFromZero): it
// shortens a long scale and leaves a short one alone.
func TestRoundMoney(t *testing.T) {
	t.Parallel()

	cases := []struct{ from, want string }{
		{"10.005", "10.01"},
		{"-10.005", "-10.01"},
		{"10.004", "10.00"},
		{"12.5", "12.5"},
		{"12", "12"},
		{"1250.00", "1250.00"},
	}

	for _, testCase := range cases {
		got := domain.NewAmount(domain.RoundMoney(decimal.RequireFromString(testCase.from)))
		if got.String() != testCase.want {
			t.Errorf("RoundMoney(%s) = %s, want %s", testCase.from, got.String(), testCase.want)
		}
	}
}

// TrimMoney reproduces what a .NET decimal division reports: 250m/500m is 0.5m,
// not 0.5000m.
func TestTrimMoney(t *testing.T) {
	t.Parallel()

	cases := []struct{ from, want string }{
		{"0.5000", "0.5"},
		{"0.6667", "0.6667"},
		{"1.0000", "1"},
		{"0.0000", "0"},
		{"-0.2500", "-0.25"},
	}

	for _, testCase := range cases {
		got := domain.NewAmount(domain.TrimMoney(decimal.RequireFromString(testCase.from)))
		if got.String() != testCase.want {
			t.Errorf("TrimMoney(%s) = %s, want %s", testCase.from, got.String(), testCase.want)
		}
	}
}

func TestMoneyArithmeticKeepsTheLongerScale(t *testing.T) {
	t.Parallel()

	sum := decimal.RequireFromString("100.00").Add(decimal.Zero)
	if domain.NewAmount(sum).String() != "100.00" {
		t.Errorf("100.00 + 0 = %s, want 100.00", domain.NewAmount(sum).String())
	}

	difference := decimal.RequireFromString("50.00").Sub(decimal.RequireFromString("10.5"))
	if domain.NewAmount(difference).String() != "39.50" {
		t.Errorf("50.00 - 10.5 = %s, want 39.50", domain.NewAmount(difference).String())
	}
}
