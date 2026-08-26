package handlers_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/apptest"
	httpapi "github.com/pharaujo/finance/backend-go/internal/http"
	"github.com/pharaujo/finance/backend-go/internal/http/handlers"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/config"
	"github.com/pharaujo/finance/backend-go/internal/infrastructure/identity"
)

// resourceNow fixes the clock of the resource handlers, so "this month" and the
// export file name do not depend on the day the suite runs.
var resourceNow = time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)

// resources is the real router with every resource handler mounted on it,
// backed by the in-memory store.
type resources struct {
	engine *gin.Engine
	token  string
}

func newResources(t *testing.T) *resources {
	t.Helper()

	store := apptest.NewStore()
	tokens := identity.NewTokenService(config.LocalDevelopmentSecret)
	clock := func() time.Time { return resourceNow }

	categories := application.NewCategoryService(store.Categories())
	transactions := application.NewTransactionService(store.Transactions(), store.Accounts(), categories)

	accounts := handlers.NewAccounts(
		application.NewAccountService(store.Accounts(), store.Transactions()).WithClock(clock))
	categoriesHandler := handlers.NewCategories(categories)
	transactionsHandler := handlers.NewTransactions(
		transactions,
		application.NewTransactionCsvService(store.Transactions(), store.Accounts(), store.Categories()),
	).WithClock(clock)
	recurring := handlers.NewRecurring(
		application.NewRecurringService(store.Rules(), store.Transactions(), store.Accounts(), categories))
	budgets := handlers.NewBudgets(
		application.NewBudgetService(store.Budgets(), store.Transactions(), categories).WithClock(clock))
	goals := handlers.NewGoals(application.NewGoalService(store.Goals()))
	analytics := handlers.NewAnalytics(
		application.NewAnalyticsService(transactions, store.Accounts(), categories).WithClock(clock)).
		WithClock(clock)

	engine := httpapi.New(httpapi.Options{
		Tokens:         tokens,
		AllowedOrigins: []string{"http://localhost:5173"},
		Register: []httpapi.RegisterFunc{
			accounts.Routes,
			categoriesHandler.Routes,
			transactionsHandler.Routes,
			recurring.Routes,
			budgets.Routes,
			goals.Routes,
			analytics.Routes,
		},
	})

	userID := uuid.New()
	token, err := tokens.Issue(userID, "owner@example.com")
	if err != nil {
		t.Fatalf("issuing a token: %v", err)
	}

	return &resources{engine: engine, token: token}
}

// send drives one request as the authenticated caller.
func (r *resources) send(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	switch value := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case string:
		reader = bytes.NewReader([]byte(value))
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("encoding body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}

	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+r.token)

	recorder := httptest.NewRecorder()
	r.engine.ServeHTTP(recorder, request)
	return recorder
}

// object decodes a response body into a generic map, which is how the shape
// assertions inspect raw JSON keys and value kinds.
func object(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding %s: %v", response.Body, err)
	}
	return body
}

func array(t *testing.T, response *httptest.ResponseRecorder) []any {
	t.Helper()

	var body []any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding %s: %v", response.Body, err)
	}
	return body
}

// createAccount posts an account and returns its id.
func (r *resources) createAccount(t *testing.T, name, initialBalance string) string {
	t.Helper()

	response := r.send(t, http.MethodPost, "/api/accounts", map[string]any{
		"name":           name,
		"type":           "checking",
		"initialBalance": json.RawMessage(initialBalance),
		"currency":       "USD",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("creating an account: %d %s", response.Code, response.Body)
	}
	return object(t, response)["id"].(string)
}

func (r *resources) createCategory(t *testing.T, name, kind string) string {
	t.Helper()

	response := r.send(t, http.MethodPost, "/api/categories", map[string]any{
		"name":  name,
		"type":  kind,
		"icon":  "cup",
		"color": "#123456",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("creating a category: %d %s", response.Code, response.Body)
	}
	return object(t, response)["id"].(string)
}

// A create answers 200 rather than 201, and the body uses camelCase keys with
// the enum as a string and the money as a bare JSON number.
func TestAccountCreateShape(t *testing.T) {
	t.Parallel()

	api := newResources(t)
	response := api.send(t, http.MethodPost, "/api/accounts", map[string]any{
		"name":           "Everyday",
		"type":           "creditCard",
		"initialBalance": 1250.00,
		"currency":       "usd",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", response.Code, response.Body)
	}

	raw := response.Body.String()
	for _, want := range []string{
		`"type":"creditCard"`,
		`"isArchived":false`,
		`"currency":"USD"`,
		`"createdAt":"2026-08-26T12:00:00Z"`,
	} {
		if !bytes.Contains([]byte(raw), []byte(want)) {
			t.Errorf("body %s is missing %s", raw, want)
		}
	}

	body := object(t, response)
	if _, ok := body["balance"].(float64); !ok {
		t.Errorf("balance = %T, want a JSON number", body["balance"])
	}
}

func TestAccountLifecycleOverHTTP(t *testing.T) {
	t.Parallel()

	api := newResources(t)
	id := api.createAccount(t, "Everyday", "100.00")

	get := api.send(t, http.MethodGet, "/api/accounts/"+id, nil)
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d", get.Code)
	}

	update := api.send(t, http.MethodPut, "/api/accounts/"+id, map[string]any{
		"name":       "Renamed",
		"type":       "savings",
		"currency":   "USD",
		"isArchived": false,
	})
	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d, body %s", update.Code, update.Body)
	}
	if object(t, update)["name"] != "Renamed" {
		t.Errorf("name = %v, want Renamed", object(t, update)["name"])
	}

	archive := api.send(t, http.MethodDelete, "/api/accounts/"+id, nil)
	if archive.Code != http.StatusNoContent {
		t.Errorf("archive status = %d, want 204", archive.Code)
	}
	if archive.Body.Len() != 0 {
		t.Errorf("archive body = %s, want empty", archive.Body)
	}

	list := api.send(t, http.MethodGet, "/api/accounts", nil)
	if items := array(t, list); len(items) != 1 {
		t.Errorf("accounts = %d, want the archived one still listed", len(items))
	}
}

// The .NET routes constrain the id with {id:guid}, so a value that is not a
// uuid matches no route and the framework answers a bare 404.
func TestAnIdThatIsNotAGuidIsNotFound(t *testing.T) {
	t.Parallel()

	api := newResources(t)

	for _, path := range []string{
		"/api/accounts/not-a-guid",
		"/api/transactions/not-a-guid",
		"/api/categories/not-a-guid",
		"/api/budgets/not-a-guid",
		"/api/goals/not-a-guid",
		"/api/recurring/not-a-guid",
	} {
		response := api.send(t, http.MethodGet, path, nil)
		if response.Code != http.StatusNotFound && response.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s = %d, want 404", path, response.Code)
		}
	}

	deleted := api.send(t, http.MethodDelete, "/api/accounts/not-a-guid", nil)
	if deleted.Code != http.StatusNotFound {
		t.Errorf("DELETE with a bad id = %d, want 404", deleted.Code)
	}
	if deleted.Body.Len() != 0 {
		t.Errorf("body = %s, want the empty body a routing miss produces", deleted.Body)
	}
}

func TestMissingAccountIsAProblemDocument(t *testing.T) {
	t.Parallel()

	api := newResources(t)
	response := api.send(t, http.MethodGet, "/api/accounts/"+uuid.NewString(), nil)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("content type = %q", got)
	}

	body := object(t, response)
	if body["detail"] != "Account was not found." || body["title"] != "Not Found" {
		t.Errorf("problem = %v", body)
	}
}

func TestEveryResourceRequiresABearerToken(t *testing.T) {
	t.Parallel()

	api := newResources(t)
	for _, path := range []string{
		"/api/accounts",
		"/api/categories",
		"/api/transactions",
		"/api/recurring",
		"/api/budgets",
		"/api/goals",
		"/api/dashboard/summary",
		"/api/reports/monthly",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		api.engine.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without a token = %d, want 401", path, recorder.Code)
		}
	}
}

func TestTransactionSearchShape(t *testing.T) {
	t.Parallel()

	api := newResources(t)
	accountID := api.createAccount(t, "Wallet", "0.00")

	created := api.send(t, http.MethodPost, "/api/transactions", map[string]any{
		"accountId":   accountID,
		"type":        "expense",
		"amount":      25.5,
		"date":        "2026-08-17",
		"description": "Books",
		"tags":        []string{"reading"},
	})
	if created.Code != http.StatusOK {
		t.Fatalf("create status = %d, body %s", created.Code, created.Body)
	}
	if raw := created.Body.String(); !bytes.Contains([]byte(raw), []byte(`"amount":25.5`)) {
		t.Errorf("body %s does not render the amount as a bare number", raw)
	}

	page := api.send(t, http.MethodGet, "/api/transactions?page=1&pageSize=10", nil)
	if page.Code != http.StatusOK {
		t.Fatalf("search status = %d", page.Code)
	}

	body := object(t, page)
	if body["total"].(float64) != 1 || body["page"].(float64) != 1 || body["pageSize"].(float64) != 10 {
		t.Errorf("envelope = %v, want one item on page 1 of 10", body)
	}

	item := body["items"].([]any)[0].(map[string]any)
	if item["type"] != "expense" || item["description"] != "Books" {
		t.Errorf("item = %v", item)
	}
	if item["categoryId"] != nil || item["notes"] != nil || item["transferAccountId"] != nil {
		t.Errorf("item = %v, want nulls for the absent members", item)
	}
	if tags, ok := item["tags"].([]any); !ok || len(tags) != 1 || tags[0] != "reading" {
		t.Errorf("tags = %v", item["tags"])
	}
	if date, ok := item["date"].(string); !ok || date != "2026-08-17T00:00:00Z" {
		t.Errorf("date = %v, want a UTC instant", item["date"])
	}
}

// A bare calendar date is what the frontend's <input type="date"> posts, and
// System.Text.Json accepts it, so this API must too.
func TestTransactionAcceptsABareCalendarDate(t *testing.T) {
	t.Parallel()

	api := newResources(t)
	accountID := api.createAccount(t, "Wallet", "0.00")

	for _, value := range []string{"2026-08-17", "2026-08-17T10:30:00", "2026-08-17T10:30:00Z"} {
		response := api.send(t, http.MethodPost, "/api/transactions", map[string]any{
			"accountId":   accountID,
			"type":        "income",
			"amount":      1,
			"date":        value,
			"description": "Dated",
		})
		if response.Code != http.StatusOK {
			t.Errorf("date %q = %d, body %s", value, response.Code, response.Body)
		}
	}
}

func TestQueryValidationIsA400(t *testing.T) {
	t.Parallel()

	api := newResources(t)

	cases := []struct {
		path  string
		field string
	}{
		{"/api/transactions?page=0", "Page"},
		{"/api/transactions?pageSize=500", "PageSize"},
		{"/api/transactions?page=abc", "Page"},
		{"/api/transactions?accountId=nope", "AccountId"},
		{"/api/transactions?type=teleport", "Type"},
		{"/api/transactions?from=nope", "From"},
		{"/api/dashboard/networth?months=abc", "months"},
		{"/api/reports/monthly?year=abc", "year"},
		{"/api/reports/categories?from=nope", "from"},
	}

	for _, testCase := range cases {
		response := api.send(t, http.MethodGet, testCase.path, nil)
		if response.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400", testCase.path, response.Code)
			continue
		}

		body := object(t, response)
		errors, ok := body["errors"].(map[string]any)
		if !ok || errors[testCase.field] == nil {
			t.Errorf("GET %s errors = %v, want a %q key", testCase.path, body["errors"], testCase.field)
		}
	}
}

func TestTransactionCsvExportAndImport(t *testing.T) {
	t.Parallel()

	api := newResources(t)
	accountID := api.createAccount(t, "Import target", "0.00")

	created := api.send(t, http.MethodPost, "/api/transactions", map[string]any{
		"accountId":   accountID,
		"type":        "income",
		"amount":      10,
		"date":        "2026-08-17",
		"description": "Refund",
	})
	if created.Code != http.StatusOK {
		t.Fatalf("create status = %d, body %s", created.Code, created.Body)
	}

	export := api.send(t, http.MethodGet, "/api/transactions/export", nil)
	if export.Code != http.StatusOK {
		t.Fatalf("export status = %d", export.Code)
	}
	if got := export.Header().Get("Content-Type"); got != "text/csv" {
		t.Errorf("content type = %q, want text/csv", got)
	}
	if got := export.Header().Get("Content-Disposition"); got !=
		"attachment; filename=transactions-2026-08-26.csv; filename*=UTF-8''transactions-2026-08-26.csv" {
		t.Errorf("content disposition = %q", got)
	}

	csv := export.Body.String()
	if header := "Date,Type,Amount,Account,Category,Description,Notes,Tags"; csv[:len(header)] != header {
		t.Fatalf("csv starts with %q", csv[:len(header)])
	}

	response := api.upload(t, "file", "transactions.csv", csv)
	if response.Code != http.StatusOK {
		t.Fatalf("import status = %d, body %s", response.Code, response.Body)
	}

	body := object(t, response)
	if body["imported"].(float64) != 1 || body["skipped"].(float64) != 0 {
		t.Errorf("result = %v, want one imported", body)
	}
}

func TestImportRejectsAnEmptyUpload(t *testing.T) {
	t.Parallel()

	api := newResources(t)

	response := api.upload(t, "file", "empty.csv", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if object(t, response)["detail"] != "A non-empty CSV file is required." {
		t.Errorf("problem = %v", object(t, response))
	}

	// The wrong field name is the same failure: there is no file part.
	wrongField := api.upload(t, "upload", "transactions.csv", "Date,Type\n")
	if wrongField.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", wrongField.Code)
	}
}

// upload posts a multipart body with one file part.
func (r *resources) upload(t *testing.T, field, name, content string) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile(field, name)
	if err != nil {
		t.Fatalf("building the multipart body: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("writing the file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the multipart body: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/transactions/import", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+r.token)

	recorder := httptest.NewRecorder()
	r.engine.ServeHTTP(recorder, request)
	return recorder
}

func TestCategoryEndpoints(t *testing.T) {
	t.Parallel()

	api := newResources(t)
	id := api.createCategory(t, "Coffee", "expense")

	list := api.send(t, http.MethodGet, "/api/categories", nil)
	items := array(t, list)
	if len(items) != 1 {
		t.Fatalf("categories = %d, want 1", len(items))
	}
	if item := items[0].(map[string]any); item["type"] != "expense" || item["isDefault"] != false {
		t.Errorf("category = %v", item)
	}

	deleted := api.send(t, http.MethodDelete, "/api/categories/"+id, nil)
	if deleted.Code != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", deleted.Code)
	}
}

func TestBudgetEndpoints(t *testing.T) {
	t.Parallel()

	api := newResources(t)
	categoryID := api.createCategory(t, "Coffee", "expense")

	created := api.send(t, http.MethodPost, "/api/budgets", map[string]any{
		"categoryId": categoryID,
		"month":      "2026-08",
		"limit":      50,
	})
	if created.Code != http.StatusOK {
		t.Fatalf("create status = %d, body %s", created.Code, created.Body)
	}
	budgetID := object(t, created)["id"].(string)

	duplicate := api.send(t, http.MethodPost, "/api/budgets", map[string]any{
		"categoryId": categoryID,
		"month":      "2026-08",
		"limit":      50,
	})
	if duplicate.Code != http.StatusConflict {
		t.Errorf("duplicate status = %d, want 409", duplicate.Code)
	}
	if object(t, duplicate)["detail"] != "A budget already exists for that category and month." {
		t.Errorf("problem = %v", object(t, duplicate))
	}

	// An omitted month means the current one, which the fixed clock puts in
	// August 2026.
	list := api.send(t, http.MethodGet, "/api/budgets", nil)
	if items := array(t, list); len(items) != 1 {
		t.Errorf("budgets = %d, want the current month's", len(items))
	}

	other := api.send(t, http.MethodGet, "/api/budgets?month=2026-09", nil)
	if items := array(t, other); len(items) != 0 {
		t.Errorf("budgets = %d, want none in September", len(items))
	}

	badMonth := api.send(t, http.MethodGet, "/api/budgets?month=nope", nil)
	if badMonth.Code != http.StatusBadRequest {
		t.Errorf("bad month status = %d, want 400", badMonth.Code)
	}

	updated := api.send(t, http.MethodPut, "/api/budgets/"+budgetID, map[string]any{"limit": 75})
	if updated.Code != http.StatusOK {
		t.Fatalf("update status = %d, body %s", updated.Code, updated.Body)
	}

	deleted := api.send(t, http.MethodDelete, "/api/budgets/"+budgetID, nil)
	if deleted.Code != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", deleted.Code)
	}
}

func TestGoalEndpointsIncludingContribute(t *testing.T) {
	t.Parallel()

	api := newResources(t)

	created := api.send(t, http.MethodPost, "/api/goals", map[string]any{
		"name":         "New bike",
		"targetAmount": 1200,
		"targetDate":   "2027-03-01",
		"color":        "#ff8800",
	})
	if created.Code != http.StatusOK {
		t.Fatalf("create status = %d, body %s", created.Code, created.Body)
	}
	goalID := object(t, created)["id"].(string)

	contributed := api.send(t, http.MethodPost, "/api/goals/"+goalID+"/contribute",
		map[string]any{"amount": 25.5})
	if contributed.Code != http.StatusOK {
		t.Fatalf("contribute status = %d, body %s", contributed.Code, contributed.Body)
	}
	if raw := contributed.Body.String(); !bytes.Contains([]byte(raw), []byte(`"currentAmount":25.5`)) {
		t.Errorf("body %s, want the contribution reflected", raw)
	}

	rejected := api.send(t, http.MethodPost, "/api/goals/"+goalID+"/contribute",
		map[string]any{"amount": 0})
	if rejected.Code != http.StatusBadRequest {
		t.Errorf("zero contribution = %d, want 400", rejected.Code)
	}

	deleted := api.send(t, http.MethodDelete, "/api/goals/"+goalID, nil)
	if deleted.Code != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", deleted.Code)
	}
}

func TestRecurringEndpoints(t *testing.T) {
	t.Parallel()

	api := newResources(t)
	accountID := api.createAccount(t, "Checking", "0.00")

	created := api.send(t, http.MethodPost, "/api/recurring", map[string]any{
		"accountId":   accountID,
		"type":        "expense",
		"amount":      12,
		"description": "Streaming",
		"frequency":   "monthly",
		"startDate":   "2026-09-01",
	})
	if created.Code != http.StatusOK {
		t.Fatalf("create status = %d, body %s", created.Code, created.Body)
	}

	body := object(t, created)
	if body["frequency"] != "monthly" || body["type"] != "expense" || body["isActive"] != true {
		t.Errorf("rule = %v", body)
	}
	if body["nextRunDate"] != "2026-09-01T00:00:00Z" {
		t.Errorf("nextRunDate = %v", body["nextRunDate"])
	}
	if body["endDate"] != nil {
		t.Errorf("endDate = %v, want null", body["endDate"])
	}

	transfer := api.send(t, http.MethodPost, "/api/recurring", map[string]any{
		"accountId":   accountID,
		"type":        "transfer",
		"amount":      12,
		"description": "Move",
		"frequency":   "monthly",
		"startDate":   "2026-09-01",
	})
	if transfer.Code != http.StatusBadRequest {
		t.Errorf("recurring transfer = %d, want 400", transfer.Code)
	}

	deleted := api.send(t, http.MethodDelete, "/api/recurring/"+body["id"].(string), nil)
	if deleted.Code != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", deleted.Code)
	}
}

func TestDashboardAndReportShapes(t *testing.T) {
	t.Parallel()

	api := newResources(t)
	accountID := api.createAccount(t, "Checking", "500.00")

	for _, payload := range []map[string]any{
		{"type": "income", "amount": 2000, "date": "2026-08-01", "description": "Salary"},
		{"type": "expense", "amount": 500, "date": "2026-08-02", "description": "Rent"},
	} {
		payload["accountId"] = accountID
		if response := api.send(t, http.MethodPost, "/api/transactions", payload); response.Code != http.StatusOK {
			t.Fatalf("seeding a transaction: %d %s", response.Code, response.Body)
		}
	}

	summary := api.send(t, http.MethodGet, "/api/dashboard/summary", nil)
	if summary.Code != http.StatusOK {
		t.Fatalf("summary status = %d", summary.Code)
	}
	raw := summary.Body.String()
	for _, want := range []string{
		`"netWorth":2000.00`,
		`"totalIncome":2000.00`,
		`"totalExpenses":500.00`,
		`"savingsRate":0.75`,
	} {
		if !bytes.Contains([]byte(raw), []byte(want)) {
			t.Errorf("summary %s is missing %s", raw, want)
		}
	}

	networth := api.send(t, http.MethodGet, "/api/dashboard/networth?months=3", nil)
	if points := array(t, networth); len(points) != 3 {
		t.Errorf("networth points = %d, want 3", len(points))
	}

	// The default windows come from the controller's parameter defaults.
	defaulted := api.send(t, http.MethodGet, "/api/dashboard/networth", nil)
	if points := array(t, defaulted); len(points) != 12 {
		t.Errorf("networth points = %d, want the default 12", len(points))
	}
	cashflow := api.send(t, http.MethodGet, "/api/dashboard/cashflow", nil)
	if points := array(t, cashflow); len(points) != 6 {
		t.Errorf("cashflow points = %d, want the default 6", len(points))
	}

	spending := api.send(t, http.MethodGet, "/api/dashboard/spending", nil)
	if slices := array(t, spending); len(slices) != 1 {
		t.Errorf("spending slices = %d, want 1", len(slices))
	}

	monthly := api.send(t, http.MethodGet, "/api/reports/monthly", nil)
	months := array(t, monthly)
	if len(months) != 12 {
		t.Fatalf("months = %d, want 12", len(months))
	}
	if august := months[7].(map[string]any); august["month"] != "2026-08" {
		t.Errorf("eighth month = %v, want 2026-08", august["month"])
	}

	badYear := api.send(t, http.MethodGet, "/api/reports/monthly?year=1800", nil)
	if badYear.Code != http.StatusBadRequest {
		t.Errorf("year 1800 = %d, want 400", badYear.Code)
	}

	categories := api.send(t, http.MethodGet, "/api/reports/categories", nil)
	if rows := array(t, categories); len(rows) != 1 {
		t.Errorf("category rows = %d, want the uncategorized group", len(rows))
	}
}

func TestBodyValidationIsA400WithFieldErrors(t *testing.T) {
	t.Parallel()

	api := newResources(t)

	response := api.send(t, http.MethodPost, "/api/accounts", map[string]any{"type": "checking"})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}

	body := object(t, response)
	if body["title"] != "One or more validation errors occurred." {
		t.Errorf("title = %v", body["title"])
	}
	errors := body["errors"].(map[string]any)
	if errors["Name"] == nil {
		t.Errorf("errors = %v, want a Name key", errors)
	}

	unparsable := api.send(t, http.MethodPost, "/api/accounts", "{not json")
	if unparsable.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", unparsable.Code)
	}

	badEnum := api.send(t, http.MethodPost, "/api/accounts", map[string]any{
		"name": "Wallet", "type": "teleport", "currency": "USD",
	})
	if badEnum.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for an unknown enum name", badEnum.Code)
	}
}

func TestListEndpointsRenderEmptyArrays(t *testing.T) {
	t.Parallel()

	api := newResources(t)

	for _, path := range []string{
		"/api/accounts",
		"/api/categories",
		"/api/recurring",
		"/api/budgets",
		"/api/goals",
		"/api/dashboard/spending",
		"/api/reports/categories",
	} {
		response := api.send(t, http.MethodGet, path, nil)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, response.Code)
		}
		if response.Body.String() != "[]" {
			t.Errorf("GET %s = %s, want []", path, response.Body)
		}
	}
}

// The update paths of every resource, which the .NET controllers expose as PUT
// on the same route as GET.
func TestUpdateEndpoints(t *testing.T) {
	t.Parallel()

	api := newResources(t)
	accountID := api.createAccount(t, "Checking", "0.00")
	categoryID := api.createCategory(t, "Coffee", "expense")

	category := api.send(t, http.MethodPut, "/api/categories/"+categoryID, map[string]any{
		"name": "Espresso", "type": "expense", "icon": "cup", "color": "#000000",
	})
	if category.Code != http.StatusOK || object(t, category)["name"] != "Espresso" {
		t.Errorf("category update = %d %s", category.Code, category.Body)
	}

	created := api.send(t, http.MethodPost, "/api/transactions", map[string]any{
		"accountId": accountID, "type": "expense", "amount": 5,
		"date": "2026-08-01", "description": "Beans",
	})
	transactionID := object(t, created)["id"].(string)

	updated := api.send(t, http.MethodPut, "/api/transactions/"+transactionID, map[string]any{
		"accountId": accountID, "categoryId": categoryID, "type": "income", "amount": 7.25,
		"date": "2026-08-02", "description": "Refund", "tags": []string{"tax"},
	})
	if updated.Code != http.StatusOK {
		t.Fatalf("transaction update = %d %s", updated.Code, updated.Body)
	}
	if body := object(t, updated); body["type"] != "income" || body["categoryId"] != categoryID {
		t.Errorf("transaction = %v", body)
	}

	deleted := api.send(t, http.MethodDelete, "/api/transactions/"+transactionID, nil)
	if deleted.Code != http.StatusNoContent {
		t.Errorf("transaction delete = %d, want 204", deleted.Code)
	}

	goal := api.send(t, http.MethodPost, "/api/goals", map[string]any{
		"name": "Trip", "targetAmount": 500,
	})
	goalID := object(t, goal)["id"].(string)

	goalUpdate := api.send(t, http.MethodPut, "/api/goals/"+goalID, map[string]any{
		"name": "Longer trip", "targetAmount": 900, "currentAmount": 100,
	})
	if goalUpdate.Code != http.StatusOK || object(t, goalUpdate)["name"] != "Longer trip" {
		t.Errorf("goal update = %d %s", goalUpdate.Code, goalUpdate.Body)
	}

	rule := api.send(t, http.MethodPost, "/api/recurring", map[string]any{
		"accountId": accountID, "type": "expense", "amount": 12,
		"description": "Streaming", "frequency": "monthly", "startDate": "2026-09-01",
	})
	ruleID := object(t, rule)["id"].(string)

	ruleUpdate := api.send(t, http.MethodPut, "/api/recurring/"+ruleID, map[string]any{
		"accountId": accountID, "type": "expense", "amount": 15,
		"description": "Streaming", "frequency": "weekly", "startDate": "2026-10-01",
		"isActive": false,
	})
	if ruleUpdate.Code != http.StatusOK {
		t.Fatalf("rule update = %d %s", ruleUpdate.Code, ruleUpdate.Body)
	}
	if body := object(t, ruleUpdate); body["frequency"] != "weekly" || body["isActive"] != false {
		t.Errorf("rule = %v", body)
	}
}
