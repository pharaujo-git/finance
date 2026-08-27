"""The identity layer, which has to interoperate with the .NET and Go APIs."""

from __future__ import annotations

import base64
import hashlib
import uuid
from datetime import UTC, datetime, timedelta

import jwt
import pytest

from app.core.security import (
    DEFAULT_ITERATIONS,
    DEFAULT_PRF,
    PRF_HMAC_SHA256,
    InvalidTokenError,
    PasswordOutcome,
    TokenService,
    hash_password,
    verify_password,
)

SECRET = "finance-tracker-local-development-signing-key-please-override"


def _blob(password: str, *, prf: int, iterations: int, salt_len: int, subkey_len: int) -> str:
    """Builds an Identity v3 blob with chosen parameters."""
    digests = {0: "sha1", 1: "sha256", 2: "sha512"}
    salt = b"\x01" * salt_len
    subkey = hashlib.pbkdf2_hmac(digests[prf], password.encode(), salt, iterations, subkey_len)
    header = (
        bytes([0x01])
        + prf.to_bytes(4, "big")
        + iterations.to_bytes(4, "big")
        + salt_len.to_bytes(4, "big")
    )
    return base64.b64encode(header + salt + subkey).decode()


class TestPasswordHashing:
    def test_round_trips(self) -> None:
        stored = hash_password("Passw0rd!123")
        assert verify_password(stored, "Passw0rd!123") is PasswordOutcome.SUCCESS

    def test_rejects_a_wrong_password(self) -> None:
        stored = hash_password("Passw0rd!123")
        assert verify_password(stored, "Passw0rd!124") is PasswordOutcome.FAILED

    def test_writes_the_current_defaults(self) -> None:
        raw = base64.b64decode(hash_password("x"))
        assert raw[0] == 0x01
        assert int.from_bytes(raw[1:5], "big") == DEFAULT_PRF
        assert int.from_bytes(raw[5:9], "big") == DEFAULT_ITERATIONS
        assert int.from_bytes(raw[9:13], "big") == 16
        assert len(raw) == 13 + 16 + 32

    def test_salts_differ_between_hashes(self) -> None:
        # Same password, two calls: a fresh salt each time means the blobs differ.
        first = hash_password("same")
        second = hash_password("same")
        assert first != second

    def test_accepts_a_weaker_blob_but_asks_for_a_rehash(self) -> None:
        # A blob the .NET API would have written years ago.
        stored = _blob("legacy", prf=PRF_HMAC_SHA256, iterations=10_000, salt_len=16, subkey_len=32)
        assert verify_password(stored, "legacy") is PasswordOutcome.SUCCESS_REHASH_NEEDED

    def test_flags_a_short_subkey_for_rehash(self) -> None:
        stored = _blob("legacy", prf=2, iterations=DEFAULT_ITERATIONS, salt_len=16, subkey_len=16)
        assert verify_password(stored, "legacy") is PasswordOutcome.SUCCESS_REHASH_NEEDED

    @pytest.mark.parametrize(
        "stored",
        [
            "not base64 at all!!",
            "",
            base64.b64encode(b"\x02" + b"\x00" * 40).decode(),  # wrong format marker
            base64.b64encode(b"\x01").decode(),  # truncated
        ],
    )
    def test_rejects_malformed_blobs(self, stored: str) -> None:
        assert verify_password(stored, "anything") is PasswordOutcome.FAILED

    def test_rejects_a_salt_below_128_bits(self) -> None:
        stored = _blob("x", prf=2, iterations=1000, salt_len=8, subkey_len=32)
        assert verify_password(stored, "x") is PasswordOutcome.FAILED

    def test_rejects_an_absurd_iteration_count(self) -> None:
        raw = bytearray(base64.b64decode(hash_password("x")))
        raw[5:9] = (20_000_000).to_bytes(4, "big")
        assert verify_password(base64.b64encode(bytes(raw)).decode(), "x") is (
            PasswordOutcome.FAILED
        )


class TestTokenService:
    def test_round_trips(self) -> None:
        service = TokenService(SECRET)
        user_id = uuid.uuid4()
        principal = service.validate(service.issue(user_id, "owner@example.com"))
        assert principal.user_id == user_id
        assert principal.email == "owner@example.com"

    def test_writes_the_shared_claim_set(self) -> None:
        service = TokenService(SECRET)
        user_id = uuid.uuid4()
        claims = jwt.decode(
            service.issue(user_id, "owner@example.com"),
            SECRET,
            algorithms=["HS256"],
            audience="finance-tracker",
        )
        assert claims["iss"] == "finance-tracker"
        assert claims["aud"] == "finance-tracker"
        assert claims["sub"] == str(user_id)
        assert claims["email"] == "owner@example.com"
        assert claims["exp"] - claims["iat"] == 7 * 24 * 3600
        assert claims["nbf"] == claims["iat"]
        uuid.UUID(claims["jti"])  # a fresh id per token

    def test_rejects_another_secret(self) -> None:
        issued = TokenService(SECRET).issue(uuid.uuid4(), "a@b.c")
        with pytest.raises(InvalidTokenError):
            TokenService("a different signing key entirely").validate(issued)

    def test_rejects_a_tampered_payload(self) -> None:
        issued = TokenService(SECRET).issue(uuid.uuid4(), "a@b.c")
        header, payload, signature = issued.split(".")
        forged = f"{header}.{payload[:-4]}AAAA.{signature}"
        with pytest.raises(InvalidTokenError):
            TokenService(SECRET).validate(forged)

    def test_rejects_an_expired_token(self) -> None:
        service = TokenService(SECRET)
        long_ago = datetime.now(UTC) - timedelta(days=8)
        with pytest.raises(InvalidTokenError):
            service.validate(service.issue(uuid.uuid4(), "a@b.c", now=long_ago))

    def test_rejects_the_none_algorithm(self) -> None:
        forged = jwt.encode(
            {"sub": str(uuid.uuid4()), "iss": "finance-tracker", "aud": "finance-tracker"},
            key="",
            algorithm="none",
        )
        with pytest.raises(InvalidTokenError):
            TokenService(SECRET).validate(forged)

    def test_rejects_a_foreign_issuer(self) -> None:
        forged = jwt.encode(
            {
                "sub": str(uuid.uuid4()),
                "iss": "somewhere-else",
                "aud": "finance-tracker",
                "exp": int((datetime.now(UTC) + timedelta(days=1)).timestamp()),
            },
            SECRET,
            algorithm="HS256",
        )
        with pytest.raises(InvalidTokenError):
            TokenService(SECRET).validate(forged)

    def test_rejects_a_non_uuid_subject(self) -> None:
        forged = jwt.encode(
            {
                "sub": "not-a-uuid",
                "iss": "finance-tracker",
                "aud": "finance-tracker",
                "exp": int((datetime.now(UTC) + timedelta(days=1)).timestamp()),
            },
            SECRET,
            algorithm="HS256",
        )
        with pytest.raises(InvalidTokenError):
            TokenService(SECRET).validate(forged)
