"""End-to-end tests through the ASGI app against a real Postgres schema."""

from __future__ import annotations

import uuid

import pytest
from httpx import AsyncClient

from tests.conftest import DEMO_EMAIL, DEMO_PASSWORD

MISSING = "00000000-0000-0000-0000-000000000000"


class TestProbes:
    async def test_health(self, client: AsyncClient) -> None:
        response = await client.get("/health")
        assert response.status_code == 200
        assert response.text == "ok"

    async def test_service_document_names_this_backend(self, client: AsyncClient) -> None:
        body = (await client.get("/")).json()
        assert body == {
            "service": "FinanceTracker API (Python)",
            "status": "ok",
            "docs": "/swagger",
        }


class TestAuth:
    async def test_register_returns_a_token_and_the_profile(self, client: AsyncClient) -> None:
        response = await client.post(
            "/api/auth/register",
            json={"email": "New@Example.COM ", "password": "Passw0rd!123", "name": " Ada "},
        )
        assert response.status_code == 200
        body = response.json()
        assert body["token"]
        # The address is trimmed and lowercased; the name is only trimmed.
        assert body["user"]["email"] == "new@example.com"
        assert body["user"]["name"] == "Ada"
        assert body["user"]["currency"] == "USD"

    async def test_rejects_a_duplicate_email(self, client: AsyncClient, token: str) -> None:
        response = await client.post(
            "/api/auth/register",
            json={"email": DEMO_EMAIL, "password": DEMO_PASSWORD, "name": "Twin"},
        )
        assert response.status_code == 409
        assert response.json()["detail"] == "An account with that email already exists."

    async def test_login_round_trips(self, client: AsyncClient, token: str) -> None:
        response = await client.post(
            "/api/auth/login", json={"email": DEMO_EMAIL, "password": DEMO_PASSWORD}
        )
        assert response.status_code == 200
        assert response.json()["user"]["email"] == DEMO_EMAIL

    @pytest.mark.parametrize(
        "payload",
        [
            {"email": DEMO_EMAIL, "password": "wrong-password"},
            {"email": "nobody@test.dev", "password": DEMO_PASSWORD},
        ],
    )
    async def test_bad_credentials_are_indistinguishable(
        self, client: AsyncClient, token: str, payload: dict
    ) -> None:
        response = await client.post("/api/auth/login", json=payload)
        assert response.status_code == 401
        assert response.json()["detail"] == "Invalid email or password."

    async def test_me_requires_a_token(self, client: AsyncClient) -> None:
        response = await client.get("/api/auth/me")
        assert response.status_code == 401
        assert response.headers["WWW-Authenticate"] == "Bearer"

    async def test_me_rejects_a_forged_token(self, client: AsyncClient) -> None:
        response = await client.get("/api/auth/me", headers={"Authorization": "Bearer not.a.token"})
        assert response.status_code == 401

    async def test_update_profile(self, client: AsyncClient, auth: dict) -> None:
        response = await client.put(
            "/api/auth/me", headers=auth, json={"name": " Grace ", "currency": " eur "}
        )
        assert response.status_code == 200
        assert response.json() == {
            **response.json(),
            "name": "Grace",
            "currency": "EUR",
        }

    async def test_validation_reports_every_broken_rule(self, client: AsyncClient) -> None:
        response = await client.post(
            "/api/auth/register", json={"email": "bad", "password": "x", "name": ""}
        )
        assert response.status_code == 400
        assert response.headers["content-type"] == "application/json; charset=utf-8"
        errors = response.json()["errors"]
        assert errors["Email"] == ["The Email field is not a valid e-mail address."]
        assert "The Name field is required." in errors["Name"]
        assert any("minimum length of '8'" in m for m in errors["Password"])


class TestAccounts:
    async def test_create_and_list(self, client: AsyncClient, auth: dict, account: dict) -> None:
        assert account["balance"] == 1000
        listed = (await client.get("/api/accounts", headers=auth)).json()
        assert [a["id"] for a in listed] == [account["id"]]

    async def test_balance_follows_the_transactions(
        self, client: AsyncClient, auth: dict, account: dict
    ) -> None:
        await client.post(
            "/api/transactions",
            headers=auth,
            json={
                "accountId": account["id"],
                "type": "expense",
                "amount": 42.50,
                "date": "2026-08-10",
                "description": "Lunch",
            },
        )
        refreshed = (await client.get(f"/api/accounts/{account['id']}", headers=auth)).json()
        assert refreshed["balance"] == 957.50

    async def test_a_transfer_moves_between_two_accounts(
        self, client: AsyncClient, auth: dict, account: dict
    ) -> None:
        savings = (
            await client.post(
                "/api/accounts",
                headers=auth,
                json={"name": "Savings", "type": "savings", "initialBalance": 0},
            )
        ).json()

        await client.post(
            "/api/transactions",
            headers=auth,
            json={
                "accountId": account["id"],
                "type": "transfer",
                "amount": 250,
                "date": "2026-08-15",
                "description": "To savings",
                "transferAccountId": savings["id"],
            },
        )

        listed = {
            a["name"]: a["balance"]
            for a in (await client.get("/api/accounts", headers=auth)).json()
        }
        assert listed["Checking"] == 750
        assert listed["Savings"] == 250

    async def test_delete_archives_rather_than_removing(
        self, client: AsyncClient, auth: dict, account: dict
    ) -> None:
        assert (
            await client.delete(f"/api/accounts/{account['id']}", headers=auth)
        ).status_code == 204
        refreshed = (await client.get(f"/api/accounts/{account['id']}", headers=auth)).json()
        assert refreshed["isArchived"] is True

    async def test_unknown_account_is_a_problem_document(
        self, client: AsyncClient, auth: dict
    ) -> None:
        response = await client.get(f"/api/accounts/{MISSING}", headers=auth)
        assert response.status_code == 404
        assert response.headers["content-type"] == "application/problem+json"
        assert response.json()["detail"] == "Account was not found."

    async def test_a_non_uuid_path_is_a_bare_404(self, client: AsyncClient, auth: dict) -> None:
        response = await client.get("/api/accounts/not-a-uuid", headers=auth)
        assert response.status_code == 404
        assert response.content == b""

    async def test_a_missing_required_member_short_circuits(
        self, client: AsyncClient, auth: dict
    ) -> None:
        response = await client.post("/api/accounts", headers=auth, json={"name": ""})
        assert response.status_code == 400
        # Only the "$" error: no other rule runs once a member is missing.
        assert response.json()["errors"] == {
            "$": ["The JSON payload was missing required properties, including the following: type"]
        }


class TestCategories:
    async def test_defaults_are_visible(self, client: AsyncClient, auth: dict) -> None:
        listed = (await client.get("/api/categories", headers=auth)).json()
        assert any(item["isDefault"] for item in listed)

    async def test_create_and_update(self, client: AsyncClient, auth: dict, category: dict) -> None:
        assert category["isDefault"] is False
        response = await client.put(
            f"/api/categories/{category['id']}",
            headers=auth,
            json={"name": "Dining", "type": "expense", "icon": "fork", "color": "#000000"},
        )
        assert response.status_code == 200
        assert response.json()["name"] == "Dining"

    async def test_a_default_category_cannot_be_modified(
        self, client: AsyncClient, auth: dict
    ) -> None:
        listed = (await client.get("/api/categories", headers=auth)).json()
        shared = next(item for item in listed if item["isDefault"])
        response = await client.delete(f"/api/categories/{shared['id']}", headers=auth)
        assert response.status_code == 400
        assert response.json()["detail"] == "Default categories cannot be modified."

    async def test_delete_detaches_it_from_transactions(
        self, client: AsyncClient, auth: dict, account: dict, category: dict
    ) -> None:
        created = (
            await client.post(
                "/api/transactions",
                headers=auth,
                json={
                    "accountId": account["id"],
                    "categoryId": category["id"],
                    "type": "expense",
                    "amount": 10,
                    "date": "2026-08-10",
                    "description": "Snack",
                },
            )
        ).json()

        assert (
            await client.delete(f"/api/categories/{category['id']}", headers=auth)
        ).status_code == 204

        refreshed = (await client.get(f"/api/transactions/{created['id']}", headers=auth)).json()
        assert refreshed["categoryId"] is None


class TestTransactions:
    async def test_create_normalises_the_row(
        self, client: AsyncClient, auth: dict, account: dict
    ) -> None:
        response = await client.post(
            "/api/transactions",
            headers=auth,
            json={
                "accountId": account["id"],
                "type": "expense",
                "amount": 42.505,
                "date": "2026-08-10",
                "description": "  Lunch  ",
                "notes": "   ",
                "tags": [" food ", "", "out"],
            },
        )
        body = response.json()
        assert body["amount"] == 42.51  # rounded half away from zero
        assert body["description"] == "Lunch"
        assert body["notes"] is None  # whitespace collapses to null
        assert body["tags"] == ["food", "out"]

    async def test_search_pages_and_filters(
        self, client: AsyncClient, auth: dict, account: dict
    ) -> None:
        for index in range(3):
            await client.post(
                "/api/transactions",
                headers=auth,
                json={
                    "accountId": account["id"],
                    "type": "expense",
                    "amount": 5 + index,
                    "date": f"2026-08-1{index}",
                    "description": f"Item {index}",
                },
            )

        page = (await client.get("/api/transactions?page=1&pageSize=2", headers=auth)).json()
        assert page["total"] == 3
        assert page["page"] == 1
        assert page["pageSize"] == 2
        assert len(page["items"]) == 2

        found = (await client.get("/api/transactions?search=item 1", headers=auth)).json()
        assert found["total"] == 1

    async def test_a_transfer_needs_a_different_destination(
        self, client: AsyncClient, auth: dict, account: dict
    ) -> None:
        payload = {
            "accountId": account["id"],
            "type": "transfer",
            "amount": 10,
            "date": "2026-08-10",
            "description": "Nowhere",
        }
        missing = await client.post("/api/transactions", headers=auth, json=payload)
        assert missing.status_code == 400
        assert missing.json()["detail"] == "A transfer requires a destination account."

        same = await client.post(
            "/api/transactions",
            headers=auth,
            json={**payload, "transferAccountId": account["id"]},
        )
        assert same.status_code == 400
        assert same.json()["detail"] == "A transfer must use two different accounts."

    async def test_update_and_delete(self, client: AsyncClient, auth: dict, account: dict) -> None:
        created = (
            await client.post(
                "/api/transactions",
                headers=auth,
                json={
                    "accountId": account["id"],
                    "type": "expense",
                    "amount": 10,
                    "date": "2026-08-10",
                    "description": "Before",
                },
            )
        ).json()

        updated = await client.put(
            f"/api/transactions/{created['id']}",
            headers=auth,
            json={
                "accountId": account["id"],
                "type": "income",
                "amount": 20,
                "date": "2026-08-11",
                "description": "After",
            },
        )
        assert updated.json()["description"] == "After"
        assert updated.json()["type"] == "income"

        assert (
            await client.delete(f"/api/transactions/{created['id']}", headers=auth)
        ).status_code == 204
        assert (
            await client.get(f"/api/transactions/{created['id']}", headers=auth)
        ).status_code == 404

    async def test_csv_round_trip(
        self, client: AsyncClient, auth: dict, account: dict, category: dict
    ) -> None:
        await client.post(
            "/api/transactions",
            headers=auth,
            json={
                "accountId": account["id"],
                "categoryId": category["id"],
                "type": "expense",
                "amount": 12.5,
                "date": "2026-08-10",
                "description": "Groceries, bulk",
            },
        )

        exported = await client.get("/api/transactions/export", headers=auth)
        assert exported.status_code == 200
        assert exported.headers["content-type"].startswith("text/csv")
        assert "attachment; filename=transactions-" in exported.headers["content-disposition"]

        lines = exported.text.splitlines()
        assert lines[0] == "Date,Type,Amount,Account,Category,Description,Notes,Tags"
        # The description carries a comma, so it must come back quoted.
        assert '"Groceries, bulk"' in lines[1]
        assert ",12.50," in lines[1]  # CSV always writes two places

        imported = await client.post(
            "/api/transactions/import",
            headers=auth,
            files={"file": ("t.csv", exported.text.encode(), "text/csv")},
        )
        assert imported.status_code == 200
        assert imported.json() == {"imported": 1, "skipped": 0}

    async def test_import_rejects_an_empty_file(self, client: AsyncClient, auth: dict) -> None:
        response = await client.post(
            "/api/transactions/import",
            headers=auth,
            files={"file": ("t.csv", b"", "text/csv")},
        )
        assert response.status_code == 400
        assert response.json()["detail"] == "A non-empty CSV file is required."

    async def test_import_skips_unusable_rows(
        self, client: AsyncClient, auth: dict, account: dict
    ) -> None:
        csv = (
            "Date,Type,Amount,Account,Category,Description,Notes,Tags\n"
            "2026-08-10,expense,10.00,Checking,,Good row,,\n"
            "not-a-date,expense,10.00,Checking,,Bad date,,\n"
            "2026-08-10,transfer,10.00,Checking,,Transfers never import,,\n"
            "2026-08-10,expense,10.00,No Such Account,,Unknown account,,\n"
        )
        response = await client.post(
            "/api/transactions/import",
            headers=auth,
            files={"file": ("t.csv", csv.encode(), "text/csv")},
        )
        assert response.json() == {"imported": 1, "skipped": 3}


class TestBudgets:
    async def test_create_and_measure_spend(
        self, client: AsyncClient, auth: dict, account: dict, category: dict
    ) -> None:
        await client.post(
            "/api/transactions",
            headers=auth,
            json={
                "accountId": account["id"],
                "categoryId": category["id"],
                "type": "expense",
                "amount": 30,
                "date": "2026-08-10",
                "description": "Food",
            },
        )

        created = await client.post(
            "/api/budgets",
            headers=auth,
            json={"categoryId": category["id"], "month": "2026-08", "limit": 100},
        )
        assert created.status_code == 200
        assert created.json()["spent"] == 30
        assert created.json()["remaining"] == 70

    async def test_rejects_a_duplicate(
        self, client: AsyncClient, auth: dict, category: dict
    ) -> None:
        payload = {"categoryId": category["id"], "month": "2026-08", "limit": 100}
        assert (await client.post("/api/budgets", headers=auth, json=payload)).status_code == 200
        clash = await client.post("/api/budgets", headers=auth, json=payload)
        assert clash.status_code == 409
        assert clash.json()["detail"] == "A budget already exists for that category and month."

    @pytest.mark.parametrize("month", ["nope", "2026-13"])
    async def test_rejects_a_bad_month(
        self, client: AsyncClient, auth: dict, category: dict, month: str
    ) -> None:
        response = await client.post(
            "/api/budgets",
            headers=auth,
            json={"categoryId": category["id"], "month": month, "limit": 100},
        )
        assert response.status_code == 400

    async def test_update_only_moves_the_limit(
        self, client: AsyncClient, auth: dict, category: dict
    ) -> None:
        created = (
            await client.post(
                "/api/budgets",
                headers=auth,
                json={"categoryId": category["id"], "month": "2026-08", "limit": 100},
            )
        ).json()
        updated = await client.put(
            f"/api/budgets/{created['id']}", headers=auth, json={"limit": 250}
        )
        assert updated.json()["limit"] == 250
        assert updated.json()["month"] == "2026-08"


class TestGoals:
    async def test_create_and_contribute(self, client: AsyncClient, auth: dict) -> None:
        created = (
            await client.post(
                "/api/goals",
                headers=auth,
                json={"name": "Trip", "targetAmount": 2000, "currentAmount": 100},
            )
        ).json()
        assert created["currentAmount"] == 100

        after = await client.post(
            f"/api/goals/{created['id']}/contribute", headers=auth, json={"amount": 50.5}
        )
        assert after.json()["currentAmount"] == 150.50

    async def test_a_goal_may_exceed_its_target(self, client: AsyncClient, auth: dict) -> None:
        created = (
            await client.post(
                "/api/goals", headers=auth, json={"name": "Small", "targetAmount": 10}
            )
        ).json()
        after = await client.post(
            f"/api/goals/{created['id']}/contribute", headers=auth, json={"amount": 999}
        )
        assert after.json()["currentAmount"] == 999

    async def test_rejects_a_zero_contribution(self, client: AsyncClient, auth: dict) -> None:
        created = (
            await client.post(
                "/api/goals", headers=auth, json={"name": "Trip", "targetAmount": 100}
            )
        ).json()
        response = await client.post(
            f"/api/goals/{created['id']}/contribute", headers=auth, json={"amount": 0}
        )
        assert response.status_code == 400


class TestRecurring:
    async def test_create_starts_on_the_start_date(
        self, client: AsyncClient, auth: dict, account: dict
    ) -> None:
        created = await client.post(
            "/api/recurring",
            headers=auth,
            json={
                "accountId": account["id"],
                "type": "expense",
                "amount": 9.99,
                "description": "Streaming",
                "frequency": "monthly",
                "startDate": "2026-09-01",
            },
        )
        assert created.status_code == 200
        body = created.json()
        assert body["nextRunDate"].startswith("2026-09-01")
        assert body["isActive"] is True

    async def test_transfers_are_not_supported(
        self, client: AsyncClient, auth: dict, account: dict
    ) -> None:
        response = await client.post(
            "/api/recurring",
            headers=auth,
            json={
                "accountId": account["id"],
                "type": "transfer",
                "amount": 10,
                "description": "Nope",
                "frequency": "monthly",
                "startDate": "2026-09-01",
            },
        )
        assert response.status_code == 400
        assert response.json()["detail"] == "Recurring transfers are not supported."

    async def test_end_date_must_not_precede_the_start(
        self, client: AsyncClient, auth: dict, account: dict
    ) -> None:
        response = await client.post(
            "/api/recurring",
            headers=auth,
            json={
                "accountId": account["id"],
                "type": "expense",
                "amount": 10,
                "description": "Backwards",
                "frequency": "monthly",
                "startDate": "2026-09-01",
                "endDate": "2026-08-01",
            },
        )
        assert response.status_code == 400
        assert response.json()["detail"] == "End date must not be before the start date."


class TestAnalytics:
    @pytest.fixture(autouse=True)
    async def _seed(self, client: AsyncClient, auth: dict, account: dict, category: dict) -> None:
        for payload in (
            {"type": "income", "amount": 3000, "date": "2026-08-01", "description": "Salary"},
            {
                "type": "expense",
                "amount": 750,
                "date": "2026-08-05",
                "description": "Rent",
                "categoryId": category["id"],
            },
        ):
            await client.post(
                "/api/transactions", headers=auth, json={"accountId": account["id"], **payload}
            )

    async def test_summary_shape(self, client: AsyncClient, auth: dict) -> None:
        body = (await client.get("/api/dashboard/summary", headers=auth)).json()
        assert set(body) == {"netWorth", "totalIncome", "totalExpenses", "savingsRate"}

    async def test_networth_window_length(self, client: AsyncClient, auth: dict) -> None:
        body = (await client.get("/api/dashboard/networth?months=3", headers=auth)).json()
        assert len(body) == 3
        assert set(body[0]) == {"month", "value"}

    async def test_months_are_clamped_not_rejected(self, client: AsyncClient, auth: dict) -> None:
        assert len((await client.get("/api/dashboard/networth?months=0", headers=auth)).json()) == 1
        assert (
            len((await client.get("/api/dashboard/networth?months=999", headers=auth)).json())
            == 120
        )

    async def test_a_bad_months_value_is_a_400(self, client: AsyncClient, auth: dict) -> None:
        response = await client.get("/api/dashboard/networth?months=abc", headers=auth)
        assert response.status_code == 400
        assert response.json()["errors"]["months"] == ["The value 'abc' is not valid for months."]

    async def test_cashflow_buckets(self, client: AsyncClient, auth: dict) -> None:
        body = (await client.get("/api/dashboard/cashflow?months=2", headers=auth)).json()
        assert len(body) == 2
        assert set(body[0]) == {"month", "income", "expenses"}

    async def test_spending_is_sorted_descending(self, client: AsyncClient, auth: dict) -> None:
        body = (await client.get("/api/dashboard/spending?month=2026-08", headers=auth)).json()
        amounts = [row["amount"] for row in body]
        assert amounts == sorted(amounts, reverse=True)

    async def test_spending_rejects_a_bad_month(self, client: AsyncClient, auth: dict) -> None:
        response = await client.get("/api/dashboard/spending?month=nope", headers=auth)
        assert response.status_code == 400
        assert response.json()["detail"] == "Month must be in YYYY-MM format."

    async def test_monthly_report_always_has_twelve_entries(
        self, client: AsyncClient, auth: dict
    ) -> None:
        body = (await client.get("/api/reports/monthly?year=2026", headers=auth)).json()
        assert len(body) == 12
        assert body[0]["month"] == "2026-01"
        assert body[11]["month"] == "2026-12"

    @pytest.mark.parametrize("year", [1800, 10000])
    async def test_monthly_report_rejects_an_out_of_range_year(
        self, client: AsyncClient, auth: dict, year: int
    ) -> None:
        response = await client.get(f"/api/reports/monthly?year={year}", headers=auth)
        assert response.status_code == 400
        assert response.json()["detail"] == "Year must be between 1900 and 9999."

    async def test_category_report(self, client: AsyncClient, auth: dict) -> None:
        body = (await client.get("/api/reports/categories", headers=auth)).json()
        assert body
        assert set(body[0]) == {"categoryId", "categoryName", "type", "color", "amount"}


class TestIsolation:
    async def test_one_user_cannot_see_another_s_data(
        self, client: AsyncClient, auth: dict, account: dict
    ) -> None:
        other = (
            await client.post(
                "/api/auth/register",
                json={
                    "email": f"other-{uuid.uuid4().hex[:8]}@test.dev",
                    "password": DEMO_PASSWORD,
                    "name": "Other",
                },
            )
        ).json()
        headers = {"Authorization": f"Bearer {other['token']}"}

        assert (await client.get("/api/accounts", headers=headers)).json() == []
        # And the first user's account is invisible, not merely absent.
        assert (
            await client.get(f"/api/accounts/{account['id']}", headers=headers)
        ).status_code == 404
