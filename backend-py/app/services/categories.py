"""Categories, plus the lookup the analytics use to label a slice."""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from typing import Any

from app.core.errors import not_found, validation
from app.domain.enums import CategoryType
from app.repositories.categories import (
    UNCATEGORIZED_COLOR,
    UNCATEGORIZED_NAME,
    Category,
    CategoryRepository,
)

CATEGORY_ENTITY = "Category"
DEFAULT_CATEGORY_MESSAGE = "Default categories cannot be modified."


def category_dto(category: Category) -> dict[str, Any]:
    return {
        "id": category.id,
        "name": category.name,
        "type": category.type,
        "icon": category.icon,
        "color": category.color,
        "isDefault": category.is_default,
    }


@dataclass(slots=True)
class CategoryInfo:
    """What a chart needs to draw one slice."""

    name: str
    color: str
    type: CategoryType


def describe(lookup: dict[uuid.UUID, CategoryInfo], category_id: uuid.UUID | None) -> CategoryInfo:
    """Labels a slice, falling back to the shared "Uncategorized" grey."""
    if category_id is not None:
        found = lookup.get(category_id)
        if found is not None:
            return found
    return CategoryInfo(UNCATEGORIZED_NAME, UNCATEGORIZED_COLOR, CategoryType.EXPENSE)


class CategoryService:
    def __init__(self, categories: CategoryRepository) -> None:
        self._categories = categories

    async def list_all(self, user_id: uuid.UUID) -> list[dict[str, Any]]:
        return [category_dto(item) for item in await self._categories.list_visible(user_id)]

    async def lookup(self, user_id: uuid.UUID) -> dict[uuid.UUID, CategoryInfo]:
        """Id -> label. A blank stored colour falls back to the grey too."""
        return {
            item.id: CategoryInfo(item.name, item.color or UNCATEGORIZED_COLOR, item.type)
            for item in await self._categories.list_visible(user_id)
        }

    async def ensure_usable(self, user_id: uuid.UUID, category_id: uuid.UUID | None) -> None:
        """A null category is fine; one the caller cannot see is a 404."""
        if category_id is None:
            return
        if await self._categories.get(user_id, category_id) is None:
            raise not_found(CATEGORY_ENTITY)

    async def create(
        self, user_id: uuid.UUID, *, name: str, category_type: CategoryType, icon: str, color: str
    ) -> dict[str, Any]:
        category = Category(
            id=uuid.uuid4(),
            user_id=user_id,
            name=name.strip(),
            type=category_type,
            icon=icon.strip(),
            color=color.strip(),
            is_default=False,
        )
        await self._categories.add(category)
        return category_dto(category)

    async def update(
        self,
        user_id: uuid.UUID,
        category_id: uuid.UUID,
        *,
        name: str,
        category_type: CategoryType,
        icon: str,
        color: str,
    ) -> dict[str, Any]:
        category = await self._load_owned(user_id, category_id)
        category.name = name.strip()
        category.type = category_type
        category.icon = icon.strip()
        category.color = color.strip()

        await self._categories.update(category)
        return category_dto(category)

    async def delete(self, user_id: uuid.UUID, category_id: uuid.UUID) -> None:
        await self._load_owned(user_id, category_id)
        await self._categories.delete(user_id, category_id)

    async def _load_owned(self, user_id: uuid.UUID, category_id: uuid.UUID) -> Category:
        """A shared default is visible to everyone but editable by no one."""
        visible = await self._categories.get(user_id, category_id)
        if visible is None:
            raise not_found(CATEGORY_ENTITY)
        if visible.is_default:
            raise validation(DEFAULT_CATEGORY_MESSAGE)

        owned = await self._categories.get_owned(user_id, category_id)
        if owned is None:
            raise not_found(CATEGORY_ENTITY)
        return owned
