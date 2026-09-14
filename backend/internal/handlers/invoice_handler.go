package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"billing-app/backend/internal/models"
	"billing-app/backend/internal/service"
)

type InvoiceHandler struct {
	svc *service.InvoiceService
	log *slog.Logger
}

func NewInvoiceHandler(svc *service.InvoiceService, log *slog.Logger) *InvoiceHandler {
	return &InvoiceHandler{svc: svc, log: log}
}

func (h *InvoiceHandler) List(c *gin.Context) {
	p, ok := parsePagination(c)
	if !ok {
		return
	}

	filter := models.InvoiceFilter{
		Status:     models.InvoiceStatus(c.Query("status")),
		Pagination: p,
	}
	if raw := c.Query("customer_id"); raw != "" {
		customerID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || customerID <= 0 {
			writeError(c, http.StatusBadRequest, "invalid_request", "customer_id must be a positive integer", nil)
			return
		}
		filter.CustomerID = customerID
	}

	res, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *InvoiceHandler) Get(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	inv, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, inv)
}

func (h *InvoiceHandler) Create(c *gin.Context) {
	var req models.CreateInvoiceRequest
	if !bindJSON(c, &req) {
		return
	}
	inv, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, inv)
}

func (h *InvoiceHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req models.UpdateInvoiceRequest
	if !bindJSON(c, &req) {
		return
	}
	inv, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, inv)
}

func (h *InvoiceHandler) Delete(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}
