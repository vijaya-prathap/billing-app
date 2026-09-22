export interface HealthResponse {
  status: string;
}

export interface FieldError {
  field: string;
  message: string;
}

export interface ApiErrorBody {
  error: {
    code: string;
    message: string;
    request_id?: string;
    details?: FieldError[];
  };
}

export type ServiceState = "checking" | "up" | "down";

export interface ServiceStatus {
  name: string;
  endpoint: string;
  state: ServiceState;
  detail: string;
  checkedAt: Date | null;
}

// Mirrors backend/internal/models/pagination.go.
export interface Paginated<T> {
  data: T[];
  page: number;
  limit: number;
  total: number;
  total_pages: number;
}

export interface Customer {
  id: number;
  name: string;
  email: string;
  phone: string;
  address: string;
  created_at: string;
  updated_at: string;
}

export interface CustomerInput {
  name: string;
  email: string;
  phone: string;
  address: string;
}

export interface Product {
  id: number;
  name: string;
  description: string;
  sku: string;
  price: number;
  stock: number;
  created_at: string;
  updated_at: string;
}

export interface ProductInput {
  name: string;
  description: string;
  sku: string;
  price: number;
  stock: number;
}

export type InvoiceStatus = "draft" | "sent" | "paid" | "cancelled";

export const INVOICE_STATUSES: InvoiceStatus[] = ["draft", "sent", "paid", "cancelled"];

// Mirrors InvoiceStatus.CanTransitionTo in backend/internal/models/invoice.go.
export function nextStatuses(current: InvoiceStatus): InvoiceStatus[] {
  switch (current) {
    case "draft":
      return ["draft", "sent", "cancelled"];
    case "sent":
      return ["sent", "paid", "cancelled"];
    default:
      return [current];
  }
}

export function isTerminal(status: InvoiceStatus): boolean {
  return status === "paid" || status === "cancelled";
}

export interface InvoiceItem {
  id: number;
  invoice_id: number;
  product_id: number;
  description: string;
  quantity: number;
  unit_price: number;
  line_total: number;
  created_at: string;
}

export interface Invoice {
  id: number;
  customer_id: number;
  invoice_number: string;
  status: InvoiceStatus;
  issue_date: string;
  due_date: string;
  subtotal: number;
  tax_rate: number;
  tax_amount: number;
  total: number;
  notes: string;
  items?: InvoiceItem[];
  created_at: string;
  updated_at: string;
}

export interface CreateInvoiceItemInput {
  product_id: number;
  description: string;
  quantity: number;
  unit_price?: number;
}

export interface UpdateInvoiceItemInput {
  description: string;
  quantity: number;
  unit_price: number;
}

export interface CreateInvoiceInput {
  customer_id: number;
  issue_date: string;
  due_date: string;
  tax_rate: number;
  notes: string;
  items: CreateInvoiceItemInput[];
}

export interface UpdateInvoiceInput {
  status: InvoiceStatus;
  due_date: string;
  notes: string;
}

export interface InvoiceListFilter {
  page?: number;
  limit?: number;
  status?: InvoiceStatus | "";
  customer_id?: number | "";
}
