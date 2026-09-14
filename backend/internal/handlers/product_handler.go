package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"billing-app/backend/internal/models"
	"billing-app/backend/internal/service"
)

type ProductHandler struct {
	svc *service.ProductService
	log *slog.Logger
}

func NewProductHandler(svc *service.ProductService, log *slog.Logger) *ProductHandler {
	return &ProductHandler{svc: svc, log: log}
}

func (h *ProductHandler) List(c *gin.Context) {
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

func (h *ProductHandler) Get(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	product, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) Create(c *gin.Context) {
	var in models.ProductInput
	if !bindJSON(c, &in) {
		return
	}
	product, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, product)
}

func (h *ProductHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var in models.ProductInput
	if !bindJSON(c, &in) {
		return
	}
	product, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) Delete(c *gin.Context) {
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
