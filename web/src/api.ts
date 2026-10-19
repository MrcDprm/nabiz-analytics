// Sunucuyla konuşan tek yer. Çerez aynı adrese otomatik gider (HttpOnly; JavaScript göremez).

export class ApiError extends Error {
  status: number; // 0: sunucuya ulaşılamadı
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function api<T>(path: string, init?: { method?: string; body?: unknown; signal?: AbortSignal }): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method: init?.method ?? "GET",
      credentials: "same-origin",
      headers: init?.body !== undefined ? { "Content-Type": "application/json" } : undefined,
      body: init?.body !== undefined ? JSON.stringify(init.body) : undefined,
      signal: init?.signal,
    });
  } catch (e) {
    if (e instanceof DOMException && e.name === "AbortError") throw e;
    throw new ApiError(0, "");
  }
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(res.status, typeof data?.error === "string" ? data.error : "");
  return data as T;
}

export type User = { email: string; verified: boolean; locale: string };
export type Site = { id: string; domain: string; public: boolean; role: string };

export type Metrics = {
  visitors: number;
  visits: number;
  pageviews: number;
  bounce_rate: number;
  visit_length: number;
};

export type Range = { from: string; to: string; prev_from: string; prev_to: string; interval: "hour" | "day" | "month" };
export type Summary = { range: Range; summary: { current: Metrics; previous: Metrics } };
export type Point = Metrics & { start: string };
export type Timeseries = { range: Range; points: Point[] };
export type Row = { value: string; visitors: number; count: number };
export type Breakdown = { property: string; rows: Row[] };
export type Live = { visitors: number; beats: number[] };
