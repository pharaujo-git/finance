"""Savings goals. Progress is left to the client: the API stores raw amounts."""

from __future__ import annotations

import uuid
from datetime import datetime
from decimal import Decimal
from typing import Any

from app.core.errors import not_found, validation
from app.domain.money import ZERO, as_utc, round_money
from app.repositories.goals import Goal, GoalRepository

GOAL_ENTITY = "Goal"
CONTRIBUTION_MESSAGE = "Contribution amount must be greater than zero."


def goal_dto(goal: Goal) -> dict[str, Any]:
    return {
        "id": goal.id,
        "name": goal.name,
        "targetAmount": goal.target_amount,
        "currentAmount": goal.current_amount,
        "targetDate": goal.target_date,
        "color": goal.color,
    }


class GoalService:
    def __init__(self, goals: GoalRepository) -> None:
        self._goals = goals

    async def list_all(self, user_id: uuid.UUID) -> list[dict[str, Any]]:
        return [goal_dto(goal) for goal in await self._goals.list_all(user_id)]

    async def create(
        self,
        user_id: uuid.UUID,
        *,
        name: str,
        target_amount: Decimal,
        current_amount: Decimal | None,
        target_date: datetime | None,
        color: str,
    ) -> dict[str, Any]:
        goal = Goal(
            id=uuid.uuid4(),
            user_id=user_id,
            name=name.strip(),
            target_amount=round_money(target_amount),
            current_amount=round_money(current_amount if current_amount is not None else ZERO),
            target_date=as_utc(target_date) if target_date else None,
            color=color.strip(),
        )
        await self._goals.add(goal)
        return goal_dto(goal)

    async def update(
        self,
        user_id: uuid.UUID,
        goal_id: uuid.UUID,
        *,
        name: str,
        target_amount: Decimal,
        current_amount: Decimal | None,
        target_date: datetime | None,
        color: str,
    ) -> dict[str, Any]:
        goal = await self._load(user_id, goal_id)
        goal.name = name.strip()
        goal.target_amount = round_money(target_amount)
        # An omitted currentAmount resets the goal rather than leaving it be,
        # matching the other two backends.
        goal.current_amount = round_money(current_amount if current_amount is not None else ZERO)
        goal.target_date = as_utc(target_date) if target_date else None
        goal.color = color.strip()

        await self._goals.update(goal)
        return goal_dto(goal)

    async def delete(self, user_id: uuid.UUID, goal_id: uuid.UUID) -> None:
        if not await self._goals.delete(user_id, goal_id):
            raise not_found(GOAL_ENTITY)

    async def contribute(
        self, user_id: uuid.UUID, goal_id: uuid.UUID, amount: Decimal
    ) -> dict[str, Any]:
        if amount <= ZERO:
            raise validation(CONTRIBUTION_MESSAGE)

        goal = await self._load(user_id, goal_id)
        # Not clamped at the target: a goal is allowed to be exceeded.
        goal.current_amount = round_money(goal.current_amount + amount)
        await self._goals.update(goal)
        return goal_dto(goal)

    async def _load(self, user_id: uuid.UUID, goal_id: uuid.UUID) -> Goal:
        goal = await self._goals.get(user_id, goal_id)
        if goal is None:
            raise not_found(GOAL_ENTITY)
        return goal
