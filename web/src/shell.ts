// Her sayfada ortak olan üst çubuk ve alt bilgi; tema ve dil düğmeleri.
import { api, type User } from "./api.ts";
import { h } from "./dom.ts";
import { lang, setLang, t } from "./i18n.ts";

type Theme = "light" | "dark";

function savedTheme(): Theme | null {
  try {
    const v = localStorage.getItem("nabiz-theme");
    return v === "light" || v === "dark" ? v : null;
  } catch {
    return null;
  }
}

export function applyTheme() {
  const v = savedTheme();
  if (v) document.documentElement.dataset.theme = v;
  else delete document.documentElement.dataset.theme; // sistem ayarı geçerli
}

function currentTheme(): Theme {
  const v = document.documentElement.dataset.theme;
  if (v === "light" || v === "dark") return v;
  return matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function brandMark(): SVGSVGElement {
  const NS = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("viewBox", "0 0 30 18");
  svg.setAttribute("aria-hidden", "true");
  const p = document.createElementNS(NS, "polyline");
  p.setAttribute("points", "0,11 8,11 10,8 12,11 14,11 16,2 18,16 20,11 30,11");
  svg.append(p);
  return svg;
}

export type ShellOptions = {
  middle?: Node[]; // site seçici vb.
  right?: Node[]; // dönem seçici vb.
  user: User | null;
  hideAccount?: boolean; // giriş sayfasında "Giriş yap" bağlantısı gereksiz
  rerender(): void;
  navigate(path: string): void;
};

export function header(o: ShellOptions): HTMLElement {
  const themeBtn = h(
    "button",
    {
      type: "button",
      class: "icon-btn",
      "aria-label": t("themeToggle"),
      title: t("themeToggle"),
      onclick: () => {
        const next: Theme = currentTheme() === "dark" ? "light" : "dark";
        document.documentElement.dataset.theme = next;
        try {
          localStorage.setItem("nabiz-theme", next);
        } catch {
          /* tercih hatırlanmaz */
        }
      },
    },
    h("span", { class: "theme-glyph", "aria-hidden": "true" }),
  );
  const langBtn = h(
    "button",
    {
      type: "button",
      class: "lang",
      "aria-label": t("langToggle"),
      onclick: () => {
        setLang(lang === "tr" ? "en" : "tr");
        if (o.user) api("/api/account", { method: "PATCH", body: { locale: lang } }).catch(() => {});
        o.rerender();
      },
    },
    h("b", null, lang === "tr" ? "TR" : "EN"),
    " / ",
    lang === "tr" ? "EN" : "TR",
  );
  const account = o.hideAccount
    ? null
    : o.user
    ? h(
        "button",
        {
          type: "button",
          class: "text-btn",
          onclick: async () => {
            await api("/api/logout", { method: "POST" }).catch(() => {});
            o.navigate("/login");
          },
        },
        t("logout"),
      )
    : h("a", { class: "text-btn", href: "/login" }, t("login"));

  return h(
    "header",
    { class: "top" },
    h(
      "div",
      { class: "wrap" },
      h("a", { class: "brand", href: "/", "aria-label": "Nabız" }, brandMark(), "Nabız"),
      ...(o.middle ?? []),
      h("div", { class: "spacer" }),
      ...(o.right ?? []),
      langBtn,
      themeBtn,
      o.user && !o.hideAccount ? h("a", { class: "text-btn", href: "/account" }, t("account")) : null,
      account,
    ),
  );
}

export function footer(): HTMLElement {
  return h(
    "footer",
    { class: "wrap foot" },
    h("span", null, t("footerPrivacy")),
    h("a", { href: "https://db-ip.com", rel: "noopener" }, t("footerGeo")),
  );
}
