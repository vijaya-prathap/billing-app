import { href, useHashRoute } from "./hooks/useHashRoute";
import type { Route } from "./hooks/useHashRoute";
import { CustomersPage } from "./pages/CustomersPage";
import { InvoiceDetailPage } from "./pages/InvoiceDetailPage";
import { InvoicesPage } from "./pages/InvoicesPage";
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
  const route = useHashRoute();

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
