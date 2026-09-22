import type { ReactNode } from "react";
import { useEffect } from "react";
import { ApiError } from "../services/api";
import type { InvoiceStatus } from "../types";

export function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="modal-backdrop" onMouseDown={onClose}>
      <div className="modal" role="dialog" aria-modal="true" aria-label={title} onMouseDown={(e) => e.stopPropagation()}>
        <div className="modal-head">
          <h2>{title}</h2>
          <button type="button" className="btn-ghost" onClick={onClose} aria-label="Close">
            ×
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

export function Field({
  label,
  error,
  children,
  hint,
}: {
  label: string;
  error?: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="field">
      <span className="field-label">{label}</span>
      {children}
      {error ? <span className="field-error">{error}</span> : hint ? <span className="muted small">{hint}</span> : null}
    </label>
  );
}

export function ErrorBanner({ error }: { error: unknown }) {
  if (!error) return null;
  const message = error instanceof Error ? error.message : String(error);
  const requestId = error instanceof ApiError ? error.requestId : undefined;
  return (
    <div className="banner banner-bad" role="alert">
      {message}
      {requestId && <span className="muted small"> · request {requestId}</span>}
    </div>
  );
}

export function EmptyState({ children }: { children: ReactNode }) {
  return <div className="empty muted">{children}</div>;
}

export function Pager({
  page,
  totalPages,
  total,
  onChange,
}: {
  page: number;
  totalPages: number;
  total: number;
  onChange: (page: number) => void;
}) {
  return (
    <div className="pager">
      <span className="muted small">
        {total} total · page {page} of {totalPages}
      </span>
      <div className="actions">
        <button type="button" disabled={page <= 1} onClick={() => onChange(page - 1)}>
          Previous
        </button>
        <button type="button" disabled={page >= totalPages} onClick={() => onChange(page + 1)}>
          Next
        </button>
      </div>
    </div>
  );
}

const statusClass: Record<InvoiceStatus, string> = {
  draft: "badge badge-muted",
  sent: "badge badge-warn",
  paid: "badge badge-ok",
  cancelled: "badge badge-bad",
};

export function InvoiceStatusBadge({ status }: { status: InvoiceStatus }) {
  return <span className={statusClass[status]}>{status}</span>;
}

export function ConfirmDelete({
  what,
  onConfirm,
  onClose,
  busy,
  error,
}: {
  what: string;
  onConfirm: () => void;
  onClose: () => void;
  busy: boolean;
  error: unknown;
}) {
  return (
    <Modal title="Confirm delete" onClose={onClose}>
      <p>
        Delete <strong>{what}</strong>? This cannot be undone.
      </p>
      <ErrorBanner error={error} />
      <div className="actions end">
        <button type="button" onClick={onClose} disabled={busy}>
          Cancel
        </button>
        <button type="button" className="btn-danger" onClick={onConfirm} disabled={busy}>
          {busy ? "Deleting…" : "Delete"}
        </button>
      </div>
    </Modal>
  );
}
