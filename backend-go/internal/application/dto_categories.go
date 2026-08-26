package application

import (
	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// Extra ModelState keys used by the resource DTOs.
const (
	FieldIcon        = "Icon"
	FieldColor       = "Color"
	FieldDescription = "Description"
	FieldNotes       = "Notes"
	FieldAmount      = "Amount"
	FieldMonth       = "Month"
	FieldLimit       = "Limit"
	FieldTarget      = "TargetAmount"
	FieldCurrent     = "CurrentAmount"
	FieldPage        = "Page"
	FieldPageSize    = "PageSize"
	FieldSearch      = "Search"
)

// Column widths from db/migrations/0001_baseline.sql, which are also the
// MaxLength attributes on the .NET DTOs.
const (
	iconMaxLength        = 64
	colorMaxLength       = 32
	descriptionMaxLength = 500
	notesMaxLength       = 2000
	searchMaxLength      = 200
)

// CategoryDto is the wire shape of a category.
type CategoryDto struct {
	ID        uuid.UUID           `json:"id"`
	Name      string              `json:"name"`
	Type      domain.CategoryType `json:"type"`
	Icon      string              `json:"icon"`
	Color     string              `json:"color"`
	IsDefault bool                `json:"isDefault"`
}

// NewCategoryDto projects a category onto the wire.
func NewCategoryDto(category domain.Category) CategoryDto {
	return CategoryDto{
		ID:        category.Id,
		Name:      category.Name,
		Type:      category.Type,
		Icon:      category.Icon,
		Color:     category.Color,
		IsDefault: category.IsDefault,
	}
}

// CategoryRequest is the body of POST and PUT /api/categories.
type CategoryRequest struct {
	Name  string               `json:"name"`
	Type  *domain.CategoryType `json:"type"`
	Icon  string               `json:"icon"`
	Color string               `json:"color"`
}

// Validate mirrors the attributes on FinanceTracker's CategoryRequest.
func (r CategoryRequest) Validate() error {
	errs := domain.NewValidationError()
	if r.Type == nil {
		requiredMembers(errs, []string{"type"})
		return errs.OrNil()
	}

	required(errs, FieldName, r.Name)
	maxLength(errs, FieldName, r.Name, nameMaxLength)
	maxLength(errs, FieldIcon, r.Icon, iconMaxLength)
	maxLength(errs, FieldColor, r.Color, colorMaxLength)

	return errs.OrNil()
}
