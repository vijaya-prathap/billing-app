import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";
import { ConfirmDelete, EmptyState, ErrorBanner, Field, InvoiceStatusBadge, Modal } from "../components/ui";
import { href, navigate } from "../hooks/useHashRoute";
import { api, ApiError, errorMessage } from "../services/api";
import type { Customer, Invoice, InvoiceItem, InvoiceStatus, Product } from "../types";
import { isTerminal, nextStatuses } from "../types";
import { formatDate, formatMoney, formatPercent, toDateInput } from "../utils/format";

type Dialog =
  | { kind: "add-item" }
  | { kind: "edit-item"; item: InvoiceItem }
  | { kind: "delete-item"; item: InvoiceItem }
  | { kind: "delete-invoice" }
  | null;

export function InvoiceDetailPage({ id }: { id: number }) {
  const [invoice, setInvoice] = useState<Invoice | null>(null);
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [products, setProducts] = useState<Product[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [dialog, setDialog] = useState<Dialog>(null);

  // Items only carry product_id; the catalogue is small enough to load once for display names.
  useEffect(() => {
    api.products
      .list(1, 100)
      .then((res) => setProducts(res.data))
      .catch(() => setProducts([]));
  }, []);
  const productLabel = (id: number) => {
    const p = products.find((p) => p.id === id);
    return p ? `${p.name} · ${p.sku}` : `#${id}`;
  };

  const load = useCallback(async () => {
    try {
      const inv = await api.invoices.get(id);
      setInvoice(inv);
      setError(null);
      api.customers
        .get(inv.customer_id)
        .then(setCustomer)
        .catch(() => setCustomer(null));
    } catch (err) {
      setError(errorMessage(err));
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  if (error) {
    return (
      <>
        <BackLink />
        <ErrorBanner error={error} />
      </>
    );
  }
  if (!invoice) {
    return (
      <>
        <BackLink />
        <EmptyState>Loading…</EmptyState>
      </>
    );
  }

  const editable = invoice.status === "draft";
  const deletable = invoice.status === "draft" || invoice.status === "cancelled";
  const items = invoice.items ?? [];

  return (
    <>
      <BackLink />
      <div className="page-head">
        <div>
          <h1>
            {invoice.invoice_number} <InvoiceStatusBadge status={invoice.status} />
          </h1>
          <p className="muted">
            {customer ? (
              <>
                {customer.name} · {customer.email}
              </>
            ) : (
              <>Customer #{invoice.customer_id}</>
            )}
          </p>
        </div>
        <button
          type="button"
          className="btn-danger-ghost"
          disabled={!deletable}
          title={deletable ? undefined : "Only draft or cancelled invoices can be deleted"}
          onClick={() => setDialog({ kind: "delete-invoice" })}
        >
          Delete invoice
        </button>
      </div>

      <section className="card">
        <h2>Details</h2>
        <InvoiceHeaderForm invoice={invoice} onSaved={setInvoice} />
      </section>

      <section className="card">
        <div className="row" style={{ borderTop: "none", paddingTop: 0 }}>
          <h2>
            Line items <code className="muted">/api/v1/invoices/{invoice.id}/items</code>
          </h2>
          <button
            type="button"
            className="btn-primary"
            disabled={!editable}
            title={editable ? undefined : "Items can only be changed while the invoice is a draft"}
            onClick={() => setDialog({ kind: "add-item" })}
          >
            Add item
          </button>
        </div>
        {!editable && <p className="muted small">Items are locked because this invoice is {invoice.status}.</p>}
        {items.length === 0 ? (
          <EmptyState>No line items.</EmptyState>
        ) : (
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>Description</th>
                  <th>Product</th>
                  <th className="num">Qty</th>
                  <th className="num">Unit price</th>
                  <th className="num">Line total</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {items.map((it) => (
                  <tr key={it.id}>
                    <td>
                      <strong>{it.description}</strong>
                    </td>
                    <td className="muted">{productLabel(it.product_id)}</td>
                    <td className="num">{it.quantity}</td>
                    <td className="num">{formatMoney(it.unit_price)}</td>
                    <td className="num">{formatMoney(it.line_total)}</td>
                    <td className="actions">
                      <button type="button" disabled={!editable} onClick={() => setDialog({ kind: "edit-item", item: it })}>
                        Edit
                      </button>
                      <button
                        type="button"
                        className="btn-danger-ghost"
                        disabled={!editable}
                        onClick={() => setDialog({ kind: "delete-item", item: it })}
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
        <div className="totals">
          <span>
            Subtotal <strong>{formatMoney(invoice.subtotal)}</strong>
          </span>
          <span>
            Tax ({formatPercent(invoice.tax_rate)}) <strong>{formatMoney(invoice.tax_amount)}</strong>
          </span>
          <span>
            Total <strong>{formatMoney(invoice.total)}</strong>
          </span>
        </div>
      </section>

      {dialog?.kind === "add-item" && (
        <AddItemForm
          invoiceId={invoice.id}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            void load();
          }}
        />
      )}
      {dialog?.kind === "edit-item" && (
        <EditItemForm
          invoiceId={invoice.id}
          item={dialog.item}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            void load();
          }}
        />
      )}
      {dialog?.kind === "delete-item" && (
        <DeleteItem
          invoiceId={invoice.id}
          item={dialog.item}
          onClose={() => setDialog(null)}
          onDeleted={() => {
            setDialog(null);
            void load();
          }}
        />
      )}
      {dialog?.kind === "delete-invoice" && (
        <DeleteInvoice invoice={invoice} onClose={() => setDialog(null)} onDeleted={() => navigate({ name: "invoices" })} />
      )}
    </>
  );
}

function BackLink() {
  return (
    <p>
      <a href={href({ name: "invoices" })}>← All invoices</a>
    </p>
  );
}

function InvoiceHeaderForm({ invoice, onSaved }: { invoice: Invoice; onSaved: (inv: Invoice) => void }) {
  const [status, setStatus] = useState<InvoiceStatus>(invoice.status);
  const [dueDate, setDueDate] = useState(toDateInput(invoice.due_date));
  const [notes, setNotes] = useState(invoice.notes);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [saved, setSaved] = useState(false);
  const fieldError = (f: string) => (error instanceof ApiError ? error.fieldMessage(f) : undefined);

  // Reset local edits when the parent reloads the invoice (e.g. after an item change).
  useEffect(() => {
    setStatus(invoice.status);
    setDueDate(toDateInput(invoice.due_date));
    setNotes(invoice.notes);
  }, [invoice]);

  const locked = isTerminal(invoice.status);
  const dirty = status !== invoice.status || dueDate !== toDateInput(invoice.due_date) || notes !== invoice.notes;

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      onSaved(await api.invoices.update(invoice.id, { status, due_date: dueDate, notes }));
      setSaved(true);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="form">
      <ErrorBanner error={error instanceof ApiError && error.details.length > 0 ? null : error} />
      <div className="grid-3">
        <Field label="Issue date">
          <input type="date" value={toDateInput(invoice.issue_date)} disabled />
        </Field>
        <Field label="Due date" error={fieldError("due_date")}>
          <input
            type="date"
            value={dueDate}
            min={toDateInput(invoice.issue_date)}
            onChange={(e) => setDueDate(e.target.value)}
            disabled={locked}
            required
          />
        </Field>
        <Field
          label="Status"
          error={fieldError("status")}
          hint={locked ? `${invoice.status} is final` : invoice.status === "draft" ? "draft → sent → paid" : "sent → paid"}
        >
          <select value={status} onChange={(e) => setStatus(e.target.value as InvoiceStatus)} disabled={locked}>
            {nextStatuses(invoice.status).map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </Field>
      </div>
      <Field label="Notes" error={fieldError("notes")}>
        <textarea value={notes} onChange={(e) => setNotes(e.target.value)} maxLength={2000} rows={2} disabled={locked} />
      </Field>
      <div className="row" style={{ borderTop: "none" }}>
        <span className="muted small">
          Created {formatDate(invoice.created_at)} · updated {formatDate(invoice.updated_at)}
        </span>
        <div className="actions">
          {saved && !dirty && <span className="muted small">Saved</span>}
          <button type="submit" className="btn-primary" disabled={locked || busy || !dirty}>
            {busy ? "Saving…" : "Save changes"}
          </button>
        </div>
      </div>
    </form>
  );
}

function AddItemForm({ invoiceId, onClose, onSaved }: { invoiceId: number; onClose: () => void; onSaved: () => void }) {
  const [products, setProducts] = useState<Product[]>([]);
  const [productId, setProductId] = useState("");
  const [description, setDescription] = useState("");
  const [quantity, setQuantity] = useState("1");
  const [unitPrice, setUnitPrice] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const fieldError = (f: string) => (error instanceof ApiError ? error.fieldMessage(f) : undefined);

  useEffect(() => {
    api.products
      .list(1, 100)
      .then((res) => setProducts(res.data))
      .catch((err) => setError(err));
  }, []);

  const product = products.find((p) => String(p.id) === productId);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.invoices.items.create(invoiceId, {
        product_id: Number(productId),
        description,
        quantity: Number(quantity),
        ...(unitPrice !== "" ? { unit_price: Number(unitPrice) } : {}),
      });
      onSaved();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  }

  return (
    <Modal title="Add line item" onClose={onClose}>
      <form onSubmit={submit} className="form">
        <ErrorBanner error={error instanceof ApiError && error.details.length > 0 ? null : error} />
        <Field label="Product" error={fieldError("product_id")}>
          <select value={productId} onChange={(e) => setProductId(e.target.value)} required autoFocus>
            <option value="" disabled>
              Select a product
            </option>
            {products.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({p.sku}) — {formatMoney(p.price)}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Description" error={fieldError("description")} hint="Defaults to the product name">
          <input value={description} placeholder={product?.name} onChange={(e) => setDescription(e.target.value)} maxLength={500} />
        </Field>
        <div className="grid-2">
          <Field label="Quantity" error={fieldError("quantity")}>
            <input type="number" value={quantity} onChange={(e) => setQuantity(e.target.value)} min="1" step="1" required />
          </Field>
          <Field label="Unit price" error={fieldError("unit_price")} hint="Leave blank to use the product price">
            <input
              type="number"
              value={unitPrice}
              placeholder={product ? product.price.toFixed(2) : ""}
              onChange={(e) => setUnitPrice(e.target.value)}
              min="0"
              step="0.01"
            />
          </Field>
        </div>
        <div className="actions end">
          <button type="button" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button type="submit" className="btn-primary" disabled={busy || !productId}>
            {busy ? "Adding…" : "Add item"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function EditItemForm({
  invoiceId,
  item,
  onClose,
  onSaved,
}: {
  invoiceId: number;
  item: InvoiceItem;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [description, setDescription] = useState(item.description);
  const [quantity, setQuantity] = useState(String(item.quantity));
  const [unitPrice, setUnitPrice] = useState(String(item.unit_price));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const fieldError = (f: string) => (error instanceof ApiError ? error.fieldMessage(f) : undefined);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.invoices.items.update(invoiceId, item.id, {
        description,
        quantity: Number(quantity),
        unit_price: Number(unitPrice),
      });
      onSaved();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  }

  return (
    <Modal title={`Edit item #${item.id}`} onClose={onClose}>
      <form onSubmit={submit} className="form">
        <ErrorBanner error={error instanceof ApiError && error.details.length > 0 ? null : error} />
        <Field label="Description" error={fieldError("description")}>
          <input value={description} onChange={(e) => setDescription(e.target.value)} maxLength={500} autoFocus />
        </Field>
        <div className="grid-2">
          <Field label="Quantity" error={fieldError("quantity")}>
            <input type="number" value={quantity} onChange={(e) => setQuantity(e.target.value)} min="1" step="1" required />
          </Field>
          <Field label="Unit price" error={fieldError("unit_price")}>
            <input type="number" value={unitPrice} onChange={(e) => setUnitPrice(e.target.value)} min="0" step="0.01" required />
          </Field>
        </div>
        <div className="actions end">
          <button type="button" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button type="submit" className="btn-primary" disabled={busy}>
            {busy ? "Saving…" : "Save changes"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function DeleteItem({
  invoiceId,
  item,
  onClose,
  onDeleted,
}: {
  invoiceId: number;
  item: InvoiceItem;
  onClose: () => void;
  onDeleted: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);

  async function confirm() {
    setBusy(true);
    setError(null);
    try {
      await api.invoices.items.remove(invoiceId, item.id);
      onDeleted();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  }

  return <ConfirmDelete what={`${item.description} × ${item.quantity}`} onConfirm={confirm} onClose={onClose} busy={busy} error={error} />;
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
