import { StatusBadge } from "../components/StatusBadge";
import { href } from "../hooks/useHashRoute";
import type { Route } from "../hooks/useHashRoute";
import { useServiceStatus } from "../hooks/useServiceStatus";
import { formatTime } from "../utils/format";

const resources: { route: Route; path: string; methods: string }[] = [
  { route: { name: "customers" }, path: "/api/v1/customers", methods: "GET · POST · PUT · DELETE" },
  { route: { name: "products" }, path: "/api/v1/products", methods: "GET · POST · PUT · DELETE" },
  { route: { name: "invoices" }, path: "/api/v1/invoices", methods: "GET · POST · PUT · DELETE" },
  { route: { name: "invoices" }, path: "/api/v1/invoices/:id/items", methods: "GET · POST · PUT · DELETE" },
];

export function StatusPage() {
  const { statuses, refresh } = useServiceStatus();

  return (
    <>
      <h1>Billing App</h1>
      <p className="muted">
        Manage customers, products and invoices. Every screen calls the REST API under <code>/api/v1</code>.
      </p>

      <section className="card">
        <div className="row" style={{ borderTop: "none", paddingTop: 0 }}>
          <h2>System status</h2>
          <button type="button" onClick={() => void refresh()}>
            Refresh
          </button>
        </div>
        {statuses.map((s) => (
          <div className="row" key={s.endpoint}>
            <div>
              <strong>{s.name}</strong>{" "}
              <code className="muted">{s.endpoint}</code>
              {s.state === "down" && <div className="muted">{s.detail}</div>}
            </div>
            <div>
              <span className="muted">{formatTime(s.checkedAt)}</span> <StatusBadge state={s.state} />
            </div>
          </div>
        ))}
      </section>

      <section className="card">
        <h2>API resources</h2>
        {resources.map((r) => (
          <div className="row" key={r.path}>
            <a href={href(r.route)}>
              <code>{r.path}</code>
            </a>
            <span className="muted">{r.methods}</span>
          </div>
        ))}
      </section>
    </>
  );
}
