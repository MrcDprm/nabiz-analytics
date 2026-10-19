// Zaman grafiği: EKG kâğıdı ızgarası, mürekkep çizgisi (bu dönem) ve kesikli
// kurşun kalem (önceki dönem). İnce/kalın ızgara çizgileri eksen değerlerine denk gelir.
import { clear, s } from "../dom.ts";
import { niceScale } from "../format.ts";

export type Series = {
  values: number[]; // labels'tan kısa olabilir: gelecekteki saatler çizilmez
  previous: number[] | null;
  labels: string[]; // eksen etiketleri (ör. "Pzt 13")
  titles: string[]; // ipucu başlıkları (ör. "13 Ekim Pazartesi")
  format(v: number): string;
  axis(v: number): string;
};

export function drawChart(svg: SVGSVGElement, tooltip: HTMLElement, data: Series) {
  clear(svg);
  const W = svg.clientWidth;
  const H = svg.clientHeight;
  if (!W || !H) return;
  svg.setAttribute("viewBox", `0 0 ${W} ${H}`);
  const narrow = W < 600;
  const left = 44, right = 12, top = 10, bottom = 28;
  const n = data.labels.length;
  const shown = data.values.length;
  const all = [...data.values, ...(data.previous ?? [])];
  const { max, step } = niceScale(Math.max(0, ...all));
  const x = (i: number) => left + (n === 1 ? (W - left - right) / 2 : (i * (W - left - right)) / (n - 1));
  const y = (v: number) => top + (H - bottom - top) * (1 - v / max);

  // Izgara: her ana aralık 5 ince çizgiye bölünür (EKG kâğıdındaki gibi).
  const fine = step / 5;
  for (let v = 0; v <= max + 1e-9; v += fine) {
    const major = Math.abs(v / step - Math.round(v / step)) < 1e-6;
    svg.append(s("line", { class: major ? "grid-bold" : "grid-fine", x1: left, x2: W - right, y1: y(v), y2: y(v) }));
    if (major) svg.append(s("text", { class: "axis", x: left - 8, y: y(v) + 4, "text-anchor": "end" }, data.axis(v)));
  }
  // Dikey çizgiler ve etiketler: çok sayıda çubukta etiketler seyreltilir.
  const every = Math.max(1, Math.ceil(n / (narrow ? 6 : 12)));
  for (let i = 0; i < n; i++) {
    if (i % every !== 0) continue;
    svg.append(s("line", { class: "grid-bold", x1: x(i), x2: x(i), y1: top, y2: H - bottom }));
    svg.append(s("text", { class: "axis", x: x(i), y: H - 8, "text-anchor": "middle" }, data.labels[i] ?? ""));
  }

  const line = (vals: number[]) => vals.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
  if (data.previous) svg.append(s("polyline", { class: "prev", points: line(data.previous) }));
  svg.append(s("polyline", { class: "cur", points: line(data.values) }));

  // Fareyle en yakın çubuğun değeri.
  const dot = s("circle", { class: "dot", r: 5, visibility: "hidden" });
  const guide = s("line", { class: "guide", y1: top, y2: H - bottom, visibility: "hidden" });
  svg.append(guide, dot);
  svg.onpointermove = (e) => {
    const rect = svg.getBoundingClientRect();
    const px = e.clientX - rect.left;
    if (!shown) return;
    const i = Math.max(0, Math.min(shown - 1, Math.round(((px - left) / (W - left - right)) * (n - 1))));
    const v = data.values[i] ?? 0;
    dot.setAttribute("cx", String(x(i)));
    dot.setAttribute("cy", String(y(v)));
    guide.setAttribute("x1", String(x(i)));
    guide.setAttribute("x2", String(x(i)));
    dot.setAttribute("visibility", "visible");
    guide.setAttribute("visibility", "visible");
    tooltip.textContent = `${data.titles[i] ?? ""}: ${data.format(v)}`;
    tooltip.hidden = false;
    const tx = Math.min(Math.max(x(i) - tooltip.offsetWidth / 2, 0), W - tooltip.offsetWidth);
    tooltip.style.transform = `translate(${tx}px, ${Math.max(y(v) - 44, 0)}px)`;
  };
  svg.onpointerleave = () => {
    dot.setAttribute("visibility", "hidden");
    guide.setAttribute("visibility", "hidden");
    tooltip.hidden = true;
  };
}
