// Package domain holds the entities and errors shared by every layer above it.
// The structs mirror the tables created by db/migrations, which the .NET API
// shares: field names match the quoted PascalCase columns one for one.
package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// TagSeparator is the unit separator (U+001F) FinanceTracker.Domain.Transaction
// joins tags with: safe because it cannot appear in user-entered tag text.
const TagSeparator = '\u001f'

// User mirrors the "Users" table.
type User struct {
	Id           uuid.UUID
	Email        string
	Name         string
	PasswordHash string
	Currency     string
	CreatedAt    time.Time
}

// Account mirrors the "Accounts" table: a bank/cash/credit account owned by a
// single user. InitialBalance is the opening balance; the live balance adds the
// signed sum of transactions.
type Account struct {
	Id             uuid.UUID
	UserId         uuid.UUID
	Name           string
	Type           AccountType
	InitialBalance Money
	Currency       string
	IsArchived     bool
	CreatedAt      time.Time
}

// Category mirrors the "Categories" table. UserId is nil for the globally
// seeded default categories, which belong to nobody and are read-only.
type Category struct {
	Id        uuid.UUID
	UserId    *uuid.UUID
	Name      string
	Type      CategoryType
	Icon      string
	Color     string
	IsDefault bool
}

// Transaction mirrors the "Transactions" table. Amount is always stored
// positive; Type carries the sign.
type Transaction struct {
	Id                uuid.UUID
	UserId            uuid.UUID
	AccountId         uuid.UUID
	CategoryId        *uuid.UUID
	Type              TransactionType
	Amount            Money
	Date              time.Time
	Description       string
	Notes             *string
	TagsRaw           string
	TransferAccountId *uuid.UUID
}

// RecurringRule mirrors the "RecurringRules" table: a template a background
// worker materializes into transactions on a schedule.
type RecurringRule struct {
	Id          uuid.UUID
	UserId      uuid.UUID
	AccountId   uuid.UUID
	CategoryId  *uuid.UUID
	Type        TransactionType
	Amount      Money
	Description string
	Frequency   Frequency
	StartDate   time.Time
	EndDate     *time.Time
	NextRunDate time.Time
	IsActive    bool
}

// Budget mirrors the "Budgets" table: a monthly spending limit for one
// category. Month is a calendar month in YYYY-MM form.
type Budget struct {
	Id         uuid.UUID
	UserId     uuid.UUID
	CategoryId uuid.UUID
	Month      string
	Limit      Money
}

// Goal mirrors the "Goals" table: a savings target the user contributes to.
type Goal struct {
	Id            uuid.UUID
	UserId        uuid.UUID
	Name          string
	TargetAmount  Money
	CurrentAmount Money
	TargetDate    *time.Time
	Color         string
}

// JoinTags is Transaction.JoinTags: blank entries are dropped and the rest are
// trimmed before being joined with the separator.
func JoinTags(tags []string) string {
	kept := make([]string, 0, len(tags))
	for _, tag := range tags {
		if trimmed := strings.TrimSpace(tag); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	return strings.Join(kept, string(TagSeparator))
}

// SplitTags is Transaction.SplitTags. It never returns nil, so the API renders
// an empty tag list as [] rather than null, as the .NET API does.
func SplitTags(raw string) []string {
	if raw == "" {
		return []string{}
	}

	parts := strings.Split(raw, string(TagSeparator))
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		// StringSplitOptions.RemoveEmptyEntries, which drops empties without
		// trimming: the values were trimmed on the way in.
		if part != "" {
			tags = append(tags, part)
		}
	}
	return tags
}
