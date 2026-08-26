package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// The four enums below are the Go twins of FinanceTracker.Domain.Enums. Two
// wire formats have to agree at once:
//
//   - the database stores the ordinal in an integer column, so the constants
//     must keep the declaration order of the C# enums;
//   - JSON carries the camelCase member name, because the .NET API registers
//     JsonStringEnumConverter(JsonNamingPolicy.CamelCase).
//
// An ordinal outside the declared set is preserved rather than rejected: the
// .NET converter accepts any number on read (AllowIntegerValues defaults to
// true) and writes an undefined value straight back out as a number.
type AccountType int

// Ordinals of FinanceTracker.Domain.AccountType.
const (
	AccountChecking AccountType = iota
	AccountSavings
	AccountCreditCard
	AccountCash
	AccountInvestment
)

// CategoryType says whether a category groups money coming in or going out.
type CategoryType int

// Ordinals of FinanceTracker.Domain.CategoryType.
const (
	CategoryIncome CategoryType = iota
	CategoryExpense
)

// TransactionType is the direction of a transaction.
type TransactionType int

// Ordinals of FinanceTracker.Domain.TransactionType.
const (
	TransactionIncome TransactionType = iota
	TransactionExpense
	TransactionTransfer
)

// Frequency is how often a recurring rule materializes a transaction.
type Frequency int

// Ordinals of FinanceTracker.Domain.Frequency.
const (
	FrequencyDaily Frequency = iota
	FrequencyWeekly
	FrequencyMonthly
	FrequencyYearly
)

// Wire names, in ordinal order. "creditCard" is the one member whose camelCase
// form differs from a plain lowercase of the C# name.
var (
	accountTypeNames     = []string{"checking", "savings", "creditCard", "cash", "investment"}
	categoryTypeNames    = []string{"income", "expense"}
	transactionTypeNames = []string{"income", "expense", "transfer"}
	frequencyNames       = []string{"daily", "weekly", "monthly", "yearly"}
)

// String returns the JSON name, or the bare ordinal for an undefined value.
func (t AccountType) String() string { return enumName(accountTypeNames, int(t)) }

// Valid reports whether the ordinal names a declared member, mirroring
// Enum.IsDefined.
func (t AccountType) Valid() bool { return enumValid(accountTypeNames, int(t)) }

// MarshalJSON writes the camelCase member name.
func (t AccountType) MarshalJSON() ([]byte, error) { return marshalEnum(accountTypeNames, int(t)) }

// UnmarshalJSON accepts the member name in any casing, or an ordinal.
func (t *AccountType) UnmarshalJSON(data []byte) error {
	value, err := unmarshalEnum(data, accountTypeNames, "AccountType")
	if err != nil {
		return err
	}
	*t = AccountType(value)
	return nil
}

// String returns the JSON name, or the bare ordinal for an undefined value.
func (t CategoryType) String() string { return enumName(categoryTypeNames, int(t)) }

// Valid reports whether the ordinal names a declared member.
func (t CategoryType) Valid() bool { return enumValid(categoryTypeNames, int(t)) }

// MarshalJSON writes the camelCase member name.
func (t CategoryType) MarshalJSON() ([]byte, error) { return marshalEnum(categoryTypeNames, int(t)) }

// UnmarshalJSON accepts the member name in any casing, or an ordinal.
func (t *CategoryType) UnmarshalJSON(data []byte) error {
	value, err := unmarshalEnum(data, categoryTypeNames, "CategoryType")
	if err != nil {
		return err
	}
	*t = CategoryType(value)
	return nil
}

// String returns the JSON name, or the bare ordinal for an undefined value.
func (t TransactionType) String() string { return enumName(transactionTypeNames, int(t)) }

// Valid reports whether the ordinal names a declared member.
func (t TransactionType) Valid() bool { return enumValid(transactionTypeNames, int(t)) }

// MarshalJSON writes the camelCase member name.
func (t TransactionType) MarshalJSON() ([]byte, error) {
	return marshalEnum(transactionTypeNames, int(t))
}

// UnmarshalJSON accepts the member name in any casing, or an ordinal.
func (t *TransactionType) UnmarshalJSON(data []byte) error {
	value, err := unmarshalEnum(data, transactionTypeNames, "TransactionType")
	if err != nil {
		return err
	}
	*t = TransactionType(value)
	return nil
}

// String returns the JSON name, or the bare ordinal for an undefined value.
func (f Frequency) String() string { return enumName(frequencyNames, int(f)) }

// Valid reports whether the ordinal names a declared member.
func (f Frequency) Valid() bool { return enumValid(frequencyNames, int(f)) }

// MarshalJSON writes the camelCase member name.
func (f Frequency) MarshalJSON() ([]byte, error) { return marshalEnum(frequencyNames, int(f)) }

// UnmarshalJSON accepts the member name in any casing, or an ordinal.
func (f *Frequency) UnmarshalJSON(data []byte) error {
	value, err := unmarshalEnum(data, frequencyNames, "Frequency")
	if err != nil {
		return err
	}
	*f = Frequency(value)
	return nil
}

// ParseAccountType reads a value the way ASP.NET Core's model binder does for
// a query string: the member name in any casing, or the ordinal as digits.
func ParseAccountType(value string) (AccountType, bool) {
	ordinal, ok := parseEnum(accountTypeNames, value)
	return AccountType(ordinal), ok
}

// ParseCategoryType reads a member name in any casing, or an ordinal.
func ParseCategoryType(value string) (CategoryType, bool) {
	ordinal, ok := parseEnum(categoryTypeNames, value)
	return CategoryType(ordinal), ok
}

// ParseTransactionType reads a member name in any casing, or an ordinal.
func ParseTransactionType(value string) (TransactionType, bool) {
	ordinal, ok := parseEnum(transactionTypeNames, value)
	return TransactionType(ordinal), ok
}

// ParseFrequency reads a member name in any casing, or an ordinal.
func ParseFrequency(value string) (Frequency, bool) {
	ordinal, ok := parseEnum(frequencyNames, value)
	return Frequency(ordinal), ok
}

func enumValid(names []string, ordinal int) bool {
	return ordinal >= 0 && ordinal < len(names)
}

func enumName(names []string, ordinal int) string {
	if enumValid(names, ordinal) {
		return names[ordinal]
	}
	return strconv.Itoa(ordinal)
}

func marshalEnum(names []string, ordinal int) ([]byte, error) {
	if enumValid(names, ordinal) {
		return json.Marshal(names[ordinal])
	}
	return []byte(strconv.Itoa(ordinal)), nil
}

// unmarshalEnum mirrors JsonStringEnumConverter: a string matches a member name
// case-insensitively, a number is taken as the ordinal whether or not it names
// a member, and anything else is an error the transport renders as a 400.
func unmarshalEnum(data []byte, names []string, typeName string) (int, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var name string
		if err := json.Unmarshal(trimmed, &name); err != nil {
			return 0, fmt.Errorf("the JSON value could not be converted to %s", typeName)
		}
		ordinal, ok := lookupEnum(names, name)
		if !ok {
			return 0, fmt.Errorf("the JSON value %q could not be converted to %s", name, typeName)
		}
		return ordinal, nil
	}

	var ordinal int
	if err := json.Unmarshal(trimmed, &ordinal); err != nil {
		return 0, fmt.Errorf("the JSON value could not be converted to %s", typeName)
	}
	return ordinal, nil
}

// parseEnum is the Enum.TryParse flavour used by the model binder and by CSV
// import: a member name in any casing, or digits. Digits are accepted even when
// they name no member, as Enum.Parse does; callers that also need
// Enum.IsDefined call Valid on the result.
func parseEnum(names []string, value string) (int, bool) {
	trimmed := strings.TrimSpace(value)
	if ordinal, err := strconv.Atoi(trimmed); err == nil {
		return ordinal, true
	}
	return lookupEnum(names, trimmed)
}

func lookupEnum(names []string, value string) (int, bool) {
	for ordinal, name := range names {
		if strings.EqualFold(name, value) {
			return ordinal, true
		}
	}
	return 0, false
}
