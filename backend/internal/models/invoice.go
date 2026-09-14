package models

import "time"

type InvoiceStatus string

const (
	InvoiceStatusDraft     InvoiceStatus = "draft"
	InvoiceStatusSent      InvoiceStatus = "sent"
	InvoiceStatusPaid      InvoiceStatus = "paid"
	InvoiceStatusCancelled InvoiceStatus = "cancelled"
)

func (s InvoiceStatus) Valid() bool {
	switch s {
	case InvoiceStatusDraft, InvoiceStatusSent, InvoiceStatusPaid, InvoiceStatusCancelled:
		return true
	}
	return false
}

// CanTransitionTo encodes the invoice lifecycle: draft -> sent -> paid, with
// cancellation allowed until payment. Paid and cancelled are terminal.
func (s InvoiceStatus) CanTransitionTo(next InvoiceStatus) bool {
	if s == next {
		return true
	}
	switch s {
	case InvoiceStatusDraft:
		return next == InvoiceStatusSent || next == InvoiceStatusCancelled
	case InvoiceStatusSent:
		return next == InvoiceStatusPaid || next == InvoiceStatusCancelled
	}
	return false
}

func (s InvoiceStatus) IsTerminal() bool {
	return s == InvoiceStatusPaid || s == InvoiceStatusCancelled
}

type Invoice struct {
	ID            int64         `json:"id"`
	CustomerID    int64         `json:"customer_id"`
	InvoiceNumber string        `json:"invoice_number"`
	Status        InvoiceStatus `json:"status"`
	IssueDate     time.Time     `json:"issue_date"`
	DueDate       time.Time     `json:"due_date"`
	Subtotal      float64       `json:"subtotal"`
	TaxRate       float64       `json:"tax_rate"`
	TaxAmount     float64       `json:"tax_amount"`
	Total         float64       `json:"total"`
	Notes         string        `json:"notes"`
	Items         []InvoiceItem `json:"items,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

type InvoiceFilter struct {
	CustomerID int64
	Status     InvoiceStatus
	Pagination
}

type CreateInvoiceRequest struct {
	CustomerID int64                      `json:"customer_id" binding:"required,gt=0"`
	IssueDate  string                     `json:"issue_date" binding:"required,datetime=2006-01-02"`
	DueDate    string                     `json:"due_date" binding:"required,datetime=2006-01-02"`
	TaxRate    float64                    `json:"tax_rate" binding:"gte=0,lte=100"`
	Notes      string                     `json:"notes" binding:"max=2000"`
	Items      []CreateInvoiceItemRequest `json:"items" binding:"required,min=1,dive"`
}

type UpdateInvoiceRequest struct {
	Status  InvoiceStatus `json:"status" binding:"required,oneof=draft sent paid cancelled"`
	DueDate string        `json:"due_date" binding:"required,datetime=2006-01-02"`
	Notes   string        `json:"notes" binding:"max=2000"`
}
