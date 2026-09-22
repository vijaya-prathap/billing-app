import { useCallback, useState } from "react";
import type { FormEvent } from "react";
import { ConfirmDelete, EmptyState, ErrorBanner, Field, Modal, Pager } from "../components/ui";
import { usePaginated } from "../hooks/usePaginated";
import { api, ApiError } from "../services/api";
import type { Customer, CustomerInput } from "../types";
import { formatDate } from "../utils/format";

const emptyInput: CustomerInput = { name: "", email: "", phone: "", address: "" };

type Dialog = { kind: "create" } | { kind: "edit"; customer: Customer } | { kind: "delete"; customer: Customer } | null;

export function CustomersPage() {
  const fetcher = useCallback((page: number) => api.customers.list(page), []);
  const list = usePaginated<Customer>(fetcher);
  const [dialog, setDialog] = useState<Dialog>(null);

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Customers</h1>
          <p className="muted">
            <code>GET · POST · PUT · DELETE /api/v1/customers</code>
          </p>
        </div>
        <button type="button" className="btn-primary" onClick={() => setDialog({ kind: "create" })}>
          New customer
        </button>
      </div>

      <section className="card">
        <ErrorBanner error={list.error} />
        {list.loading && list.rows.length === 0 ? (
          <EmptyState>Loading…</EmptyState>
        ) : list.rows.length === 0 ? (
          <EmptyState>No customers yet. Create the first one.</EmptyState>
        ) : (
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Name</th>
                  <th>Email</th>
                  <th>Phone</th>
                  <th>Address</th>
                  <th>Created</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {list.rows.map((c) => (
                  <tr key={c.id}>
                    <td className="muted">{c.id}</td>
                    <td>
                      <strong>{c.name}</strong>
                    </td>
                    <td>{c.email}</td>
                    <td>{c.phone || "—"}</td>
                    <td className="truncate">{c.address || "—"}</td>
                    <td className="muted">{formatDate(c.created_at)}</td>
                    <td className="actions">
                      <button type="button" onClick={() => setDialog({ kind: "edit", customer: c })}>
                        Edit
                      </button>
                      <button type="button" className="btn-danger-ghost" onClick={() => setDialog({ kind: "delete", customer: c })}>
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
        <CustomerForm
          customer={dialog.kind === "edit" ? dialog.customer : undefined}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            void list.reload();
          }}
        />
      )}
      {dialog?.kind === "delete" && (
        <DeleteCustomer
          customer={dialog.customer}
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

function CustomerForm({ customer, onClose, onSaved }: { customer?: Customer; onClose: () => void; onSaved: () => void }) {
  const [input, setInput] = useState<CustomerInput>(
    customer ? { name: customer.name, email: customer.email, phone: customer.phone, address: customer.address } : emptyInput,
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const fieldError = (f: string) => (error instanceof ApiError ? error.fieldMessage(f) : undefined);

  const set = (k: keyof CustomerInput) => (e: { target: { value: string } }) => setInput((i) => ({ ...i, [k]: e.target.value }));

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (customer) await api.customers.update(customer.id, input);
      else await api.customers.create(input);
      onSaved();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal title={customer ? `Edit customer #${customer.id}` : "New customer"} onClose={onClose}>
      <form onSubmit={submit} className="form">
        {/* Field-level validation errors render inline; everything else goes in the banner. */}
        <ErrorBanner error={error instanceof ApiError && error.details.length > 0 ? null : error} />
        <Field label="Name" error={fieldError("name")}>
          <input value={input.name} onChange={set("name")} required minLength={2} maxLength={255} autoFocus />
        </Field>
        <Field label="Email" error={fieldError("email")}>
          <input type="email" value={input.email} onChange={set("email")} required maxLength={255} />
        </Field>
        <Field label="Phone" error={fieldError("phone")}>
          <input value={input.phone} onChange={set("phone")} maxLength={50} />
        </Field>
        <Field label="Address" error={fieldError("address")}>
          <textarea value={input.address} onChange={set("address")} maxLength={1000} rows={2} />
        </Field>
        <div className="actions end">
          <button type="button" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button type="submit" className="btn-primary" disabled={busy}>
            {busy ? "Saving…" : customer ? "Save changes" : "Create customer"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function DeleteCustomer({ customer, onClose, onDeleted }: { customer: Customer; onClose: () => void; onDeleted: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);

  async function confirm() {
    setBusy(true);
    setError(null);
    try {
      await api.customers.remove(customer.id);
      onDeleted();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  }

  return <ConfirmDelete what={customer.name} onConfirm={confirm} onClose={onClose} busy={busy} error={error} />;
}
