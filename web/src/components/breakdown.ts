// Kırılım paneli: başlık, sekmeler ve "değer — ziyaretçi" listesi. Satırın arkasındaki
// dolgu, değerin listedeki en büyük değere oranıdır. Satıra tıklamak o değere göre süzer.
import type { Row } from "../api.ts";
import { ApiError } from "../api.ts";
import { clear, h } from "../dom.ts";
import { t } from "../i18n.ts";

export type Tab = { prop: string; label: string };

export type PanelOptions = {
  title: string;
  tabs: Tab[];
  column(prop: string): string;
  label(prop: string, value: string): { text: string; code?: string };
  count(row: Row): string;
  load(prop: string, limit: number, signal: AbortSignal): Promise<Row[]>;
  onPick(prop: string, value: string): void;
  emptyText?: string;
  wide?: boolean;
};

const PAGE = 10;

export function createPanel(o: PanelOptions) {
  let active = o.tabs[0]!.prop;
  let limit = PAGE;
  let ctrl: AbortController | null = null;

  const list = h("ul", { class: "rows" });
  const status = h("p", { class: "panel-status", hidden: true });
  const more = h("button", { class: "more", type: "button", hidden: true }, t("more"));
  const colName = h("span");
  const tablist = h("div", { class: "tabs", role: "tablist" });
  const heading = h("h2", null, o.title);
  const el = h(
    "section",
    { class: o.wide ? "panel wide" : "panel" },
    h("div", { class: "panel-head" }, heading, o.tabs.length > 1 ? tablist : null),
    h("div", { class: "cols", "aria-hidden": "true" }, colName, h("span", null, t("col.visitors"))),
    list,
    status,
    more,
  );

  function renderTabs() {
    clear(tablist);
    for (const tab of o.tabs) {
      tablist.append(
        h(
          "button",
          {
            type: "button",
            role: "tab",
            "aria-selected": String(tab.prop === active),
            onclick: () => {
              if (tab.prop === active) return;
              active = tab.prop;
              limit = PAGE;
              refresh();
            },
          },
          tab.label,
        ),
      );
    }
  }

  async function refresh() {
    renderTabs();
    colName.textContent = o.column(active);
    ctrl?.abort();
    ctrl = new AbortController();
    el.setAttribute("aria-busy", "true");
    try {
      const rows = await o.load(active, limit + 1, ctrl.signal); // +1: "daha fazla" var mı?
      render(rows.slice(0, limit));
      more.hidden = rows.length <= limit;
    } catch (e) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      clear(list);
      more.hidden = true;
      status.hidden = false;
      status.textContent = e instanceof ApiError && e.status === 0 ? t("errorOffline") : t("errorGeneric");
    } finally {
      el.removeAttribute("aria-busy");
    }
  }

  function render(rows: Row[]) {
    clear(list);
    status.hidden = rows.length > 0;
    status.textContent = o.emptyText ?? t("empty");
    const top = Math.max(1, ...rows.map((r) => r.visitors));
    for (const row of rows) {
      const { text, code } = o.label(active, row.value);
      const btn = h(
        "button",
        { type: "button", title: t("filterHint"), onclick: () => o.onPick(active, row.value) },
        h("span", { class: "v" }, code ? h("span", { class: "code" }, code) : null, text),
        h("span", { class: "n" }, o.count(row)),
      );
      const li = h("li", null, btn);
      li.style.setProperty("--w", `${(row.visitors / top) * 100}%`);
      list.append(li);
    }
  }

  more.addEventListener("click", () => {
    limit += PAGE * 4;
    refresh();
  });

  return { el, refresh };
}
