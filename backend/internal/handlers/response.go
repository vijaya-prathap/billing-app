package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"billing-app/backend/internal/middleware"
	"billing-app/backend/internal/models"
	"billing-app/backend/internal/service"
)

type errorResponse struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code      string       `json:"code"`
	Message   string       `json:"message"`
	Details   []fieldError `json:"details,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
}

type fieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func writeError(c *gin.Context, status int, code, message string, details []fieldError) {
	c.AbortWithStatusJSON(status, errorResponse{Error: errorPayload{
		Code:      code,
		Message:   message,
		Details:   details,
		RequestID: c.GetString(middleware.RequestIDKey),
	}})
}

func handleServiceError(c *gin.Context, log *slog.Logger, err error) {
	var svcErr *service.Error
	if errors.As(err, &svcErr) {
		switch svcErr.Kind {
		case service.KindValidation:
			writeError(c, http.StatusUnprocessableEntity, "validation_failed", svcErr.Message, nil)
		case service.KindNotFound:
			writeError(c, http.StatusNotFound, "not_found", svcErr.Message, nil)
		case service.KindConflict:
			writeError(c, http.StatusConflict, "conflict", svcErr.Message, nil)
		default:
			writeError(c, http.StatusInternalServerError, "internal_error", "an unexpected error occurred", nil)
		}
		return
	}

	log.ErrorContext(c.Request.Context(), "request failed",
		slog.String("request_id", c.GetString(middleware.RequestIDKey)),
		slog.String("route", c.FullPath()),
		slog.Any("error", err),
	)
	writeError(c, http.StatusInternalServerError, "internal_error", "an unexpected error occurred", nil)
}

func bindJSON(c *gin.Context, dst any) bool {
	err := c.ShouldBindJSON(dst)
	if err == nil {
		return true
	}

	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		details := make([]fieldError, 0, len(validationErrs))
		for _, fe := range validationErrs {
			details = append(details, fieldError{Field: fieldPath(fe), Message: validationMessage(fe)})
		}
		writeError(c, http.StatusUnprocessableEntity, "validation_failed", "request validation failed", details)
		return false
	}

	writeError(c, http.StatusBadRequest, "invalid_request", "request body must be valid JSON matching the expected schema", nil)
	return false
}

func parseIDParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid_request", fmt.Sprintf("%s must be a positive integer", name), nil)
		return 0, false
	}
	return id, true
}

func parsePagination(c *gin.Context) (models.Pagination, bool) {
	page, ok := parseOptionalInt(c, "page")
	if !ok {
		return models.Pagination{}, false
	}
	limit, ok := parseOptionalInt(c, "limit")
	if !ok {
		return models.Pagination{}, false
	}
	return models.NewPagination(page, limit), true
}

func parseOptionalInt(c *gin.Context, key string) (int, bool) {
	raw := c.Query(key)
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		writeError(c, http.StatusBadRequest, "invalid_request", fmt.Sprintf("%s must be a non-negative integer", key), nil)
		return 0, false
	}
	return n, true
}

// fieldPath drops the root struct name so clients see "items[0].quantity", not "CreateInvoiceRequest.items[0].quantity".
func fieldPath(fe validator.FieldError) string {
	ns := fe.Namespace()
	if i := strings.IndexByte(ns, '.'); i >= 0 {
		return ns[i+1:]
	}
	return ns
}

func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		if fe.Kind().String() == "slice" {
			return fmt.Sprintf("must contain at least %s entries", fe.Param())
		}
		return fmt.Sprintf("must be at least %s characters", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters", fe.Param())
	case "gt":
		return fmt.Sprintf("must be greater than %s", fe.Param())
	case "gte":
		return fmt.Sprintf("must be greater than or equal to %s", fe.Param())
	case "lte":
		return fmt.Sprintf("must be less than or equal to %s", fe.Param())
	case "oneof":
		return "must be one of: " + strings.ReplaceAll(fe.Param(), " ", ", ")
	case "datetime":
		return "must be a date in YYYY-MM-DD format"
	}
	return "is invalid"
}
