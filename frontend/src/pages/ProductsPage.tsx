import { useCallback, useState } from "react";
import type { FormEvent } from "react";
import { ConfirmDelete, EmptyState, ErrorBanner, Field, Modal, Pager } from "../components/ui";
import { usePaginated } from "../hooks/usePaginated";
import { api, ApiError } from "../services/api";
import type { Product, ProductInput } from "../types";
import { formatMoney } from "../utils/format";

// Form state keeps numbers as strings so partially typed values ("12.") don't get clobbered.
interface Draft {
  name: string;
  description: string;
  sku: string;
  price: string;
  stock: string;
}

const emptyDraft: Draft = { name: "", description: "", sku: "", price: "", stock: "0" };

function toInput(d: Draft): ProductInput {
  return {
    name: d.name,
    description: d.description,
    sku: d.sku,
    price: Number(d.price),
    stock: Number(d.stock || 0),
  };
}

type Dialog = { kind: "create" } | { kind: "edit"; product: Product } | { kind: "delete"; product: Product } | null;

export function ProductsPage() {
  const fetcher = useCallback((page: number) => api.products.list(page), []);
  const list = usePaginated<Product>(fetcher);
  const [dialog, setDialog] = useState<Dialog>(null);

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Products</h1>
          <p className="muted">
            <code>GET · POST · PUT · DELETE /api/v1/products</code>
          </p>
        </div>
        <button type="button" className="btn-primary" onClick={() => setDialog({ kind: "create" })}>
          New product
        </button>
      </div>

      <section className="card">
        <ErrorBanner error={list.error} />
        {list.loading && list.rows.length === 0 ? (
          <EmptyState>Loading…</EmptyState>
        ) : list.rows.length === 0 ? (
          <EmptyState>No products yet. Create the first one.</EmptyState>
        ) : (
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Name</th>
                  <th>SKU</th>
                  <th>Description</th>
                  <th className="num">Price</th>
                  <th className="num">Stock</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {list.rows.map((p) => (
                  <tr key={p.id}>
                    <td className="muted">{p.id}</td>
                    <td>
                      <strong>{p.name}</strong>
                    </td>
                    <td>
                      <code>{p.sku}</code>
                    </td>
                    <td className="truncate muted">{p.description || "—"}</td>
                    <td className="num">{formatMoney(p.price)}</td>
                    <td className="num">{p.stock}</td>
                    <td className="actions">
                      <button type="button" onClick={() => setDialog({ kind: "edit", product: p })}>
                        Edit
                      </button>
                      <button type="button" className="btn-danger-ghost" onClick={() => setDialog({ kind: "delete", product: p })}>
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

      {(dialog?.kind === "create" || dialog?.kind === "edit") && (
        <ProductForm
          product={dialog.kind === "edit" ? dialog.product : undefined}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            void list.reload();
          }}
        />
      )}
      {dialog?.kind === "delete" && (
        <DeleteProduct
          product={dialog.product}
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

function ProductForm({ product, onClose, onSaved }: { product?: Product; onClose: () => void; onSaved: () => void }) {
  const [draft, setDraft] = useState<Draft>(
    product
      ? {
          name: product.name,
          description: product.description,
          sku: product.sku,
          price: String(product.price),
          stock: String(product.stock),
        }
      : emptyDraft,
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const fieldError = (f: string) => (error instanceof ApiError ? error.fieldMessage(f) : undefined);

  const set = (k: keyof Draft) => (e: { target: { value: string } }) => setDraft((d) => ({ ...d, [k]: e.target.value }));

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const input = toInput(draft);
      if (product) await api.products.update(product.id, input);
      else await api.products.create(input);
      onSaved();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal title={product ? `Edit product #${product.id}` : "New product"} onClose={onClose}>
      <form onSubmit={submit} className="form">
        <ErrorBanner error={error instanceof ApiError && error.details.length > 0 ? null : error} />
        <Field label="Name" error={fieldError("name")}>
          <input value={draft.name} onChange={set("name")} required minLength={2} maxLength={255} autoFocus />
        </Field>
        <Field label="SKU" error={fieldError("sku")}>
          <input value={draft.sku} onChange={set("sku")} required maxLength={100} placeholder="HW-KBD-001" />
        </Field>
        <div className="grid-2">
          <Field label="Price" error={fieldError("price")}>
            <input type="number" value={draft.price} onChange={set("price")} required min="0.01" step="0.01" />
          </Field>
          <Field label="Stock" error={fieldError("stock")} hint="Use 0 for services">
            <input type="number" value={draft.stock} onChange={set("stock")} min="0" step="1" />
          </Field>
        </div>
        <Field label="Description" error={fieldError("description")}>
          <textarea value={draft.description} onChange={set("description")} maxLength={2000} rows={2} />
        </Field>
        <div className="actions end">
          <button type="button" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button type="submit" className="btn-primary" disabled={busy}>
            {busy ? "Saving…" : product ? "Save changes" : "Create product"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function DeleteProduct({ product, onClose, onDeleted }: { product: Product; onClose: () => void; onDeleted: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);

  async function confirm() {
    setBusy(true);
    setError(null);
    try {
      await api.products.remove(product.id);
      onDeleted();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  }

  return <ConfirmDelete what={`${product.name} (${product.sku})`} onConfirm={confirm} onClose={onClose} busy={busy} error={error} />;
}
