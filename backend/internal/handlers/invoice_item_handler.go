package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"billing-app/backend/internal/models"
	"billing-app/backend/internal/service"
)

type InvoiceItemHandler struct {
	svc *service.InvoiceItemService
	log *slog.Logger
}

func NewInvoiceItemHandler(svc *service.InvoiceItemService, log *slog.Logger) *InvoiceItemHandler {
	return &InvoiceItemHandler{svc: svc, log: log}
}

func (h *InvoiceItemHandler) List(c *gin.Context) {
	invoiceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	items, err := h.svc.List(c.Request.Context(), invoiceID)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *InvoiceItemHandler) Create(c *gin.Context) {
	invoiceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req models.CreateInvoiceItemRequest
	if !bindJSON(c, &req) {
		return
	}
	item, err := h.svc.Create(c.Request.Context(), invoiceID, req)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *InvoiceItemHandler) Update(c *gin.Context) {
	invoiceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	itemID, ok := parseIDParam(c, "itemId")
	if !ok {
		return
	}
	var req models.UpdateInvoiceItemRequest
	if !bindJSON(c, &req) {
		return
	}
	item, err := h.svc.Update(c.Request.Context(), invoiceID, itemID, req)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *InvoiceItemHandler) Delete(c *gin.Context) {
	invoiceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	itemID, ok := parseIDParam(c, "itemId")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), invoiceID, itemID); err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}
