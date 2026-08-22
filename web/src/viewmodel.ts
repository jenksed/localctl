export function escapeHTML(value: unknown): string {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

export function stateClass(state: unknown): string {
  const normalized = String(state ?? "UNKNOWN").toUpperCase();
  switch (normalized) {
    case "STRONG":
    case "SUPPORTED":
      return "state-supported";
    case "PROMISING":
      return "state-promising";
    case "MIXED":
    case "REVIEW_REQUIRED":
      return "state-mixed";
    case "WEAK":
    case "RUNTIME_UNRELIABLE":
      return "state-weak";
    case "STALE":
      return "state-stale";
    default:
      return "state-unknown";
  }
}

export function formatPercent(value: unknown): string {
  const number = Number(value);
  if (!Number.isFinite(number)) return "—";
  return `${Math.round(number * 100)}%`;
}

export function formatDate(value: unknown): string {
  if (!value) return "—";
  const date = new Date(String(value));
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString();
}

export function terminalJob(state: unknown): boolean {
  return ["COMPLETED", "FAILED", "CANCELLED", "INTERRUPTED"].includes(String(state));
}

export function capabilityLabel(state: unknown, freshness: unknown): string {
  const s = String(state ?? "UNKNOWN");
  const f = String(freshness ?? "UNKNOWN");
  if (s === "UNKNOWN") return "UNKNOWN";
  if (f === "STALE") return "STALE";
  return s;
}
