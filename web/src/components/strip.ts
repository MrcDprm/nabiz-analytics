// Nabız şeridi: EKG kâğıdında akan bir çizgi. Son 5 dakikadaki her sayfa
// görüntüleme, kendi zamanında bir kalp atışı kompleksi (P, QRS, T) olarak çizilir.
import { s } from "../dom.ts";

const WINDOW = 5 * 60; // saniye
const PEN = 0.94; // kalemin ucu şeridin %94'ünde; sağında boşluk kalsın

export function createStrip(): { el: SVGSVGElement; setBeats(beats: number[]): void; destroy(): void } {
  const trace = s("polyline", { class: "trace" });
  const pen = s("circle", { class: "pen", r: 4.5 });
  const el = s("svg", { class: "ecg", "aria-hidden": "true" }, trace, pen);
  let beats: number[] = [];
  let frame = 0;
  let last = 0;
  const reduced = matchMedia("(prefers-reduced-motion: reduce)");

  function draw(nowMs: number) {
    const W = el.clientWidth;
    const H = el.clientHeight;
    if (!W || !H) return;
    el.setAttribute("viewBox", `0 0 ${W} ${H}`);
    const base = H * 0.62;
    const amp = Math.min(66, H * 0.5);
    const penX = W * PEN;
    const now = nowMs / 1000;
    const pts: number[][] = [[0, base]];
    for (const ts of beats) {
      const x = penX - ((now - ts) / WINDOW) * penX;
      if (x < -40 || x > penX) continue;
      pts.push(
        [x - 26, base], [x - 20, base - amp * 0.09], [x - 14, base], [x - 8, base],
        [x - 4, base + amp * 0.14], [x, base - amp], [x + 5, base + amp * 0.27], [x + 10, base],
        [x + 22, base], [x + 32, base - amp * 0.18], [x + 42, base],
      );
    }
    // Birbirine çok yakın atışlarda çizgi geri gitmesin.
    const clipped = pts.filter((p) => p[0]! <= penX).sort((a, b) => a[0]! - b[0]!);
    clipped.push([penX, base]);
    trace.setAttribute("points", clipped.map((p) => `${p[0]!.toFixed(1)},${p[1]!.toFixed(1)}`).join(" "));
    pen.setAttribute("cx", penX.toFixed(1));
    pen.setAttribute("cy", base.toFixed(1));
  }

  // Saniyede ~20 kare yeter: şerit 5 dakikada bir boy kayıyor, göz farkı görmez.
  function loop(t: number) {
    if (t - last > 50) {
      draw(Date.now());
      last = t;
    }
    frame = requestAnimationFrame(loop);
  }

  function start() {
    cancelAnimationFrame(frame);
    if (reduced.matches || document.hidden) draw(Date.now());
    else frame = requestAnimationFrame(loop);
  }
  document.addEventListener("visibilitychange", start);
  reduced.addEventListener("change", start);
  addEventListener("resize", start);

  return {
    el,
    setBeats(b) {
      beats = b;
      start();
    },
    destroy() {
      cancelAnimationFrame(frame);
      document.removeEventListener("visibilitychange", start);
      reduced.removeEventListener("change", start);
      removeEventListener("resize", start);
    },
  };
}
