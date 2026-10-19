// Olaylar paneli: özel olaylar listesi; bir olaya tıklayınca o olayın özellikleri
// (props) açılır: özellik adları sekme, değerleri liste olur.
import type { Row } from "../api.ts";
import { clear, h } from "../dom.ts";
import { t } from "../i18n.ts";

type Opts = {
  loadEvents(signal: AbortSignal): Promise<Row[]>;
  loadKeys(event: string, signal: AbortSignal): Promise<Row[]>;
  loadValues(event: string, key: string, signal: AbortSignal): Promise<Row[]>;
  count(r: Row): string;
  onFilter(event: string): void;
};

export function createEventsPanel(o: Opts) {
  let event: string | null = null;
  let key: string | null = null;
  let ctrl: AbortController | null = null;

  const title = h("h2", null, t("events"));
  const back = h("button", { type: "button", class: "text-btn", hidden: true }, t("backToEvents"));
  const filterBtn = h("button", { type: "button", class: "text-btn", hidden: true }, t("filterEvent"));
  const tabs = h("div", { class: "tabs", role: "tablist" });
  const colName = h("span", null, t("col.event"));
  const list = h("ul", { class: "rows" });
  const status = h("p", { class: "panel-status", hidden: true });
  const el = h(
    "section",
    { class: "panel" },
    h("div", { class: "panel-head" }, back, title, tabs, filterBtn),
    h("div", { class: "cols", "aria-hidden": "true" }, colName, h("span", null, t("col.visitors"))),
    list,
    status,
  );
  back.addEventListener("click", () => {
    event = key = null;
    refresh();
  });
  filterBtn.addEventListener("click", () => event && o.onFilter(event));

  function render(rows: Row[], onPick: ((r: Row) => void) | null, empty: string) {
    clear(list);
    status.hidden = rows.length > 0;
    status.textContent = empty;
    const top = Math.max(1, ...rows.map((r) => r.visitors));
    for (const r of rows) {
      const body = [h("span", { class: "v" }, r.value), h("span", { class: "n" }, o.count(r))];
      const inner = onPick ? h("button", { type: "button", title: t("showProps"), onclick: () => onPick(r) }, ...body) : h("div", { class: "row-static" }, ...body);
      const li = h("li", null, inner);
      li.style.setProperty("--w", `${(r.visitors / top) * 100}%`);
      list.append(li);
    }
  }

  async function refresh() {
    ctrl?.abort();
    ctrl = new AbortController();
    const signal = ctrl.signal;
    el.setAttribute("aria-busy", "true");
    try {
      if (!event) {
        title.textContent = t("events");
        back.hidden = filterBtn.hidden = true;
        clear(tabs);
        colName.textContent = t("col.event");
        render(await o.loadEvents(signal), (r) => {
          event = r.value;
          key = null;
          refresh();
        }, t("emptyEvents"));
        return;
      }
      title.textContent = event;
      back.hidden = filterBtn.hidden = false;
      const keys = await o.loadKeys(event, signal);
      if (!key || !keys.some((k) => k.value === key)) key = keys[0]?.value ?? null;
      clear(tabs);
      for (const k of keys) {
        tabs.append(
          h("button", {
            type: "button",
            role: "tab",
            "aria-selected": String(k.value === key),
            onclick: () => {
              key = k.value;
              refresh();
            },
          }, k.value),
        );
      }
      colName.textContent = t("col.value");
      render(key ? await o.loadValues(event, key, signal) : [], null, t("noProps"));
    } catch (e) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      clear(list);
      status.hidden = false;
      status.textContent = t("errorGeneric");
    } finally {
      el.removeAttribute("aria-busy");
    }
  }

  return { el, refresh };
}
