// Huni paneli: her adımda kalan ziyaretçi; satır dolgusu ilk adıma oran,
// yanında bir önceki adıma göre kayıp.
import type { FunnelStats } from "../api.ts";
import { clear, h } from "../dom.ts";
import { locale, t } from "../i18n.ts";
import { num, share } from "../format.ts";

type Opts = {
  load(id: string | null, signal: AbortSignal): Promise<FunnelStats>;
  settingsHref: string | null; // yetkisi olmayan için ayarlar bağlantısı gösterilmez
};

export function createFunnelPanel(o: Opts) {
  let selected: string | null = null;
  let ctrl: AbortController | null = null;
  const tabs = h("div", { class: "tabs", role: "tablist" });
  const list = h("ol", { class: "rows funnel" });
  const status = h("p", { class: "panel-status", hidden: true });
  const el = h(
    "section",
    { class: "panel wide" },
    h("div", { class: "panel-head" }, h("h2", null, t("funnels")), tabs),
    h("div", { class: "cols", "aria-hidden": "true" }, h("span", null, t("col.step")), h("span", null, t("col.reached"))),
    list,
    status,
  );

  async function refresh() {
    ctrl?.abort();
    ctrl = new AbortController();
    el.setAttribute("aria-busy", "true");
    try {
      const data = await o.load(selected, ctrl.signal);
      selected = data.selected ?? null;
      clear(tabs);
      for (const f of data.funnels) {
        tabs.append(
          h("button", {
            type: "button",
            role: "tab",
            "aria-selected": String(f.id === selected),
            onclick: () => {
              selected = f.id;
              refresh();
            },
          }, f.name),
        );
      }
      clear(list);
      const steps = data.steps ?? [];
      status.hidden = steps.length > 0;
      clear(status);
      if (!steps.length) {
        status.append(t("noFunnelsDash"), " ");
        if (o.settingsHref) status.append(h("a", { href: o.settingsHref }, t("settings")));
      }
      steps.forEach((st, i) => {
        const li = h(
          "li",
          null,
          h(
            "div",
            { class: "row-static" },
            h("span", { class: "v" }, h("span", { class: "code" }, String(i + 1)), st.value, i > 0 && st.drop > 0 ? h("span", { class: "muted drop" }, ` −${share(st.drop, locale())}`) : null),
            h("span", { class: "n" }, h("span", { class: "muted" }, i > 0 ? `${share(st.rate, locale())} ${t("ofFirst")}` : ""), "  ", num(st.visitors, locale())),
          ),
        );
        li.style.setProperty("--w", `${st.rate}%`);
        list.append(li);
      });
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
