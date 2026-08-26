package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// The JSON names are what JsonStringEnumConverter(JsonNamingPolicy.CamelCase)
// writes; "creditCard" is the one that is not a plain lowercase.
func TestEnumsMarshalAsCamelCaseNames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		value any
		want  string
	}{
		{domain.AccountChecking, `"checking"`},
		{domain.AccountSavings, `"savings"`},
		{domain.AccountCreditCard, `"creditCard"`},
		{domain.AccountCash, `"cash"`},
		{domain.AccountInvestment, `"investment"`},
		{domain.CategoryIncome, `"income"`},
		{domain.CategoryExpense, `"expense"`},
		{domain.TransactionIncome, `"income"`},
		{domain.TransactionExpense, `"expense"`},
		{domain.TransactionTransfer, `"transfer"`},
		{domain.FrequencyDaily, `"daily"`},
		{domain.FrequencyWeekly, `"weekly"`},
		{domain.FrequencyMonthly, `"monthly"`},
		{domain.FrequencyYearly, `"yearly"`},
	}

	for _, testCase := range cases {
		encoded, err := json.Marshal(testCase.value)
		if err != nil {
			t.Fatalf("Marshal(%v): %v", testCase.value, err)
		}
		if string(encoded) != testCase.want {
			t.Errorf("Marshal(%v) = %s, want %s", testCase.value, encoded, testCase.want)
		}
	}
}

// The ordinals are what the integer columns store, so they must match the
// declaration order of the C# enums exactly.
func TestEnumOrdinalsMatchTheSchema(t *testing.T) {
	t.Parallel()

	if int(domain.AccountChecking) != 0 || int(domain.AccountCreditCard) != 2 ||
		int(domain.AccountInvestment) != 4 {
		t.Error("AccountType ordinals drifted from FinanceTracker.Domain.AccountType")
	}
	if int(domain.CategoryIncome) != 0 || int(domain.CategoryExpense) != 1 {
		t.Error("CategoryType ordinals drifted")
	}
	if int(domain.TransactionIncome) != 0 || int(domain.TransactionTransfer) != 2 {
		t.Error("TransactionType ordinals drifted")
	}
	if int(domain.FrequencyDaily) != 0 || int(domain.FrequencyYearly) != 3 {
		t.Error("Frequency ordinals drifted")
	}
}

func TestEnumsUnmarshalNamesAndOrdinals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		json string
		want domain.AccountType
	}{
		{`"creditCard"`, domain.AccountCreditCard},
		{`"CreditCard"`, domain.AccountCreditCard},
		{`"CREDITCARD"`, domain.AccountCreditCard},
		// JsonStringEnumConverter accepts integers on read by default.
		{`2`, domain.AccountCreditCard},
	}

	for _, testCase := range cases {
		var got domain.AccountType
		if err := json.Unmarshal([]byte(testCase.json), &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", testCase.json, err)
		}
		if got != testCase.want {
			t.Errorf("Unmarshal(%s) = %v, want %v", testCase.json, got, testCase.want)
		}
	}
}

func TestUnknownEnumNameIsAnError(t *testing.T) {
	t.Parallel()

	var value domain.TransactionType
	if err := json.Unmarshal([]byte(`"teleport"`), &value); err == nil {
		t.Error("Unmarshal accepted a name that is not a member")
	}
	if err := json.Unmarshal([]byte(`{}`), &value); err == nil {
		t.Error("Unmarshal accepted an object")
	}
}

// An ordinal outside the declared set round-trips as a number rather than
// failing, which is what System.Text.Json does for an undefined enum value.
func TestUndefinedOrdinalsRoundTripAsNumbers(t *testing.T) {
	t.Parallel()

	var value domain.AccountType
	if err := json.Unmarshal([]byte(`99`), &value); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if value.Valid() {
		t.Error("Valid() = true for an undefined ordinal")
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(encoded) != "99" {
		t.Errorf("Marshal = %s, want 99", encoded)
	}
}

func TestParsingFromAQueryString(t *testing.T) {
	t.Parallel()

	if value, ok := domain.ParseTransactionType(" Expense "); !ok || value != domain.TransactionExpense {
		t.Errorf("ParseTransactionType = %v, %v", value, ok)
	}
	if value, ok := domain.ParseTransactionType("2"); !ok || value != domain.TransactionTransfer {
		t.Errorf("ParseTransactionType(2) = %v, %v", value, ok)
	}
	if _, ok := domain.ParseTransactionType("teleport"); ok {
		t.Error("ParseTransactionType accepted a name that is not a member")
	}

	if value, ok := domain.ParseAccountType("creditcard"); !ok || value != domain.AccountCreditCard {
		t.Errorf("ParseAccountType = %v, %v", value, ok)
	}
	if value, ok := domain.ParseCategoryType("income"); !ok || value != domain.CategoryIncome {
		t.Errorf("ParseCategoryType = %v, %v", value, ok)
	}
	if value, ok := domain.ParseFrequency("YEARLY"); !ok || value != domain.FrequencyYearly {
		t.Errorf("ParseFrequency = %v, %v", value, ok)
	}
}

func TestEnumStrings(t *testing.T) {
	t.Parallel()

	if domain.TransactionTransfer.String() != "transfer" {
		t.Errorf("String() = %q", domain.TransactionTransfer.String())
	}
	if domain.Frequency(9).String() != "9" {
		t.Errorf("String() = %q, want the bare ordinal", domain.Frequency(9).String())
	}
	if !domain.CategoryExpense.Valid() || domain.CategoryType(7).Valid() {
		t.Error("Valid() disagrees with Enum.IsDefined")
	}
}
