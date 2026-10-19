// Bir sitenin paneli: nabız şeridi, ölçüler, zaman grafiği, filtreler ve kırılımlar.
import { api, ApiError, type Breakdown, type GoalResult, type Live, type Metrics, type Site, type Summary, type Timeseries, type User } from "../api.ts";
import { clear, h, s } from "../dom.ts";
import { changePercent, changePoints, changeSeconds, compact, duration, num, percent, type Delta } from "../format.ts";
import { lang, locale, t, tryT } from "../i18n.ts";
import { footer, header } from "../shell.ts";
import { apiQuery, parseState, periods, stateSearch, withFilter, withoutFilter, type Metric, type State } from "../state.ts";
import { createPanel } from "../components/breakdown.ts";
import { drawChart } from "../components/chart.ts";
import { createStrip } from "../components/strip.ts";

const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
const LIVE_EVERY = 15_000;

type Ctx = { rerender(): void; navigate(p: string): void; onLeave(fn: () => void): void };

export function dashboardView(root: HTMLElement, domain: string, user: User | null, sites: Site[], ctx: Ctx) {
  let state: State = parseState(location.search);
  const base = `/api/stats/${encodeURIComponent(domain)}`;
  let loadCtrl: AbortController | null = null;

  // --- Üst çubuk: site seçici ve dönem ---
  const middle: Node[] = [];
  if (sites.length > 1) {
    const sel = h("select", { class: "site-picker", "aria-label": t("sites") });
    for (const st of sites) sel.append(h("option", { value: st.domain, selected: st.domain === domain }, st.domain));
    sel.addEventListener("change", () => ctx.navigate(`/${sel.value}${stateSearch({ ...state, filters: [] })}`));
    middle.push(sel);
  } else {
    middle.push(h("span", { class: "site-name" }, domain));
  }
  const mine = sites.find((st) => st.domain === domain);
  if (mine && (mine.role === "owner" || mine.role === "admin")) {
    middle.push(h("a", { class: "text-btn", href: `/settings/${domain}` }, t("settings")));
  }

  const periodSel = h("select", { class: "period", "aria-label": t("period") });
  for (const p of periods) periodSel.append(h("option", { value: p, selected: p === state.period }, tryT(`period.${p}`) ?? p));
  const fromIn = h("input", { type: "date", "aria-label": t("from"), value: state.from });
  const toIn = h("input", { type: "date", "aria-label": t("to"), value: state.to });
  const custom = h(
    "form",
    { class: "custom-range", hidden: state.period !== "custom" },
    fromIn,
    toIn,
    h("button", { type: "submit", class: "text-btn" }, t("apply")),
  );
  periodSel.addEventListener("change", () => {
    if (periodSel.value === "custom") {
      custom.hidden = false;
      fromIn.focus();
      return;
    }
    custom.hidden = true;
    update({ ...state, period: periodSel.value as State["period"] });
  });
  custom.addEventListener("submit", (e) => {
    e.preventDefault();
    if (!fromIn.value || !toIn.value) return;
    update({ ...state, period: "custom", from: fromIn.value, to: toIn.value });
  });

  // --- Nabız şeridi ---
  const strip = createStrip();
  const liveN = h("span", { class: "n" }, "0");
  const liveT = h("span", { class: "t" }, t("liveMany"));
  const stripEl = h(
    "section",
    { class: "strip", "aria-label": t("liveLabel") },
    strip.el,
    h("div", { class: "live", "aria-live": "polite" }, liveN, liveT),
    h("div", { class: "strip-note" }, t("liveNote")),
  );

  // --- Ölçüler ---
  const metricKeys: Metric[] = ["visitors", "pageviews", "bounce_rate", "visit_length"];
  const vitalEls = new Map<Metric, { btn: HTMLButtonElement; value: HTMLElement; delta: HTMLElement }>();
  const vitals = h("div", { class: "vitals", role: "group", "aria-label": t("metricGroup") });
  for (const m of metricKeys) {
    const value = h("span", { class: "value" }, "–");
    const delta = h("span", { class: "delta" });
    const btn = h(
      "button",
      { type: "button", class: "vital", "aria-pressed": String(m === state.metric), onclick: () => update({ ...state, metric: m }) },
      h("span", { class: "label" }, t(m)),
      h("span", { class: "row" }, value, delta),
    );
    vitalEls.set(m, { btn, value, delta });
    vitals.append(btn);
  }

  // --- Grafik ---
  const chartSvg = s("svg", { class: "chart-svg", role: "img", "aria-label": t("chartLabel") });
  const tooltip = h("div", { class: "tip", hidden: true });
  const legendCur = h("span");
  const legendPrev = h("span", { class: "p" });
  const notice = h("p", { class: "notice", hidden: true }, t("noData"));
  const chart = h("section", { class: "chart" }, notice, h("div", { class: "chart-box" }, chartSvg, tooltip), h("div", { class: "legend" }, legendCur, legendPrev));
  let lastSeries: Timeseries | null = null;
  let lastPrev: Timeseries["points"] | null = null;

  // --- Filtre satırı ---
  const filterBar = h("div", { class: "filterbar" });
  function renderFilters() {
    clear(filterBar);
    filterBar.hidden = state.filters.length === 0;
    if (!state.filters.length) return;
    filterBar.append(h("span", { class: "lbl" }, t("filtering")));
    for (const [dim, value] of state.filters) {
      filterBar.append(
        h(
          "span",
          { class: "chip" },
          `${tryT(`dim.${dim}`) ?? dim}: ${valueLabel(dim, value).text}`,
          h("button", { type: "button", "aria-label": t("removeFilter"), onclick: () => update(withoutFilter(state, dim)) }, "✕"),
        ),
      );
    }
    if (state.filters.length > 1) {
      filterBar.append(h("button", { type: "button", class: "text-btn", onclick: () => update({ ...state, filters: [] }) }, t("clearFilters")));
    }
  }

  // --- Kırılımlar ---
  const regionNames = new Intl.DisplayNames([locale()], { type: "region" });
  function valueLabel(prop: string, value: string): { text: string; code?: string } {
    if (prop === "source") return { text: value || t("direct") };
    if (prop.startsWith("utm_")) return { text: value || t("noUtm") };
    if (prop === "country") {
      if (!value) return { text: t("unknown") };
      let name = value;
      try {
        name = regionNames.of(value) ?? value;
      } catch {
        /* geçersiz kod: olduğu gibi göster */
      }
      return { text: name, code: value };
    }
    if (prop === "device") return { text: value ? (tryT(`device.${value}`) ?? value) : t("unknown") };
    if (prop === "browser" || prop === "os") return { text: !value ? t("unknown") : value === "Other" ? t("other") : value };
    return { text: value };
  }
  const load = (prop: string, limit: number, signal: AbortSignal) =>
    api<Breakdown>(`${base}/breakdown?${apiQuery(state, tz, { property: prop, limit: String(limit) })}`, { signal }).then((b) => b.rows);
  const pick = (prop: string, value: string) => update(withFilter(state, [prop, value]));
  const count = (r: { visitors: number }) => num(r.visitors, locale());
  const tabs = (props: string[]) => props.map((p) => ({ prop: p, label: tryT(`tab.${p}`) ?? p }));

  const panels = [
    createPanel({ title: t("pages"), tabs: tabs(["page", "entry", "exit"]), column: () => t("col.page"), label: valueLabel, count, load, onPick: pick }),
    createPanel({ title: t("sources"), tabs: tabs(["source", "utm_source", "utm_medium", "utm_campaign"]), column: () => t("col.source"), label: valueLabel, count, load, onPick: pick }),
    createPanel({ title: t("countries"), tabs: [{ prop: "country", label: t("countries") }], column: () => t("col.country"), label: valueLabel, count, load, onPick: pick }),
    createPanel({ title: t("devices"), tabs: tabs(["device", "browser", "os"]), column: () => t("col.device"), label: valueLabel, count, load, onPick: pick }),
    createPanel({ title: t("events"), tabs: [{ prop: "event", label: t("events") }], column: () => t("col.event"), label: valueLabel, count, load, onPick: pick, emptyText: t("emptyEvents") }),
  ];

  // --- Hedefler: ulaşan kişi ve dönüşüm oranı ---
  const goalRows = h("ul", { class: "rows" });
  const goalStatus = h("p", { class: "panel-status", hidden: true });
  const goalsPanel = h(
    "section",
    { class: "panel" },
    h("div", { class: "panel-head" }, h("h2", null, t("goals"))),
    h("div", { class: "cols", "aria-hidden": "true" }, h("span", null, t("col.goal")), h("span", null, t("col.conversion"))),
    goalRows,
    goalStatus,
  );
  async function loadGoals(signal: AbortSignal) {
    try {
      const { goals } = await api<{ goals: GoalResult[] }>(`${base}/goals?${apiQuery(state, tz)}`, { signal });
      clear(goalRows);
      goalStatus.hidden = goals.length > 0;
      goalStatus.textContent = t("noGoalsDash");
      const top = Math.max(1, ...goals.map((g) => g.visitors));
      const rate = new Intl.NumberFormat(locale(), { style: "percent", maximumFractionDigits: 1 });
      for (const g of goals) {
        // "/blog/*" gibi joker hedefler tek bir sayfa değil; onlara göre süzülmez.
        const exact = !(g.kind === "page" && g.value.endsWith("*"));
        const body = [
          h("span", { class: "v" }, g.value),
          h("span", { class: "n" }, h("span", { class: "muted" }, `${num(g.visitors, locale())} ${t("people")}`), "  ", rate.format(g.rate / 100)),
        ];
        const inner = exact
          ? h("button", { type: "button", title: t("filterHint"), onclick: () => update(withFilter(state, [g.kind === "page" ? "page" : "event", g.value])) }, ...body)
          : h("div", { class: "row-static" }, ...body);
        const li = h("li", null, inner);
        li.style.setProperty("--w", `${(g.visitors / top) * 100}%`);
        goalRows.append(li);
      }
    } catch (e) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      goalStatus.hidden = false;
      goalStatus.textContent = t("errorGeneric");
    }
  }

  const main = h(
    "main",
    { class: "wrap" },
    vitals,
    chart,
    filterBar,
    h("div", { class: "panels" }, ...panels.slice(0, 4).map((p) => p.el), goalsPanel, panels[4]!.el),
  );
  root.replaceChildren(
    header({ user, middle, right: [periodSel, custom], rerender: ctx.rerender, navigate: ctx.navigate }),
    stripEl,
    main,
    footer(),
  );

  // --- Veri ---
  function update(next: State) {
    state = next;
    history.replaceState(null, "", `/${domain}${stateSearch(state)}`);
    periodSel.value = state.period;
    custom.hidden = state.period !== "custom";
    for (const [m, el] of vitalEls) el.btn.setAttribute("aria-pressed", String(m === state.metric));
    loadAll();
  }

  function setVitals(cur: Metrics, prev: Metrics) {
    const loc = locale();
    const show: Record<Metric, [string, Delta]> = {
      visitors: [num(cur.visitors, loc), changePercent(cur.visitors, prev.visitors, loc)],
      pageviews: [num(cur.pageviews, loc), changePercent(cur.pageviews, prev.pageviews, loc)],
      bounce_rate: [percent(cur.bounce_rate, loc), cur.visits && prev.visits ? changePoints(cur.bounce_rate, prev.bounce_rate, t("points")) : null],
      visit_length: [duration(cur.visit_length, lang), cur.visits && prev.visits ? changeSeconds(cur.visit_length, prev.visit_length, lang) : null],
    };
    for (const [m, [value, delta]] of Object.entries(show) as [Metric, [string, Delta]][]) {
      const el = vitalEls.get(m)!;
      el.value.textContent = value;
      el.delta.textContent = delta?.text ?? "";
      el.delta.title = t("previous");
      el.delta.classList.toggle("up", !!delta?.up);
    }
  }

  function fmtMetric(m: Metric, v: number): string {
    if (m === "bounce_rate") return percent(v, locale());
    if (m === "visit_length") return duration(v, lang);
    return num(v, locale());
  }
  function axisMetric(m: Metric, v: number): string {
    if (m === "bounce_rate") return percent(v, locale());
    if (m === "visit_length") return duration(v, lang);
    return compact(v, locale());
  }

  function renderChart() {
    if (!lastSeries) return;
    const m = state.metric;
    const { interval } = lastSeries.range;
    const pts = lastSeries.points;
    const dt = (o: Intl.DateTimeFormatOptions) => new Intl.DateTimeFormat(locale(), { timeZone: tz, ...o });
    const narrow = chartSvg.clientWidth < 600;
    const axisFmt =
      interval === "hour" ? dt({ hour: "2-digit", minute: "2-digit" })
      : interval === "month" ? dt({ month: "short" })
      : narrow ? dt({ day: "numeric" }) : dt({ weekday: "short", day: "numeric" });
    const tipFmt =
      interval === "hour" ? dt({ day: "numeric", month: "long", hour: "2-digit", minute: "2-digit" })
      : interval === "month" ? dt({ month: "long", year: "numeric" })
      : dt({ weekday: "long", day: "numeric", month: "long" });
    // Henüz gelmemiş saat/gün 0 gibi çizilmesin: çizgi şimdiki zamanda biter.
    const now = Date.now();
    const past = pts.filter((p) => new Date(p.start).getTime() <= now);
    drawChart(chartSvg, tooltip, {
      values: past.map((p) => p[m]),
      previous: lastPrev ? pts.map((_, i) => lastPrev![i]?.[m] ?? 0) : null,
      labels: pts.map((p) => axisFmt.format(new Date(p.start))),
      titles: pts.map((p) => tipFmt.format(new Date(p.start))),
      format: (v) => fmtMetric(m, v),
      axis: (v) => axisMetric(m, v),
    });
    const span = (a: string, b: string) => dt({ day: "numeric", month: "short" }).formatRange(new Date(a), new Date(new Date(b).getTime() - 1));
    const r = lastSeries.range;
    legendCur.textContent = interval === "hour" ? dt({ day: "numeric", month: "long" }).format(new Date(r.from)) : span(r.from, r.to);
    legendPrev.textContent = interval === "hour" ? dt({ day: "numeric", month: "long" }).format(new Date(r.prev_from)) : span(r.prev_from, r.prev_to);
  }

  async function loadAll() {
    renderFilters();
    loadCtrl?.abort();
    loadCtrl = new AbortController();
    const signal = loadCtrl.signal;
    for (const p of panels) p.refresh();
    loadGoals(signal);
    try {
      const [sum, series] = await Promise.all([
        api<Summary>(`${base}/summary?${apiQuery(state, tz)}`, { signal }),
        api<Timeseries & { previous: Timeseries["points"] }>(`${base}/timeseries?${apiQuery(state, tz, { compare: "1" })}`, { signal }),
      ]);
      setVitals(sum.summary.current, sum.summary.previous);
      notice.hidden = !(sum.summary.current.visits === 0 && sum.summary.previous.visits === 0 && state.filters.length === 0);
      lastSeries = series;
      lastPrev = series.previous ?? null;
      renderChart();
    } catch (e) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      if (e instanceof ApiError && e.status === 404) return ctx.rerender();
      notice.hidden = false;
      notice.textContent = e instanceof ApiError && e.status === 0 ? t("errorOffline") : t("errorGeneric");
    }
  }

  async function loadLive() {
    if (document.hidden) return;
    try {
      const live = await api<Live>(`${base}/live`);
      liveN.textContent = num(live.visitors, locale());
      liveT.textContent = live.visitors === 1 ? t("liveOne") : t("liveMany");
      strip.setBeats(live.beats);
    } catch {
      /* bir sonraki denemede düzelir */
    }
  }

  const liveTimer = setInterval(loadLive, LIVE_EVERY);
  const onVisible = () => !document.hidden && loadLive();
  document.addEventListener("visibilitychange", onVisible);
  let resizeTimer = 0;
  const onResize = () => {
    clearTimeout(resizeTimer);
    resizeTimer = window.setTimeout(renderChart, 120);
  };
  addEventListener("resize", onResize);
  ctx.onLeave(() => {
    clearInterval(liveTimer);
    document.removeEventListener("visibilitychange", onVisible);
    removeEventListener("resize", onResize);
    loadCtrl?.abort();
    strip.destroy();
  });

  loadAll();
  loadLive();
}
