import { ErrorBanner } from "./components/ui";
import { UserMenu } from "./components/UserMenu";
import { useAuth } from "./hooks/useAuth";
import { href, useHashRoute } from "./hooks/useHashRoute";
import type { Route } from "./hooks/useHashRoute";
import { CustomersPage } from "./pages/CustomersPage";
import { InvoiceDetailPage } from "./pages/InvoiceDetailPage";
import { InvoicesPage } from "./pages/InvoicesPage";
import { LoginPage } from "./pages/LoginPage";
import { ProductsPage } from "./pages/ProductsPage";
import { StatusPage } from "./pages/StatusPage";

const nav: { route: Route; label: string }[] = [
  { route: { name: "status" }, label: "Status" },
  { route: { name: "customers" }, label: "Customers" },
  { route: { name: "products" }, label: "Products" },
  { route: { name: "invoices" }, label: "Invoices" },
];

function isActive(current: Route, target: Route): boolean {
  if (current.name === target.name) return true;
  return current.name === "invoice" && target.name === "invoices";
}

export default function App() {
  const auth = useAuth();
  const route = useHashRoute();

  if (auth.status === "loading") {
    return (
      <div className="splash" aria-busy="true">
        <span className="muted">Loading…</span>
      </div>
    );
  }

  if (auth.status === "error") {
    return (
      <div className="splash">
        <div className="splash-card">
          <h2>Can't reach the billing API</h2>
          <ErrorBanner error={auth.error} />
          <button type="button" className="btn-primary" onClick={auth.retry}>
            Try again
          </button>
        </div>
      </div>
    );
  }

  if (auth.status === "anonymous") {
    return <LoginPage />;
  }

  return (
    <>
      <header className="topbar">
        <div className="topbar-inner">
          <a className="brand" href={href({ name: "status" })}>
            Billing App
          </a>
          <nav>
            {nav.map(({ route: r, label }) => (
              <a key={r.name} href={href(r)} className={isActive(route, r) ? "active" : undefined}>
                {label}
              </a>
            ))}
          </nav>
          {auth.status === "authenticated" && <UserMenu />}
        </div>
      </header>
      <main className="page">
        {route.name === "status" && <StatusPage />}
        {route.name === "customers" && <CustomersPage />}
        {route.name === "products" && <ProductsPage />}
        {route.name === "invoices" && <InvoicesPage />}
        {route.name === "invoice" && <InvoiceDetailPage key={route.id} id={route.id} />}
      </main>
    </>
  );
}
