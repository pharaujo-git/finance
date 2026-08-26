package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// defaultCategoryMessage is the 400 both edit paths raise for a shared default.
const defaultCategoryMessage = "Default categories cannot be modified."

// CategoryService is the Go twin of FinanceTracker's CategoryService: the
// categories visible to a user are the shared defaults plus their own, and the
// defaults are read-only for everyone.
type CategoryService struct {
	categories CategoryRepository

	newID func() uuid.UUID
}

// NewCategoryService wires the service to its port.
func NewCategoryService(categories CategoryRepository) *CategoryService {
	return &CategoryService{categories: categories, newID: uuid.New}
}

// List returns the visible categories ordered by type then name.
func (s *CategoryService) List(ctx context.Context, userID uuid.UUID) ([]CategoryDto, error) {
	categories, err := s.categories.ListVisible(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("application: listing categories: %w", err)
	}

	dtos := make([]CategoryDto, 0, len(categories))
	for _, category := range categories {
		dtos = append(dtos, NewCategoryDto(category))
	}
	return dtos, nil
}

// Create adds a category owned by the caller. Duplicate names are allowed, as
// they are in the .NET service.
func (s *CategoryService) Create(
	ctx context.Context,
	userID uuid.UUID,
	request CategoryRequest,
) (CategoryDto, error) {
	if err := request.Validate(); err != nil {
		return CategoryDto{}, err
	}

	owner := userID
	category := &domain.Category{
		Id:        s.newID(),
		UserId:    &owner,
		Name:      strings.TrimSpace(request.Name),
		Type:      *request.Type,
		Icon:      strings.TrimSpace(request.Icon),
		Color:     strings.TrimSpace(request.Color),
		IsDefault: false,
	}

	if err := s.categories.Add(ctx, category); err != nil {
		return CategoryDto{}, fmt.Errorf("application: inserting category: %w", err)
	}
	return NewCategoryDto(*category), nil
}

// Update rewrites a category the caller owns. The type is editable, as it is in
// the .NET service; only the shared defaults are frozen.
func (s *CategoryService) Update(
	ctx context.Context,
	userID, id uuid.UUID,
	request CategoryRequest,
) (CategoryDto, error) {
	if err := request.Validate(); err != nil {
		return CategoryDto{}, err
	}

	category, err := s.loadEditable(ctx, userID, id)
	if err != nil {
		return CategoryDto{}, err
	}

	category.Name = strings.TrimSpace(request.Name)
	category.Type = *request.Type
	category.Icon = strings.TrimSpace(request.Icon)
	category.Color = strings.TrimSpace(request.Color)

	if err := s.categories.Update(ctx, category); err != nil {
		if errors.Is(err, ErrRowNotFound) {
			return CategoryDto{}, domain.NotFound(categoryEntityName)
		}
		return CategoryDto{}, fmt.Errorf("application: updating category: %w", err)
	}
	return NewCategoryDto(*category), nil
}

// Delete removes a category the caller owns, detaching it from their
// transactions and dropping their budgets for it in the same transaction.
func (s *CategoryService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	if _, err := s.loadEditable(ctx, userID, id); err != nil {
		return err
	}

	if err := s.categories.Delete(ctx, id, userID); err != nil {
		if errors.Is(err, ErrRowNotFound) {
			return domain.NotFound(categoryEntityName)
		}
		return fmt.Errorf("application: deleting category: %w", err)
	}
	return nil
}

// EnsureUsable throws when the category does not exist or is not usable by this
// user. A nil id is fine: transactions and rules may be uncategorized.
func (s *CategoryService) EnsureUsable(ctx context.Context, userID uuid.UUID, categoryID *uuid.UUID) error {
	if categoryID == nil {
		return nil
	}

	_, err := s.categories.FindVisible(ctx, *categoryID, userID)
	switch {
	case errors.Is(err, ErrRowNotFound):
		return domain.NotFound(categoryEntityName)
	case err != nil:
		return fmt.Errorf("application: checking category visibility: %w", err)
	}
	return nil
}

// loadEditable is LoadEditableAsync: an invisible category is a 404, and a
// visible default is a 400.
func (s *CategoryService) loadEditable(
	ctx context.Context,
	userID, id uuid.UUID,
) (*domain.Category, error) {
	category, err := s.categories.FindVisible(ctx, id, userID)
	switch {
	case errors.Is(err, ErrRowNotFound):
		return nil, domain.NotFound(categoryEntityName)
	case err != nil:
		return nil, fmt.Errorf("application: reading category: %w", err)
	}

	if category.IsDefault {
		return nil, domain.BadRequest(defaultCategoryMessage)
	}
	return category, nil
}

// categoryInfo is what an aggregation needs to label a group.
type categoryInfo struct {
	Name  string
	Color string
	Type  domain.CategoryType
}

// The labels an aggregation falls back to for a transaction with no category.
const (
	uncategorizedName  = "Uncategorized"
	uncategorizedColor = "#94a3b8"
)

// lookup builds the id-to-label map the dashboard and reports use, defaulting a
// blank colour to the uncategorized grey exactly as the .NET service does.
func (s *CategoryService) lookup(
	ctx context.Context,
	userID uuid.UUID,
) (map[uuid.UUID]categoryInfo, error) {
	categories, err := s.categories.ListVisible(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("application: listing categories: %w", err)
	}

	infos := make(map[uuid.UUID]categoryInfo, len(categories))
	for _, category := range categories {
		color := category.Color
		if color == "" {
			color = uncategorizedColor
		}
		infos[category.Id] = categoryInfo{Name: category.Name, Color: color, Type: category.Type}
	}
	return infos, nil
}

// describe is the Describe helper: an unknown or absent category reads as an
// expense called Uncategorized.
func describe(lookup map[uuid.UUID]categoryInfo, categoryID *uuid.UUID) categoryInfo {
	if categoryID != nil {
		if info, ok := lookup[*categoryID]; ok {
			return info
		}
	}
	return categoryInfo{Name: uncategorizedName, Color: uncategorizedColor, Type: domain.CategoryExpense}
}
