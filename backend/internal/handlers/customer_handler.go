package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"billing-app/backend/internal/models"
	"billing-app/backend/internal/service"
)

type CustomerHandler struct {
	svc *service.CustomerService
	log *slog.Logger
}

func NewCustomerHandler(svc *service.CustomerService, log *slog.Logger) *CustomerHandler {
	return &CustomerHandler{svc: svc, log: log}
}

func (h *CustomerHandler) List(c *gin.Context) {
	p, ok := parsePagination(c)
	if !ok {
		return
	}
	res, err := h.svc.List(c.Request.Context(), p)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *CustomerHandler) Get(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	customer, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, customer)
}

func (h *CustomerHandler) Create(c *gin.Context) {
	var in models.CustomerInput
	if !bindJSON(c, &in) {
		return
	}
	customer, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, customer)
}

func (h *CustomerHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var in models.CustomerInput
	if !bindJSON(c, &in) {
		return
	}
	customer, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, customer)
}

func (h *CustomerHandler) Delete(c *gin.Context) {
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
