import type { ServiceState } from "../types";

const variants: Record<ServiceState, { label: string; className: string }> = {
  checking: { label: "Checking", className: "badge badge-warn" },
  up: { label: "Operational", className: "badge badge-ok" },
  down: { label: "Unavailable", className: "badge badge-bad" },
};

export function StatusBadge({ state }: { state: ServiceState }) {
  const { label, className } = variants[state];
  return <span className={className}>{label}</span>;
}
