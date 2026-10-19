// Paket sayfası (/billing): şu anki paket ve kullanım, paket karşılaştırması,
// fatura bilgileri formu, paket değiştirme ve iptal.
import { api, type User } from "../api.ts";
import { clear, h } from "../dom.ts";
import { lang, locale, t, tryT } from "../i18n.ts";
import { num } from "../format.ts";
import { config, footer, header } from "../shell.ts";
import { errText } from "./auth.ts";

type Ctx = { rerender(): void; navigate(p: string): void };
type Plan = { id: string; pageviews: number; sites: number; members: number; monthly_kurus: number; yearly_kurus: number };
type Status = {
  plan: Plan;
  interval?: "monthly" | "yearly";
  state: "free" | "active" | "canceled" | "unpaid";
  period_end?: string;
  usage: { pageviews: number; sites: number; last_month_over: boolean };
};
type Billing = { enabled: boolean; payments?: boolean; plans?: Plan[]; status?: Status };

const REPO = "https://github.com/MrcDprm/nabiz-analytics#installation";

const planName = (id: string) => tryT(`plan.${id}`) ?? id;
const lira = (kurus: number) => new Intl.NumberFormat(locale(), { style: "currency", currency: "TRY", maximumFractionDigits: 0 }).format(kurus / 100);
const day = (iso: string) => new Intl.DateTimeFormat(locale(), { day: "numeric", month: "long", year: "numeric" }).format(new Date(iso));

function meter(label: string, used: number, limit: number): HTMLElement {
  const bar = h("span", { class: "meter-fill" });
  bar.style.setProperty("--w", `${Math.min(100, (used / Math.max(1, limit)) * 100)}%`);
  return h(
    "div",
    { class: used > limit ? "meter over" : "meter" },
    h("div", { class: "meter-head" }, h("span", null, label), h("b", null, `${num(used, locale())} / ${num(limit, locale())}`)),
    h("span", { class: "meter-track", role: "presentation" }, bar),
  );
}

export async function billingView(root: HTMLElement, user: User, ctx: Ctx) {
  const main = h("main", { class: "wrap narrow wide-ish" }, h("h1", null, t("billingTitle")));
  root.replaceChildren(header({ user, rerender: ctx.rerender, navigate: ctx.navigate }), main, footer());

  // Ödemeden dönüş mesajı (adresten okunur, sonra adres temizlenir).
  const status = new URLSearchParams(location.search).get("status");
  if (status && tryT(`status.${status}`)) {
    main.append(h("p", { class: status === "ok" ? "notice ok" : "notice", role: "status" }, t(`status.${status}` as "status.ok")));
    history.replaceState(null, "", "/billing");
  }

  let data: Billing;
  try {
    data = await api<Billing>("/api/billing");
  } catch (e) {
    main.append(h("p", { class: "form-error" }, errText(e)));
    return;
  }
  if (!data.enabled || !data.plans || !data.status) {
    main.append(h("p", { class: "muted" }, t("billingOff")));
    return;
  }
  const st = data.status;
  const out = h("p", { class: "small", role: "status", hidden: true });
  const say = (text: string, ok: boolean) => {
    out.hidden = false;
    out.className = ok ? "small ok" : "form-error";
    out.textContent = text;
  };

  // --- Şu anki paket ve kullanım ---
  const stateText =
    st.state === "active" ? `${t("state.active")}${st.period_end ? `. ${t("renews").replace("{date}", day(st.period_end))}` : ""}`
    : st.state === "canceled" && st.period_end ? t("state.canceled").replace("{date}", day(st.period_end))
    : st.state === "unpaid" && st.period_end ? t("state.unpaid").replace("{date}", day(st.period_end))
    : "";
  const current = h(
    "section",
    { class: "settings-block" },
    h("h2", null, planName(st.plan.id), st.interval ? h("span", { class: "muted small" }, ` (${st.interval === "yearly" ? t("yearly") : t("monthly")})`) : null),
    stateText ? h("p", { class: st.state === "unpaid" ? "form-error" : "muted" }, stateText) : null,
    ...(st.plan.id === "unlimited"
      ? [h("p", { class: "muted" }, `${t("usageMonth")}: ${num(st.usage.pageviews, locale())}. ${t("usageSites")}: ${st.usage.sites}.`)]
      : [meter(t("usageMonth"), st.usage.pageviews, st.plan.pageviews), meter(t("usageSites"), st.usage.sites, st.plan.sites)]),
    st.usage.pageviews > st.plan.pageviews ? h("p", { class: "notice" }, t("overUsage")) : null,
  );
  if (st.state === "active" || st.state === "unpaid") {
    current.append(
      h("button", {
        type: "button",
        class: "text-btn",
        onclick: async () => {
          if (!confirm(t("cancelConfirm"))) return;
          try {
            await api("/api/billing/cancel", { method: "POST" });
            say(t("canceled"), true);
            billingView(root, user, ctx);
          } catch (e) {
            say(errText(e), false);
          }
        },
      }, t("cancelSub")),
    );
  }
  main.append(current, out);
  if (st.plan.id === "unlimited") return; // işletmeci: satın alma ya da sınır yok

  // Ödeme kapalı (ücretsiz barındırma): satın alma yok, kendi sunucuna kurma yolu var.
  if (!data.payments) {
    main.append(
      h("section", { class: "settings-block" },
        h("p", null, t("freeHosting")),
        h("a", { class: "secondary link-btn", href: REPO, rel: "noopener", target: "_blank" }, t("selfHostLink"))),
    );
    return;
  }

  // --- Paket karşılaştırması ---
  let interval: "monthly" | "yearly" = st.interval ?? "monthly";
  const active = st.state === "active";
  const toggle = h("div", { class: "tabs interval", role: "radiogroup", "aria-label": t("col.price") });
  const table = h("table", { class: "plans" });
  const formBox = h("div");

  function renderToggle() {
    clear(toggle);
    for (const [id, label] of [["monthly", t("monthly")], ["yearly", t("yearly")]] as const) {
      toggle.append(h("button", {
        type: "button",
        role: "radio",
        "aria-checked": String(id === interval),
        "aria-selected": String(id === interval),
        disabled: active && id !== st.interval, // aktif abonelikte dönem değişmez
        onclick: () => {
          interval = id;
          renderToggle();
          renderTable();
        },
      }, label));
    }
  }

  function renderTable() {
    clear(table);
    table.append(
      h("thead", null, h("tr", null,
        h("th", { scope: "col" }, t("col.plan")), h("th", { scope: "col" }, t("col.pageviews")), h("th", { scope: "col" }, t("col.sites")),
        h("th", { scope: "col" }, t("col.members")), h("th", { scope: "col" }, t("col.price")), h("td"))),
    );
    const body = h("tbody");
    for (const p of data.plans!) {
      const price = p.id === "free" ? lira(0) : interval === "yearly" ? `${lira(p.yearly_kurus)}${t("perYear")}` : `${lira(p.monthly_kurus)}${t("perMonth")}`;
      let action: Node;
      if (p.id === st.plan.id && (p.id === "free" || st.interval === interval)) {
        action = h("span", { class: "muted small" }, t("current"));
      } else if (p.id === "free") {
        action = h("span");
      } else if (active) {
        action = h("button", {
          type: "button",
          class: "secondary",
          onclick: async () => {
            try {
              await api("/api/billing/change", { method: "POST", body: { plan: p.id, interval } });
              billingView(root, user, ctx);
            } catch (e) {
              say(errText(e), false);
            }
          },
        }, t("switchTo"));
      } else {
        action = h("button", { type: "button", class: "primary", onclick: () => showForm(p) }, t("choose"));
      }
      body.append(h("tr", { class: p.id === st.plan.id ? "is-current" : null },
        h("th", { scope: "row" }, planName(p.id)),
        h("td", { "data-label": t("col.pageviews") }, num(p.pageviews, locale())),
        h("td", { "data-label": t("col.sites") }, String(p.sites)),
        h("td", { "data-label": t("col.members") }, p.members === 1 ? t("onlyYou") : `${p.members} ${t("people")}`),
        h("td", { "data-label": t("col.price") }, price),
        h("td", { class: "act" }, action)));
    }
    table.append(body);
  }

  // --- Fatura bilgileri formu ---
  function showForm(p: Plan) {
    clear(formBox);
    const f = (id: string, label: string, attrs: Record<string, string | boolean> = {}) => {
      const input = h("input", { id, name: id, required: true, ...attrs });
      return { input, wrap: h("div", { class: "field" }, h("label", { for: id }, label), input) };
    };
    const name = f("name", t("firstName"), { autocomplete: "given-name" });
    const surname = f("surname", t("lastName"), { autocomplete: "family-name" });
    const phone = f("phone", t("phone"), { type: "tel", autocomplete: "tel", placeholder: "+90 555 111 22 33" });
    const identity = f("identity", t("identity"), { inputmode: "numeric", maxlength: "11", pattern: "[0-9]{11}" });
    const address = f("address", t("address"), { autocomplete: "street-address" });
    const city = f("city", t("city"), { autocomplete: "address-level2" });
    const country = f("country", t("country"), { autocomplete: "country-name", value: lang === "tr" ? "Türkiye" : "Turkey" });
    const accept = h("input", { type: "checkbox", id: "accept", required: true });
    const submit = h("button", { type: "submit", class: "primary" }, t("goToPayment"));
    const err = h("p", { class: "form-error", role: "alert", hidden: true });
    const form = h(
      "form",
      { class: "invoice" },
      h("h2", null, `${planName(p.id)}, ${interval === "yearly" ? `${lira(p.yearly_kurus)}${t("perYear")}` : `${lira(p.monthly_kurus)}${t("perMonth")}`}`),
      h("h3", null, t("invoiceTitle")),
      h("p", { class: "muted small" }, t("invoiceHelp")),
      h("div", { class: "grid2" }, name.wrap, surname.wrap, phone.wrap, identity.wrap),
      h("p", { class: "muted small" }, t("identityHelp")),
      address.wrap,
      h("div", { class: "grid2" }, city.wrap, country.wrap),
      h("label", { class: "check terms" }, accept, h("span", null, t("acceptTerms"), " ",
        h("a", { href: config().sales_contract_url, target: "_blank", rel: "noopener" }, t("salesContract")), ", ",
        h("a", { href: config().pre_info_url, target: "_blank", rel: "noopener" }, t("preInfo")))),
      err,
      submit,
    );
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      err.hidden = true;
      submit.disabled = true;
      try {
        const res = await api<{ url: string }>("/api/billing/checkout", {
          method: "POST",
          body: {
            plan: p.id, interval, accept_terms: accept.checked,
            name: name.input.value, surname: surname.input.value, phone: phone.input.value, identity: identity.input.value,
            address: address.input.value, city: city.input.value, country: country.input.value,
          },
        });
        // Ödeme sayfası sunucunun kendi sayfası (farklı CSP); tam sayfa geçiş.
        location.assign(res.url);
      } catch (e2) {
        err.hidden = false;
        err.textContent = errText(e2);
        submit.disabled = false;
      }
    });
    formBox.append(form);
    form.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth" });
    name.input.focus({ preventScroll: true });
  }

  renderToggle();
  renderTable();
  main.append(
    h("section", { class: "settings-block" }, h("div", { class: "plans-head" }, h("h2", null, t("plan")), toggle), table,
      active ? h("p", { class: "muted small" }, t("changeIntervalHint")) : null),
    formBox,
  );
}
