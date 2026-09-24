import type {
  ApiErrorBody,
  AuthConfig,
  AuthUser,
  CreateInvoiceInput,
  CreateInvoiceItemInput,
  Customer,
  CustomerInput,
  FieldError,
  HealthResponse,
  Invoice,
  InvoiceItem,
  InvoiceListFilter,
  Paginated,
  Product,
  ProductInput,
  UpdateInvoiceInput,
  UpdateInvoiceItemInput,
} from "../types";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
    readonly requestId?: string,
    readonly details: FieldError[] = [],
  ) {
    super(message);
    this.name = "ApiError";
  }

  // Validation errors from the backend are keyed by JSON field name ("email", "items[0].quantity").
  fieldMessage(field: string): string | undefined {
    return this.details.find((d) => d.field === field)?.message;
  }
}

// Fired when a protected call is answered with 401, so the app can drop back to the login screen.
export const AUTH_EXPIRED_EVENT = "billing:auth-expired";
// Fired on 403: the account was removed from the allowlist while signed in.
export const AUTH_REVOKED_EVENT = "billing:auth-revoked";

export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  if (err instanceof Error) return err.message;
  return "Something went wrong";
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { Accept: "application/json", ...init?.headers },
  });

  const body: unknown = res.status === 204 ? null : await res.json().catch(() => null);

  if (!res.ok) {
    if (!path.startsWith(`${base}/auth/`)) {
      if (res.status === 401) window.dispatchEvent(new Event(AUTH_EXPIRED_EVENT));
      if (res.status === 403) window.dispatchEvent(new Event(AUTH_REVOKED_EVENT));
    }
    const err = (body as ApiErrorBody | null)?.error;
    throw new ApiError(res.status, err?.message ?? `HTTP ${res.status}`, err?.request_id, err?.details ?? []);
  }
  return body as T;
}

function json(method: "POST" | "PUT", payload: unknown): RequestInit {
  return { method, headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) };
}

function query(params: Record<string, string | number | undefined | "">): string {
  const search = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== "") search.set(k, String(v));
  }
  const s = search.toString();
  return s ? `?${s}` : "";
}

const base = "/api/v1";

export const api = {
  health: () => request<HealthResponse>("/health"),
  ready: () => request<HealthResponse>("/ready"),

  auth: {
    // Full-page navigation target: the API redirects the browser to Google and back.
    googleLoginUrl: `${base}/auth/google/login`,
    config: () => request<AuthConfig>(`${base}/auth/config`),
    me: () => request<{ user: AuthUser }>(`${base}/auth/me`),
    logout: () => request<void>(`${base}/auth/logout`, { method: "POST" }),
  },

  customers: {
    list: (page = 1, limit = 20) => request<Paginated<Customer>>(`${base}/customers${query({ page, limit })}`),
    get: (id: number) => request<Customer>(`${base}/customers/${id}`),
    create: (input: CustomerInput) => request<Customer>(`${base}/customers`, json("POST", input)),
    update: (id: number, input: CustomerInput) => request<Customer>(`${base}/customers/${id}`, json("PUT", input)),
    remove: (id: number) => request<void>(`${base}/customers/${id}`, { method: "DELETE" }),
  },

  products: {
    list: (page = 1, limit = 20) => request<Paginated<Product>>(`${base}/products${query({ page, limit })}`),
    get: (id: number) => request<Product>(`${base}/products/${id}`),
    create: (input: ProductInput) => request<Product>(`${base}/products`, json("POST", input)),
    update: (id: number, input: ProductInput) => request<Product>(`${base}/products/${id}`, json("PUT", input)),
    remove: (id: number) => request<void>(`${base}/products/${id}`, { method: "DELETE" }),
  },

  invoices: {
    list: (f: InvoiceListFilter = {}) =>
      request<Paginated<Invoice>>(
        `${base}/invoices${query({ page: f.page ?? 1, limit: f.limit ?? 20, status: f.status, customer_id: f.customer_id })}`,
      ),
    get: (id: number) => request<Invoice>(`${base}/invoices/${id}`),
    create: (input: CreateInvoiceInput) => request<Invoice>(`${base}/invoices`, json("POST", input)),
    update: (id: number, input: UpdateInvoiceInput) => request<Invoice>(`${base}/invoices/${id}`, json("PUT", input)),
    remove: (id: number) => request<void>(`${base}/invoices/${id}`, { method: "DELETE" }),

    items: {
      list: (invoiceId: number) => request<{ data: InvoiceItem[] }>(`${base}/invoices/${invoiceId}/items`),
      create: (invoiceId: number, input: CreateInvoiceItemInput) =>
        request<InvoiceItem>(`${base}/invoices/${invoiceId}/items`, json("POST", input)),
      update: (invoiceId: number, itemId: number, input: UpdateInvoiceItemInput) =>
        request<InvoiceItem>(`${base}/invoices/${invoiceId}/items/${itemId}`, json("PUT", input)),
      remove: (invoiceId: number, itemId: number) =>
        request<void>(`${base}/invoices/${invoiceId}/items/${itemId}`, { method: "DELETE" }),
    },
  },
};
