"""Registration, sign-in and profile maintenance."""

from __future__ import annotations

import uuid
from datetime import UTC, datetime
from typing import Any

from app.core.errors import conflict, not_found, unauthorized
from app.core.security import PasswordOutcome, TokenService, hash_password, verify_password
from app.repositories.users import EmailTakenError, User, UserRepository

# The frontend renders the problem document's `detail` verbatim, so these are
# byte-identical to the .NET and Go services'.
DEFAULT_CURRENCY = "USD"
DUPLICATE_EMAIL_MESSAGE = "An account with that email already exists."
INVALID_CREDENTIALS_MESSAGE = "Invalid email or password."
USER_ENTITY = "User"


def normalize_email(email: str) -> str:
    """Trim, then lowercase -- so "  Owner@Example.COM " is one account."""
    return email.strip().lower()


def user_dto(user: User) -> dict[str, Any]:
    return {
        "id": user.id,
        "email": user.email,
        "name": user.name,
        "currency": user.currency,
    }


class AuthService:
    def __init__(self, users: UserRepository, tokens: TokenService) -> None:
        self._users = users
        self._tokens = tokens

    async def register(self, email: str, password: str, name: str) -> dict[str, Any]:
        """Creates an account and signs the caller straight in."""
        normalized = normalize_email(email)
        if await self._users.find_by_email(normalized) is not None:
            raise conflict(DUPLICATE_EMAIL_MESSAGE)

        user = User(
            id=uuid.uuid4(),
            email=normalized,
            name=name.strip(),
            password_hash=hash_password(password),
            currency=DEFAULT_CURRENCY,
            created_at=datetime.now(UTC),
        )

        try:
            await self._users.add(user)
        except EmailTakenError as exc:
            # The lookup above is racy on its own; the unique index decides, and
            # the loser reports the same conflict as the early check.
            raise conflict(DUPLICATE_EMAIL_MESSAGE) from exc

        return self._auth_response(user)

    async def login(self, email: str, password: str) -> dict[str, Any]:
        """Verifies credentials and issues a token.

        An unknown address and a wrong password fail identically, so the
        response cannot be used to enumerate accounts.
        """
        user = await self._users.find_by_email(normalize_email(email))
        if user is None:
            raise unauthorized(INVALID_CREDENTIALS_MESSAGE)

        outcome = verify_password(user.password_hash, password)
        if outcome is PasswordOutcome.FAILED:
            raise unauthorized(INVALID_CREDENTIALS_MESSAGE)

        if outcome is PasswordOutcome.SUCCESS_REHASH_NEEDED:
            # The stored blob predates the current parameters; upgrade it now
            # that the plaintext is in hand.
            upgraded = hash_password(password)
            await self._users.update_password_hash(user.id, upgraded)
            user.password_hash = upgraded

        return self._auth_response(user)

    async def profile(self, user_id: uuid.UUID) -> dict[str, Any]:
        return user_dto(await self._load(user_id))

    async def update_profile(self, user_id: uuid.UUID, name: str, currency: str) -> dict[str, Any]:
        user = await self._load(user_id)
        user.name = name.strip()
        user.currency = currency.strip().upper()

        if not await self._users.update_profile(user.id, user.name, user.currency):
            raise not_found(USER_ENTITY)
        return user_dto(user)

    async def _load(self, user_id: uuid.UUID) -> User:
        """A token whose subject no longer exists gets a 404, not a 401."""
        user = await self._users.find_by_id(user_id)
        if user is None:
            raise not_found(USER_ENTITY)
        return user

    def _auth_response(self, user: User) -> dict[str, Any]:
        return {"token": self._tokens.issue(user.id, user.email), "user": user_dto(user)}
