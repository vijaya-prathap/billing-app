import { StatusBadge } from "../components/StatusBadge";
import { useServiceStatus } from "../hooks/useServiceStatus";
import { formatTime } from "../utils/format";

const resources = ["customers", "products", "invoices"];

export function StatusPage() {
  const { statuses, refresh } = useServiceStatus();

  return (
    <main className="page">
      <h1>Billing App</h1>
      <p className="muted">
        Frontend shell. CRUD screens are not built yet; the API is fully available under <code>/api/v1</code>.
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
          <div className="row" key={r}>
            <code>/api/v1/{r}</code>
            <span className="muted">GET · POST · PUT · DELETE</span>
          </div>
        ))}
      </section>
    </main>
  );
}
