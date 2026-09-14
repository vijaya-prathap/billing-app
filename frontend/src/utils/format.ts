const timeFormatter = new Intl.DateTimeFormat(undefined, {
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
});

export function formatTime(date: Date | null): string {
  return date ? timeFormatter.format(date) : "—";
}
