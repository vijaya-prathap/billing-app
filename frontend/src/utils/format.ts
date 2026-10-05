const timeFormatter = new Intl.DateTimeFormat(undefined, {
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
});

const dateFormatter = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });

const moneyFormatter = new Intl.NumberFormat(undefined, {
  style: "currency",
  currency: "USD",
  minimumFractionDigits: 2,
});

export function formatTime(date: Date | null): string {
  return date ? timeFormatter.format(date) : "—";
}

// API dates are RFC 3339 timestamps; only the calendar day is meaningful for invoices.
export function formatDate(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : dateFormatter.format(d);
}

export function toDateInput(iso: string): string {
  return iso.slice(0, 10);
}

export function todayInput(): string {
  return new Date().toISOString().slice(0, 10);
}

export function formatMoney(n: number): string {
  return moneyFormatter.format(n);
}

export function formatPercent(n: number): string {
  return `${n}%`;
}
