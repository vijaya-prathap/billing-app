package models

import "time"

type InvoiceItem struct {
	ID          int64     `json:"id"`
	InvoiceID   int64     `json:"invoice_id"`
	ProductID   int64     `json:"product_id"`
	Description string    `json:"description"`
	Quantity    int       `json:"quantity"`
	UnitPrice   float64   `json:"unit_price"`
	LineTotal   float64   `json:"line_total"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateInvoiceItemRequest struct {
	ProductID   int64    `json:"product_id" binding:"required,gt=0"`
	Description string   `json:"description" binding:"max=500"`
	Quantity    int      `json:"quantity" binding:"required,gt=0"`
	UnitPrice   *float64 `json:"unit_price" binding:"omitempty,gte=0"`
}

type UpdateInvoiceItemRequest struct {
	Description string  `json:"description" binding:"max=500"`
	Quantity    int     `json:"quantity" binding:"required,gt=0"`
	UnitPrice   float64 `json:"unit_price" binding:"gte=0"`
}
