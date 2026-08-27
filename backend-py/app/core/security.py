"""The two things all three backends must agree on byte for byte: the ASP.NET
Core Identity v3 password blob and the HS256 bearer token.

Ported from backend-go/internal/infrastructure/identity, which was itself
matched against the .NET API. A token minted here must be accepted there and
vice versa, so none of the constants below are free to change.
"""

from __future__ import annotations

import base64
import enum
import hashlib
import hmac
import secrets
import uuid
from datetime import UTC, datetime, timedelta
from typing import Final

import jwt

# --- password hashing -------------------------------------------------------
#
# Layout of the base64-encoded version-3 blob:
#
#   byte  0      format marker, always 0x01
#   bytes 1..4   PRF id, uint32 big-endian
#   bytes 5..8   iteration count, uint32 big-endian
#   bytes 9..12  salt length in bytes, uint32 big-endian
#   bytes 13..   salt, then the derived subkey (rest of the blob)

FORMAT_MARKER_V3: Final = 0x01
HEADER_LEN: Final = 13

# PRF 0 and 1 are only ever read from stored blobs; matching one means the hash
# predates the current defaults and should be rewritten on next sign-in.
PRF_HMAC_SHA1: Final = 0
PRF_HMAC_SHA256: Final = 1
PRF_HMAC_SHA512: Final = 2

_PRF_DIGESTS: Final = {
    PRF_HMAC_SHA1: "sha1",
    PRF_HMAC_SHA256: "sha256",
    PRF_HMAC_SHA512: "sha512",
}

DEFAULT_PRF: Final = PRF_HMAC_SHA512
DEFAULT_ITERATIONS: Final = 100_000
DEFAULT_SALT_LEN: Final = 16
DEFAULT_SUBKEY_LEN: Final = 32

# Identity rejects salts under 128 bits, and treats absurd iteration counts as
# corruption rather than working through them.
_MIN_SALT_LEN: Final = 16
_MAX_ITERATIONS: Final = 10_000_000


class PasswordOutcome(enum.Enum):
    """Mirrors Identity's PasswordVerificationResult."""

    FAILED = "failed"
    SUCCESS = "success"
    SUCCESS_REHASH_NEEDED = "success_rehash_needed"


def hash_password(password: str) -> str:
    """Derives a new Identity v3 blob using the current defaults."""
    salt = secrets.token_bytes(DEFAULT_SALT_LEN)
    subkey = hashlib.pbkdf2_hmac(
        _PRF_DIGESTS[DEFAULT_PRF], password.encode(), salt, DEFAULT_ITERATIONS, DEFAULT_SUBKEY_LEN
    )

    blob = bytearray(HEADER_LEN)
    blob[0] = FORMAT_MARKER_V3
    blob[1:5] = DEFAULT_PRF.to_bytes(4, "big")
    blob[5:9] = DEFAULT_ITERATIONS.to_bytes(4, "big")
    blob[9:13] = DEFAULT_SALT_LEN.to_bytes(4, "big")
    blob += salt
    blob += subkey

    return base64.b64encode(bytes(blob)).decode()


def verify_password(stored_hash: str, password: str) -> PasswordOutcome:
    """Checks a password against a stored blob.

    A correct password whose blob uses parameters weaker than the current
    defaults reports SUCCESS_REHASH_NEEDED, exactly as Identity's own hasher
    does, so the caller can transparently upgrade it.
    """
    try:
        blob = base64.b64decode(stored_hash, validate=True)
    except (ValueError, TypeError):
        return PasswordOutcome.FAILED

    if len(blob) < HEADER_LEN + 1 or blob[0] != FORMAT_MARKER_V3:
        return PasswordOutcome.FAILED

    prf_id = int.from_bytes(blob[1:5], "big")
    iterations = int.from_bytes(blob[5:9], "big")
    salt_len = int.from_bytes(blob[9:13], "big")

    # Both bounds also keep the slicing below in range.
    if salt_len < _MIN_SALT_LEN or HEADER_LEN + salt_len >= len(blob):
        return PasswordOutcome.FAILED
    if iterations == 0 or iterations > _MAX_ITERATIONS:
        return PasswordOutcome.FAILED

    digest = _PRF_DIGESTS.get(prf_id)
    if digest is None:
        return PasswordOutcome.FAILED

    salt_end = HEADER_LEN + salt_len
    salt = blob[HEADER_LEN:salt_end]
    expected = blob[salt_end:]
    if len(expected) < 16:
        return PasswordOutcome.FAILED

    actual = hashlib.pbkdf2_hmac(digest, password.encode(), salt, iterations, len(expected))
    if not hmac.compare_digest(actual, expected):
        return PasswordOutcome.FAILED

    if (
        prf_id != DEFAULT_PRF
        or iterations < DEFAULT_ITERATIONS
        or len(expected) < DEFAULT_SUBKEY_LEN
    ):
        return PasswordOutcome.SUCCESS_REHASH_NEEDED
    return PasswordOutcome.SUCCESS


# --- bearer tokens ----------------------------------------------------------

# Issuer, Audience and the 7-day lifetime match JwtOptions in the .NET API.
ISSUER: Final = "finance-tracker"
AUDIENCE: Final = "finance-tracker"
TOKEN_LIFETIME: Final = timedelta(days=7)
# Matches the JwtBearer ClockSkew of one minute.
CLOCK_LEEWAY: Final = timedelta(minutes=1)
SIGNING_ALG: Final = "HS256"


class InvalidTokenError(Exception):
    """Raised for every rejection reason, so callers cannot leak which check failed."""


class Principal:
    """The authenticated caller."""

    __slots__ = ("email", "user_id")

    def __init__(self, user_id: uuid.UUID, email: str) -> None:
        self.user_id = user_id
        self.email = email


class TokenService:
    """Issues and validates tokens interchangeable with the .NET and Go APIs."""

    def __init__(self, secret: str) -> None:
        self._secret = secret

    def issue(self, user_id: uuid.UUID, email: str, *, now: datetime | None = None) -> str:
        """Mints a token. The claim set matches what the other two backends write."""
        moment = (now or datetime.now(UTC)).astimezone(UTC)
        claims = {
            "iss": ISSUER,
            "aud": AUDIENCE,
            "exp": int((moment + TOKEN_LIFETIME).timestamp()),
            "iat": int(moment.timestamp()),
            "nbf": int(moment.timestamp()),
            "sub": str(user_id),
            "email": email,
            "jti": str(uuid.uuid4()),
        }
        return jwt.encode(claims, self._secret, algorithm=SIGNING_ALG)

    def validate(self, token: str) -> Principal:
        """Checks signature, issuer, audience and lifetime, then extracts the caller."""
        try:
            claims = jwt.decode(
                token,
                self._secret,
                algorithms=[SIGNING_ALG],
                issuer=ISSUER,
                audience=AUDIENCE,
                leeway=CLOCK_LEEWAY,
                options={"require": ["exp", "sub"]},
            )
        except jwt.PyJWTError as exc:
            raise InvalidTokenError(str(exc)) from exc

        subject = claims.get("sub") or ""
        try:
            user_id = uuid.UUID(subject)
        except (ValueError, AttributeError, TypeError) as exc:
            raise InvalidTokenError("subject is not a uuid") from exc

        email = claims.get("email")
        return Principal(user_id, email if isinstance(email, str) else "")
