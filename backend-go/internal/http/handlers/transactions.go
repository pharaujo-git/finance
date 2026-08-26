package handlers

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pharaujo/finance/backend-go/internal/application"
	"github.com/pharaujo/finance/backend-go/internal/domain"
	"github.com/pharaujo/finance/backend-go/internal/http/middleware"
)

// maxUploadBytes is the 5 MB cap TransactionsController.Import enforces.
const maxUploadBytes int64 = 5 * 1024 * 1024

// The two 400s an unusable upload raises, byte for byte.
const (
	missingUploadMessage = "A non-empty CSV file is required."
	oversizeMessage      = "The uploaded file is larger than 5 MB."
)

// csvContentType is what the .NET File(...) result sets; the charset is left
// off there, so it is left off here too.
const csvContentType = "text/csv"

// Transactions serves /api/transactions, CSV export and import included.
type Transactions struct {
	service *application.TransactionService
	csv     *application.TransactionCsvService

	now func() time.Time
}

// NewTransactions builds the handler around the two transaction services.
func NewTransactions(
	service *application.TransactionService,
	csv *application.TransactionCsvService,
) *Transactions {
	return &Transactions{service: service, csv: csv, now: func() time.Time { return time.Now().UTC() }}
}

// WithClock returns a copy that stamps the export file name from now.
func (h *Transactions) WithClock(now func() time.Time) *Transactions {
	clone := *h
	clone.now = now
	return &clone
}

// Routes registers the endpoints of TransactionsController. The two literal
// segments are registered before the parameterised one so gin matches
// /export and /import as routes of their own.
func (h *Transactions) Routes(api *gin.RouterGroup) {
	transactions := api.Group("/transactions")
	transactions.GET("", h.Search)
	transactions.POST("", h.Create)
	transactions.GET("/export", h.Export)
	transactions.POST("/import", h.Import)
	transactions.GET("/:id", h.Get)
	transactions.PUT("/:id", h.Update)
	transactions.DELETE("/:id", h.Delete)
}

// Search handles GET /api/transactions.
func (h *Transactions) Search(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	reader := newQueryReader(c)
	query := application.TransactionQuery{
		Page:       reader.number("page", application.FieldPage),
		PageSize:   reader.number("pageSize", application.FieldPageSize),
		AccountID:  reader.identifier("accountId", "AccountId"),
		CategoryID: reader.identifier("categoryId", "CategoryId"),
		Type:       reader.transactionType("type", "Type"),
		From:       reader.moment("from", "From"),
		To:         reader.moment("to", "To"),
	}
	if search := reader.text("search"); search != nil {
		query.Search = *search
	}
	if !reader.ok() {
		return
	}

	page, err := h.service.Search(c.Request.Context(), userID, query)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

// Get handles GET /api/transactions/{id}.
func (h *Transactions) Get(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}

	transaction, err := h.service.Get(c.Request.Context(), userID, id)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, transaction)
}

// Create handles POST /api/transactions.
func (h *Transactions) Create(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	var request application.TransactionRequest
	if !bindJSON(c, &request) {
		return
	}

	transaction, err := h.service.Create(c.Request.Context(), userID, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, transaction)
}

// Update handles PUT /api/transactions/{id}.
func (h *Transactions) Update(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}

	var request application.TransactionRequest
	if !bindJSON(c, &request) {
		return
	}

	transaction, err := h.service.Update(c.Request.Context(), userID, id, request)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, transaction)
}

// Delete handles DELETE /api/transactions/{id}.
func (h *Transactions) Delete(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), userID, id); err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Export handles GET /api/transactions/export, returning a CSV download.
func (h *Transactions) Export(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	reader := newQueryReader(c)
	from := reader.moment("from", "from")
	to := reader.moment("to", "to")
	if !reader.ok() {
		return
	}

	content, err := h.csv.Export(c.Request.Context(), userID, from, to)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}

	name := application.ExportFileName(h.now())
	c.Header("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s; filename*=UTF-8''%s", name, name))
	c.Data(http.StatusOK, csvContentType, []byte(content))
}

// Import handles POST /api/transactions/import, a multipart upload whose file
// part is named "file".
func (h *Transactions) Import(c *gin.Context) {
	userID, ok := caller(c)
	if !ok {
		return
	}

	header, err := c.FormFile("file")
	if err != nil || header.Size == 0 {
		middleware.WriteAppError(c, domain.BadRequest(missingUploadMessage))
		return
	}
	if header.Size > maxUploadBytes {
		middleware.WriteAppError(c, domain.BadRequest(oversizeMessage))
		return
	}

	file, err := header.Open()
	if err != nil {
		middleware.WriteAppError(c, domain.BadRequest(missingUploadMessage))
		return
	}
	defer func() { _ = file.Close() }()

	content, err := io.ReadAll(io.LimitReader(file, maxUploadBytes))
	if err != nil {
		middleware.WriteAppError(c, domain.BadRequest(missingUploadMessage))
		return
	}

	result, err := h.csv.Import(c.Request.Context(), userID, content)
	if err != nil {
		middleware.WriteAppError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
