# frozen_string_literal: true

require "rails_helper"

# End-to-end through the real Rails stack against a throwaway Postgres schema.
RSpec.describe "API", type: :request do
  MISSING = "00000000-0000-0000-0000-000000000000"
  PASSWORD = "Passw0rd!123"

  def json = JSON.parse(response.body)

  def register(email)
    post "/api/auth/register", params: { email: email, password: PASSWORD, name: "Harness" }.to_json,
                               headers: { "CONTENT_TYPE" => "application/json" }
    expect(response).to have_http_status(:ok)
    json["token"]
  end

  def auth = { "Authorization" => "Bearer #{@token}", "CONTENT_TYPE" => "application/json" }

  def post_json(path, payload) = post(path, params: payload.to_json, headers: auth)
  def put_json(path, payload) = put(path, params: payload.to_json, headers: auth)

  def make_account(**overrides)
    post_json("/api/accounts",
              { name: "Checking", type: "checking", initialBalance: 1000 }.merge(overrides))
    expect(response).to have_http_status(:ok)
    json
  end

  def make_category
    post_json("/api/categories",
              { name: "Food", type: "expense", icon: "utensils", color: "#ef4444" })
    expect(response).to have_http_status(:ok)
    json
  end

  before { @token = register("harness@test.dev") }

  describe "probes" do
    it "answers health as plain text" do
      get "/health"
      expect(response).to have_http_status(:ok)
      expect(response.body).to eq("ok")
    end

    it "names this backend in the service document" do
      get "/"
      expect(json).to eq({ "service" => "FinanceTracker API (Rails)", "status" => "ok",
                           "docs" => "/swagger" })
    end
  end

  describe "auth" do
    it "normalises the address and trims the name on register" do
      post "/api/auth/register",
           params: { email: "New@Example.COM ", password: PASSWORD, name: " Ada " }.to_json,
           headers: { "CONTENT_TYPE" => "application/json" }
      expect(json["user"]).to include("email" => "new@example.com", "name" => "Ada",
                                      "currency" => "USD")
    end

    it "rejects a duplicate email" do
      post "/api/auth/register",
           params: { email: "harness@test.dev", password: PASSWORD, name: "Twin" }.to_json,
           headers: { "CONTENT_TYPE" => "application/json" }
      expect(response).to have_http_status(:conflict)
      expect(json["detail"]).to eq("An account with that email already exists.")
    end

    it "round-trips a login" do
      post "/api/auth/login", params: { email: "harness@test.dev", password: PASSWORD }.to_json,
                              headers: { "CONTENT_TYPE" => "application/json" }
      expect(json["user"]["email"]).to eq("harness@test.dev")
    end

    [ { email: "harness@test.dev", password: "wrong-password" },
      { email: "nobody@test.dev", password: PASSWORD } ].each do |payload|
      it "fails bad credentials indistinguishably (#{payload[:email]})" do
        post "/api/auth/login", params: payload.to_json,
                                headers: { "CONTENT_TYPE" => "application/json" }
        expect(response).to have_http_status(:unauthorized)
        expect(json["detail"]).to eq("Invalid email or password.")
      end
    end

    it "requires a token for the profile" do
      get "/api/auth/me"
      expect(response).to have_http_status(:unauthorized)
      expect(response.headers["WWW-Authenticate"]).to eq("Bearer")
    end

    it "rejects a forged token" do
      get "/api/auth/me", headers: { "Authorization" => "Bearer not.a.token" }
      expect(response).to have_http_status(:unauthorized)
    end

    it "trims the name and upper-cases the currency on update" do
      put_json("/api/auth/me", { name: " Grace ", currency: " eur " })
      expect(json).to include("name" => "Grace", "currency" => "EUR")
    end

    it "reports every broken rule at once" do
      post "/api/auth/register", params: { email: "bad", password: "x", name: "" }.to_json,
                                 headers: { "CONTENT_TYPE" => "application/json" }
      expect(response).to have_http_status(:bad_request)
      expect(json["errors"]["Email"]).to eq([ "The Email field is not a valid e-mail address." ])
      expect(json["errors"]["Name"]).to include("The Name field is required.")
    end
  end

  describe "accounts" do
    it "creates and lists" do
      account = make_account
      expect(account["balance"]).to eq(1000)

      get "/api/accounts", headers: auth
      expect(json.map { |item| item["id"] }).to eq([ account["id"] ])
    end

    it "follows the transactions in the balance" do
      account = make_account
      post_json("/api/transactions", { accountId: account["id"], type: "expense", amount: 42.50,
                                       date: "2026-08-10", description: "Lunch" })

      get "/api/accounts/#{account['id']}", headers: auth
      expect(json["balance"]).to eq(957.50)
    end

    it "moves money between two accounts on a transfer" do
      checking = make_account
      savings = make_account(name: "Savings", type: "savings", initialBalance: 0)

      post_json("/api/transactions", { accountId: checking["id"], type: "transfer", amount: 250,
                                       date: "2026-08-15", description: "To savings",
                                       transferAccountId: savings["id"] })

      get "/api/accounts", headers: auth
      balances = json.to_h { |item| [ item["name"], item["balance"] ] }
      expect(balances["Checking"]).to eq(750)
      expect(balances["Savings"]).to eq(250)
    end

    it "archives rather than removing on delete" do
      account = make_account
      delete "/api/accounts/#{account['id']}", headers: auth
      expect(response).to have_http_status(:no_content)

      get "/api/accounts/#{account['id']}", headers: auth
      expect(json["isArchived"]).to be(true)
    end

    it "renders an unknown account as a problem document" do
      get "/api/accounts/#{MISSING}", headers: auth
      expect(response).to have_http_status(:not_found)
      expect(response.headers["Content-Type"]).to include("application/problem+json")
      expect(json["detail"]).to eq("Account was not found.")
    end

    it "answers a bare 404 for a non-uuid path" do
      get "/api/accounts/not-a-uuid", headers: auth
      expect(response).to have_http_status(:not_found)
      expect(response.body).to eq("")
    end

    it "short-circuits on a missing required member" do
      post_json("/api/accounts", { name: "" })
      expect(response).to have_http_status(:bad_request)
      # Only the "$" error: no other rule runs once a member is missing.
      expect(json["errors"]).to eq(
        { "$" => [ "The JSON payload was missing required properties, including the following: type" ] }
      )
    end
  end

  describe "categories" do
    it "shows the shared defaults" do
      get "/api/categories", headers: auth
      expect(json.any? { |item| item["isDefault"] }).to be(true)
    end

    it "creates and updates" do
      category = make_category
      expect(category["isDefault"]).to be(false)

      put_json("/api/categories/#{category['id']}",
               { name: "Dining", type: "expense", icon: "fork", color: "#000000" })
      expect(json["name"]).to eq("Dining")
    end

    it "refuses to modify a default category" do
      get "/api/categories", headers: auth
      shared = json.find { |item| item["isDefault"] }

      delete "/api/categories/#{shared['id']}", headers: auth
      expect(response).to have_http_status(:bad_request)
      expect(json["detail"]).to eq("Default categories cannot be modified.")
    end

    it "detaches a deleted category from its transactions" do
      account = make_account
      category = make_category
      post_json("/api/transactions", { accountId: account["id"], categoryId: category["id"],
                                       type: "expense", amount: 10, date: "2026-08-10",
                                       description: "Snack" })
      created = json

      delete "/api/categories/#{category['id']}", headers: auth
      expect(response).to have_http_status(:no_content)

      get "/api/transactions/#{created['id']}", headers: auth
      expect(json["categoryId"]).to be_nil
    end
  end

  describe "transactions" do
    it "normalises the row it stores" do
      account = make_account
      post_json("/api/transactions", { accountId: account["id"], type: "expense", amount: 42.505,
                                       date: "2026-08-10", description: "  Lunch  ", notes: "   ",
                                       tags: [ " food ", "", "out" ] })

      expect(json["amount"]).to eq(42.51) # rounded half away from zero
      expect(json["description"]).to eq("Lunch")
      expect(json["notes"]).to be_nil     # whitespace collapses to null
      expect(json["tags"]).to eq(%w[food out])
    end

    it "pages and filters the search" do
      account = make_account
      3.times do |index|
        post_json("/api/transactions", { accountId: account["id"], type: "expense",
                                         amount: 5 + index, date: "2026-08-1#{index}",
                                         description: "Item #{index}" })
      end

      get "/api/transactions?page=1&pageSize=2", headers: auth
      expect(json).to include("total" => 3, "page" => 1, "pageSize" => 2)
      expect(json["items"].length).to eq(2)

      get "/api/transactions?search=item%201", headers: auth
      expect(json["total"]).to eq(1)
    end

    it "requires a different destination for a transfer" do
      account = make_account
      payload = { accountId: account["id"], type: "transfer", amount: 10,
                  date: "2026-08-10", description: "Nowhere" }

      post_json("/api/transactions", payload)
      expect(json["detail"]).to eq("A transfer requires a destination account.")

      post_json("/api/transactions", payload.merge(transferAccountId: account["id"]))
      expect(json["detail"]).to eq("A transfer must use two different accounts.")
    end

    it "updates and deletes" do
      account = make_account
      post_json("/api/transactions", { accountId: account["id"], type: "expense", amount: 10,
                                       date: "2026-08-10", description: "Before" })
      created = json

      put_json("/api/transactions/#{created['id']}",
               { accountId: account["id"], type: "income", amount: 20, date: "2026-08-11",
                 description: "After" })
      expect(json).to include("description" => "After", "type" => "income")

      delete "/api/transactions/#{created['id']}", headers: auth
      expect(response).to have_http_status(:no_content)

      get "/api/transactions/#{created['id']}", headers: auth
      expect(response).to have_http_status(:not_found)
    end

    it "exports as CSV" do
      account = make_account
      make_category
      post_json("/api/transactions", { accountId: account["id"], type: "expense", amount: 12.5,
                                       date: "2026-08-10", description: "Groceries, bulk" })

      get "/api/transactions/export", headers: auth
      expect(response).to have_http_status(:ok)
      expect(response.headers["Content-Type"]).to include("text/csv")
      expect(response.headers["Content-Disposition"]).to include("attachment; filename=transactions-")

      lines = response.body.split("\n")
      expect(lines[0]).to eq("Date,Type,Amount,Account,Category,Description,Notes,Tags")
      # The description carries a comma, so it comes back quoted, and the CSV
      # column always writes two places.
      expect(lines[1]).to include(%("Groceries, bulk"))
      expect(lines[1]).to include(",12.50,")
    end
  end

  describe "budgets" do
    it "measures the month spend" do
      account = make_account
      category = make_category
      post_json("/api/transactions", { accountId: account["id"], categoryId: category["id"],
                                       type: "expense", amount: 30, date: "2026-08-10",
                                       description: "Food" })

      post_json("/api/budgets", { categoryId: category["id"], month: "2026-08", limit: 100 })
      expect(json["spent"]).to eq(30)
      expect(json["remaining"]).to eq(70)
    end

    it "rejects a duplicate" do
      category = make_category
      payload = { categoryId: category["id"], month: "2026-08", limit: 100 }
      post_json("/api/budgets", payload)
      expect(response).to have_http_status(:ok)

      post_json("/api/budgets", payload)
      expect(response).to have_http_status(:conflict)
      expect(json["detail"]).to eq("A budget already exists for that category and month.")
    end

    %w[nope 2026-13].each do |month|
      it "rejects the month #{month}" do
        category = make_category
        post_json("/api/budgets", { categoryId: category["id"], month: month, limit: 100 })
        expect(response).to have_http_status(:bad_request)
      end
    end

    it "moves only the limit on update" do
      category = make_category
      post_json("/api/budgets", { categoryId: category["id"], month: "2026-08", limit: 100 })
      created = json

      put_json("/api/budgets/#{created['id']}", { limit: 250 })
      expect(json).to include("limit" => 250, "month" => "2026-08")
    end
  end

  describe "goals" do
    it "creates and contributes" do
      post_json("/api/goals", { name: "Trip", targetAmount: 2000, currentAmount: 100 })
      created = json
      expect(created["currentAmount"]).to eq(100)

      post_json("/api/goals/#{created['id']}/contribute", { amount: 50.5 })
      expect(json["currentAmount"]).to eq(150.50)
    end

    it "lets a goal exceed its target" do
      post_json("/api/goals", { name: "Small", targetAmount: 10 })
      post_json("/api/goals/#{json['id']}/contribute", { amount: 999 })
      expect(json["currentAmount"]).to eq(999)
    end

    it "rejects a zero contribution" do
      post_json("/api/goals", { name: "Trip", targetAmount: 100 })
      post_json("/api/goals/#{json['id']}/contribute", { amount: 0 })
      expect(response).to have_http_status(:bad_request)
    end
  end

  describe "recurring" do
    it "starts on the start date" do
      account = make_account
      post_json("/api/recurring", { accountId: account["id"], type: "expense", amount: 9.99,
                                    description: "Streaming", frequency: "monthly",
                                    startDate: "2026-09-01" })
      expect(json["nextRunDate"]).to include("2026-09-01")
      expect(json["isActive"]).to be(true)
    end

    it "does not support transfers" do
      account = make_account
      post_json("/api/recurring", { accountId: account["id"], type: "transfer", amount: 10,
                                    description: "Nope", frequency: "monthly",
                                    startDate: "2026-09-01" })
      expect(response).to have_http_status(:bad_request)
      expect(json["detail"]).to eq("Recurring transfers are not supported.")
    end

    it "refuses an end date before the start" do
      account = make_account
      post_json("/api/recurring", { accountId: account["id"], type: "expense", amount: 10,
                                    description: "Backwards", frequency: "monthly",
                                    startDate: "2026-09-01", endDate: "2026-08-01" })
      expect(json["detail"]).to eq("End date must not be before the start date.")
    end
  end

  describe "analytics" do
    before do
      account = make_account
      category = make_category
      post_json("/api/transactions", { accountId: account["id"], type: "income", amount: 3000,
                                       date: "2026-08-01", description: "Salary" })
      post_json("/api/transactions", { accountId: account["id"], categoryId: category["id"],
                                       type: "expense", amount: 750, date: "2026-08-05",
                                       description: "Rent" })
    end

    it "returns the summary shape" do
      get "/api/dashboard/summary", headers: auth
      expect(json.keys.sort).to eq(%w[netWorth savingsRate totalExpenses totalIncome])
    end

    it "returns a window of the requested length" do
      get "/api/dashboard/networth?months=3", headers: auth
      expect(json.length).to eq(3)
      expect(json.first.keys.sort).to eq(%w[month value])
    end

    it "clamps the window rather than rejecting it" do
      get "/api/dashboard/networth?months=0", headers: auth
      expect(json.length).to eq(1)
      get "/api/dashboard/networth?months=999", headers: auth
      expect(json.length).to eq(120)
    end

    it "rejects a non-numeric months value" do
      get "/api/dashboard/networth?months=abc", headers: auth
      expect(response).to have_http_status(:bad_request)
      expect(json["errors"]["months"]).to eq([ "The value 'abc' is not valid for months." ])
    end

    it "buckets the cashflow" do
      get "/api/dashboard/cashflow?months=2", headers: auth
      expect(json.length).to eq(2)
      expect(json.first.keys.sort).to eq(%w[expenses income month])
    end

    it "sorts spending descending" do
      get "/api/dashboard/spending?month=2026-08", headers: auth
      amounts = json.map { |row| row["amount"] }
      expect(amounts).to eq(amounts.sort.reverse)
    end

    it "rejects a bad spending month" do
      get "/api/dashboard/spending?month=nope", headers: auth
      expect(response).to have_http_status(:bad_request)
      expect(json["detail"]).to eq("Month must be in YYYY-MM format.")
    end

    it "always returns twelve months in the yearly report" do
      get "/api/reports/monthly?year=2026", headers: auth
      expect(json.length).to eq(12)
      expect(json.first["month"]).to eq("2026-01")
      expect(json.last["month"]).to eq("2026-12")
    end

    [ 1800, 10_000 ].each do |year|
      it "rejects the out-of-range year #{year}" do
        get "/api/reports/monthly?year=#{year}", headers: auth
        expect(response).to have_http_status(:bad_request)
        expect(json["detail"]).to eq("Year must be between 1900 and 9999.")
      end
    end

    it "returns the category report shape" do
      get "/api/reports/categories", headers: auth
      expect(json.first.keys.sort).to eq(%w[amount categoryId categoryName color type])
    end
  end

  describe "isolation" do
    it "hides one user's data from another" do
      account = make_account
      other = register("other-#{SecureRandom.hex(4)}@test.dev")
      other_auth = { "Authorization" => "Bearer #{other}" }

      get "/api/accounts", headers: other_auth
      expect(json).to eq([])

      # And the first user's account is invisible, not merely absent.
      get "/api/accounts/#{account['id']}", headers: other_auth
      expect(response).to have_http_status(:not_found)
    end
  end
end
