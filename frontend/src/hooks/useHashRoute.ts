import { useEffect, useState } from "react";

export type Route =
  | { name: "status" }
  | { name: "customers" }
  | { name: "products" }
  | { name: "invoices" }
  | { name: "invoice"; id: number };

// Hash-based routing keeps the app a single static file behind nginx's try_files
// and avoids pulling in a router dependency for four screens.
export function parseHash(hash: string): Route {
  const path = hash.replace(/^#\/?/, "");
  const invoice = /^invoices\/(\d+)$/.exec(path);
  if (invoice) return { name: "invoice", id: Number(invoice[1]) };
  switch (path) {
    case "customers":
    case "products":
    case "invoices":
      return { name: path };
    default:
      return { name: "status" };
  }
}

export function href(route: Route): string {
  return route.name === "invoice" ? `#/invoices/${route.id}` : `#/${route.name}`;
}

export function navigate(route: Route): void {
  window.location.hash = href(route);
}

export function useHashRoute(): Route {
  const [route, setRoute] = useState<Route>(() => parseHash(window.location.hash));

  useEffect(() => {
    const onChange = () => setRoute(parseHash(window.location.hash));
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);

  return route;
}
