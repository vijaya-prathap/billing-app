import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";
import { ConfirmDelete, EmptyState, ErrorBanner, Field, InvoiceStatusBadge, Modal, Pager } from "../components/ui";
import { href } from "../hooks/useHashRoute";
import { usePaginated } from "../hooks/usePaginated";
import { api, ApiError } from "../services/api";
import type { CreateInvoiceInput, Customer, Invoice, InvoiceStatus, Product } from "../types";
import { INVOICE_STATUSES } from "../types";
import { formatDate, formatMoney, todayInput } from "../utils/format";

type Dialog = { kind: "create" } | { kind: "delete"; invoice: Invoice } | null;

export function InvoicesPage() {
  const [status, setStatus] = useState<InvoiceStatus | "">("");
  const [customerId, setCustomerId] = useState<number | "">("");
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [dialog, setDialog] = useState<Dialog>(null);

  useEffect(() => {
    api.customers
      .list(1, 100)
      .then((res) => setCustomers(res.data))
      .catch(() => setCustomers([]));
  }, []);

  const fetcher = useCallback(
    (page: number) => api.invoices.list({ page, status, customer_id: customerId }),
    [status, customerId],
  );
  const list = usePaginated<Invoice>(fetcher);
  const customerName = (id: number) => customers.find((c) => c.id === id)?.name ?? `#${id}`;

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Invoices</h1>
          <p className="muted">
            <code>GET · POST · PUT · DELETE /api/v1/invoices</code>
          </p>
        </div>
        <button type="button" className="btn-primary" onClick={() => setDialog({ kind: "create" })}>
          New invoice
        </button>
      </div>

      <section className="card">
        <div className="filters">
          <label className="field inline">
            <span className="field-label">Status</span>
            <select
              value={status}
              onChange={(e) => {
                setStatus(e.target.value as InvoiceStatus | "");
                list.setPage(1);
              }}
            >
              <option value="">All</option>
              {INVOICE_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </label>
          <label className="field inline">
            <span className="field-label">Customer</span>
            <select
              value={customerId}
              onChange={(e) => {
                setCustomerId(e.target.value ? Number(e.target.value) : "");
                list.setPage(1);
              }}
            >
              <option value="">All</option>
              {customers.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          </label>
        </div>

        <ErrorBanner error={list.error} />
        {list.loading && list.rows.length === 0 ? (
          <EmptyState>Loading…</EmptyState>
        ) : list.rows.length === 0 ? (
          <EmptyState>No invoices match. Create one or clear the filters.</EmptyState>
        ) : (
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>Number</th>
                  <th>Customer</th>
                  <th>Status</th>
                  <th>Issued</th>
                  <th>Due</th>
                  <th className="num">Total</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {list.rows.map((inv) => (
                  <tr key={inv.id}>
                    <td>
                      <a href={href({ name: "invoice", id: inv.id })}>
                        <strong>{inv.invoice_number}</strong>
                      </a>
                    </td>
                    <td>{customerName(inv.customer_id)}</td>
                    <td>
                      <InvoiceStatusBadge status={inv.status} />
                    </td>
                    <td className="muted">{formatDate(inv.issue_date)}</td>
                    <td className="muted">{formatDate(inv.due_date)}</td>
                    <td className="num">{formatMoney(inv.total)}</td>
                    <td className="actions">
                      <a className="button" href={href({ name: "invoice", id: inv.id })}>
                        Open
                      </a>
                      <button
                        type="button"
                        className="btn-danger-ghost"
                        disabled={inv.status === "sent" || inv.status === "paid"}
                        title={inv.status === "sent" || inv.status === "paid" ? "Only draft or cancelled invoices can be deleted" : undefined}
                        onClick={() => setDialog({ kind: "delete", invoice: inv })}
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <Pager page={list.page} totalPages={list.totalPages} total={list.total} onChange={list.setPage} />
      </section>

      {dialog?.kind === "create" && (
        <InvoiceCreateForm
          customers={customers}
          onClose={() => setDialog(null)}
          onSaved={(inv) => {
            setDialog(null);
            window.location.hash = href({ name: "invoice", id: inv.id });
          }}
        />
      )}
      {dialog?.kind === "delete" && (
        <DeleteInvoice
          invoice={dialog.invoice}
          onClose={() => setDialog(null)}
          onDeleted={() => {
            setDialog(null);
            list.reloadAfterDelete();
          }}
        />
      )}
    </>
  );
}

interface ItemDraft {
  product_id: string;
  description: string;
  quantity: string;
  unit_price: string; // blank means "use the product's price"
}

const emptyItem: ItemDraft = { product_id: "", description: "", quantity: "1", unit_price: "" };

function InvoiceCreateForm({
  customers,
  onClose,
  onSaved,
}: {
  customers: Customer[];
  onClose: () => void;
  onSaved: (inv: Invoice) => void;
}) {
  const [products, setProducts] = useState<Product[]>([]);
  const [customerId, setCustomerId] = useState(customers[0] ? String(customers[0].id) : "");
  const [issueDate, setIssueDate] = useState(todayInput());
  const [dueDate, setDueDate] = useState(todayInput());
  const [taxRate, setTaxRate] = useState("0");
  const [notes, setNotes] = useState("");
  const [items, setItems] = useState<ItemDraft[]>([emptyItem]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const fieldError = (f: string) => (error instanceof ApiError ? error.fieldMessage(f) : undefined);

  useEffect(() => {
    api.products
      .list(1, 100)
      .then((res) => setProducts(res.data))
      .catch((err) => setError(err));
  }, []);

  const productById = (id: string) => products.find((p) => String(p.id) === id);
  const updateItem = (i: number, patch: Partial<ItemDraft>) =>
    setItems((list) => list.map((it, idx) => (idx === i ? { ...it, ...patch } : it)));

  const subtotal = items.reduce((sum, it) => {
    const price = it.unit_price !== "" ? Number(it.unit_price) : (productById(it.product_id)?.price ?? 0);
    return sum + price * Number(it.quantity || 0);
  }, 0);
  const tax = subtotal * (Number(taxRate || 0) / 100);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const input: CreateInvoiceInput = {
      customer_id: Number(customerId),
      issue_date: issueDate,
      due_date: dueDate,
      tax_rate: Number(taxRate || 0),
      notes,
      items: items.map((it) => ({
        product_id: Number(it.product_id),
        description: it.description,
        quantity: Number(it.quantity),
        ...(it.unit_price !== "" ? { unit_price: Number(it.unit_price) } : {}),
      })),
    };
    try {
      onSaved(await api.invoices.create(input));
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  }

  return (
    <Modal title="New invoice" onClose={onClose}>
      <form onSubmit={submit} className="form">
        <ErrorBanner error={error instanceof ApiError && error.details.length > 0 ? null : error} />
        <Field label="Customer" error={fieldError("customer_id")}>
          <select value={customerId} onChange={(e) => setCustomerId(e.target.value)} required autoFocus>
            <option value="" disabled>
              Select a customer
            </option>
            {customers.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </Field>
        <div className="grid-3">
          <Field label="Issue date" error={fieldError("issue_date")}>
            <input type="date" value={issueDate} onChange={(e) => setIssueDate(e.target.value)} required />
          </Field>
          <Field label="Due date" error={fieldError("due_date")}>
            <input type="date" value={dueDate} min={issueDate} onChange={(e) => setDueDate(e.target.value)} required />
          </Field>
          <Field label="Tax rate %" error={fieldError("tax_rate")}>
            <input type="number" value={taxRate} onChange={(e) => setTaxRate(e.target.value)} min="0" max="100" step="0.01" />
          </Field>
        </div>

        <div className="field-label">Line items</div>
        {fieldError("items") && <span className="field-error">{fieldError("items")}</span>}
        <div className="table-wrap">
          <table className="table compact">
            <thead>
              <tr>
                <th>Product</th>
                <th>Description</th>
                <th className="num">Qty</th>
                <th className="num">Unit price</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {items.map((it, i) => {
                const product = productById(it.product_id);
                return (
                  <tr key={i}>
                    <td>
                      <select value={it.product_id} onChange={(e) => updateItem(i, { product_id: e.target.value })} required>
                        <option value="" disabled>
                          Select
                        </option>
                        {products.map((p) => (
                          <option key={p.id} value={p.id}>
                            {p.name} ({p.sku})
                          </option>
                        ))}
                      </select>
                      {fieldError(`items[${i}].product_id`) && (
                        <span className="field-error">{fieldError(`items[${i}].product_id`)}</span>
                      )}
                    </td>
                    <td>
                      <input
                        value={it.description}
                        placeholder={product?.name ?? "Defaults to product name"}
                        onChange={(e) => updateItem(i, { description: e.target.value })}
                        maxLength={500}
                      />
                    </td>
                    <td className="num">
                      <input
                        type="number"
                        className="narrow"
                        value={it.quantity}
                        onChange={(e) => updateItem(i, { quantity: e.target.value })}
                        min="1"
                        step="1"
                        required
                      />
                      {fieldError(`items[${i}].quantity`) && (
                        <span className="field-error">{fieldError(`items[${i}].quantity`)}</span>
                      )}
                    </td>
                    <td className="num">
                      <input
                        type="number"
                        className="narrow"
                        value={it.unit_price}
                        placeholder={product ? product.price.toFixed(2) : ""}
                        onChange={(e) => updateItem(i, { unit_price: e.target.value })}
                        min="0"
                        step="0.01"
                      />
                    </td>
                    <td className="actions">
                      <button
                        type="button"
                        className="btn-ghost"
                        aria-label="Remove item"
                        disabled={items.length === 1}
                        onClick={() => setItems((list) => list.filter((_, idx) => idx !== i))}
                      >
                        ×
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <div className="actions">
          <button type="button" onClick={() => setItems((list) => [...list, emptyItem])}>
            Add item
          </button>
        </div>

        <Field label="Notes" error={fieldError("notes")}>
          <textarea value={notes} onChange={(e) => setNotes(e.target.value)} maxLength={2000} rows={2} />
        </Field>

        <div className="totals">
          <span>
            Subtotal <strong>{formatMoney(subtotal)}</strong>
          </span>
          <span>
            Tax <strong>{formatMoney(tax)}</strong>
          </span>
          <span>
            Total <strong>{formatMoney(subtotal + tax)}</strong>
          </span>
        </div>

        <div className="actions end">
          <button type="button" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button type="submit" className="btn-primary" disabled={busy || products.length === 0}>
            {busy ? "Creating…" : "Create invoice"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function DeleteInvoice({ invoice, onClose, onDeleted }: { invoice: Invoice; onClose: () => void; onDeleted: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);

  async function confirm() {
    setBusy(true);
    setError(null);
    try {
      await api.invoices.remove(invoice.id);
      onDeleted();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  }

  return <ConfirmDelete what={invoice.invoice_number} onConfirm={confirm} onClose={onClose} busy={busy} error={error} />;
}
