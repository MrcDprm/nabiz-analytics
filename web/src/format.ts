// Sayı, süre ve değişim biçimlendirme. Saf fonksiyonlar: test/format.test.ts dener.

export function num(n: number, locale: string): string {
  return new Intl.NumberFormat(locale).format(Math.round(n));
}

// Kısa sayı: 1.204 / 12,4 B / 1,2 Mn (kırılım listelerinde değil, eksenlerde).
export function compact(n: number, locale: string): string {
  return new Intl.NumberFormat(locale, { notation: "compact", maximumFractionDigits: 1 }).format(n);
}

export function percent(n: number, locale: string): string {
  return new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 0 }).format(n / 100);
}

// Süre: "1 dk 32 sn", "45 sn", "2 sa 5 dk".
export function duration(seconds: number, lang: "tr" | "en"): string {
  const s = Math.round(seconds);
  const u = lang === "tr" ? { h: "sa", m: "dk", s: "sn" } : { h: "h", m: "m", s: "s" };
  const sep = lang === "tr" ? " " : "";
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const r = s % 60;
  if (h > 0) return `${h}${sep}${u.h} ${m}${sep}${u.m}`;
  if (m > 0) return `${m}${sep}${u.m} ${r}${sep}${u.s}`;
  return `${r}${sep}${u.s}`;
}

export type Delta = { text: string; up: boolean } | null;

// Değişim yüzdesi: önceki dönem 0 ise anlamlı bir yüzde yok, gösterilmez.
export function changePercent(cur: number, prev: number, locale: string): Delta {
  if (prev === 0) return null;
  const ratio = (cur - prev) / prev;
  if (Math.abs(ratio) < 0.005) return { text: "0", up: false };
  const text = new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 0, signDisplay: "always" }).format(ratio);
  return { text: text.replace("-", "−"), up: ratio > 0 };
}

// Oranlardaki değişim yüzde değil puan farkıdır: %44 → %41 = "−3 puan".
export function changePoints(cur: number, prev: number, unit: string): Delta {
  const d = Math.round(cur - prev);
  if (d === 0) return { text: `0 ${unit}`, up: false };
  return { text: `${d > 0 ? "+" : "−"}${Math.abs(d)} ${unit}`, up: d > 0 };
}

export function changeSeconds(cur: number, prev: number, lang: "tr" | "en"): Delta {
  const d = Math.round(cur - prev);
  if (d === 0) return { text: duration(0, lang), up: false };
  return { text: `${d > 0 ? "+" : "−"}${duration(Math.abs(d), lang)}`, up: d > 0 };
}

// Eksen için "güzel" bir üst sınır ve adım: 1, 2 ya da 5 × 10^n.
export function niceScale(max: number, ticks = 5): { max: number; step: number } {
  if (max <= 0) return { max: ticks, step: 1 };
  const raw = max / ticks;
  const pow = 10 ** Math.floor(Math.log10(raw));
  const step = [1, 2, 5, 10].map((m) => m * pow).find((s) => s >= raw) ?? 10 * pow;
  return { max: Math.ceil(max / step) * step, step: Math.max(step, 1) };
}
