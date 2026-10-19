// Panelin durumu (dönem, filtreler, ölçü) adreste durur: sayfa yenilenince ya da
// bağlantı paylaşılınca aynı görünüm açılır.

export const periods = ["day", "7d", "30d", "month", "12m", "custom"] as const;
export type Period = (typeof periods)[number];
export const metrics = ["visitors", "pageviews", "bounce_rate", "visit_length"] as const;
export type Metric = (typeof metrics)[number];
export type Filter = [dim: string, value: string];

export type State = {
  period: Period;
  from: string;
  to: string;
  metric: Metric;
  filters: Filter[];
};

const dims = new Set(["page", "entry", "exit", "source", "utm_source", "utm_medium", "utm_campaign", "country", "device", "browser", "os", "event"]);
const date = /^\d{4}-\d{2}-\d{2}$/;

export function parseState(search: string): State {
  const q = new URLSearchParams(search);
  const period = (periods as readonly string[]).includes(q.get("period") ?? "") ? (q.get("period") as Period) : "7d";
  const metric = (metrics as readonly string[]).includes(q.get("metric") ?? "") ? (q.get("metric") as Metric) : "visitors";
  let filters: Filter[] = [];
  try {
    const raw = JSON.parse(q.get("f") ?? "[]");
    if (Array.isArray(raw)) {
      filters = raw.filter(
        (f): f is Filter => Array.isArray(f) && f.length === 2 && dims.has(f[0]) && typeof f[1] === "string",
      );
    }
  } catch {
    /* bozuk adres: filtresiz aç */
  }
  const from = date.test(q.get("from") ?? "") ? q.get("from")! : "";
  const to = date.test(q.get("to") ?? "") ? q.get("to")! : "";
  return { period: period === "custom" && (!from || !to) ? "7d" : period, from, to, metric, filters };
}

export function stateSearch(s: State): string {
  const q = new URLSearchParams();
  if (s.period !== "7d") q.set("period", s.period);
  if (s.period === "custom") {
    q.set("from", s.from);
    q.set("to", s.to);
  }
  if (s.metric !== "visitors") q.set("metric", s.metric);
  if (s.filters.length) q.set("f", JSON.stringify(s.filters));
  const str = q.toString();
  return str ? `?${str}` : "";
}

// API sorgusu: dönem + saat dilimi + filtreler.
export function apiQuery(s: State, tz: string, extra: Record<string, string> = {}): string {
  const q = new URLSearchParams({ period: s.period, tz, ...extra });
  if (s.period === "custom") {
    q.set("from", s.from);
    q.set("to", s.to);
  }
  if (s.filters.length) q.set("filters", JSON.stringify(s.filters));
  return q.toString();
}

// Aynı boyutta ikinci filtre eklenmez, eskisinin yerini alır.
export function withFilter(s: State, f: Filter): State {
  return { ...s, filters: [...s.filters.filter(([d]) => d !== f[0]), f] };
}

export function withoutFilter(s: State, dim: string): State {
  return { ...s, filters: s.filters.filter(([d]) => d !== dim) };
}
