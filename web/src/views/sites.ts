import { api, type Site, type User } from "../api.ts";
import { errText } from "./auth.ts";
import { verifyBanner } from "./account.ts";
import { h } from "../dom.ts";
import { t } from "../i18n.ts";
import { footer, header } from "../shell.ts";
import { snippet } from "../components/snippet.ts";

type Ctx = { rerender(): void; navigate(p: string): void };

export function sitesView(root: HTMLElement, user: User, sites: Site[], ctx: Ctx) {
  const list = h("ul", { class: "site-list" });
  for (const s of sites) {
    list.append(
      h(
        "li",
        null,
        h("a", { href: `/${s.domain}` }, s.domain),
        s.public ? h("span", { class: "muted" }, t("public")) : null,
      ),
    );
  }

  // Site ekleme formu. Eklenince kurulum satırı gösterilir.
  const input = h("input", { id: "domain", name: "domain", placeholder: "example.com", autocomplete: "off", required: true, inputmode: "url" });
  const error = h("p", { class: "form-error", role: "alert", hidden: true });
  const submit = h("button", { type: "submit", class: "primary" }, t("addSite"));
  const result = h("div", { class: "added", hidden: true });
  const form = h(
    "form",
    { class: "add-site" },
    h("h2", null, t("addSite")),
    h("label", { for: "domain" }, t("domain")),
    h("div", { class: "inline" }, input, submit),
    h("p", { class: "muted small" }, t("addSiteHint")),
    error,
  );
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    error.hidden = true;
    submit.disabled = true;
    submit.textContent = t("adding");
    try {
      const site = await api<Site>("/api/sites", { method: "POST", body: { domain: input.value } });
      form.hidden = true;
      result.hidden = false;
      result.replaceChildren(
        h("h2", null, site.domain),
        h("h3", null, t("snippetTitle")),
        snippet(site.domain),
        h("a", { class: "primary link-btn", href: `/${site.domain}` }, t("openDashboard")),
      );
    } catch (err) {
      error.hidden = false;
      error.textContent = errText(err);
    } finally {
      submit.disabled = false;
      submit.textContent = t("addSite");
    }
  });

  root.replaceChildren(
    header({ user, rerender: ctx.rerender, navigate: ctx.navigate }),
    verifyBanner(user) ?? "",
    h(
      "main",
      { class: "wrap narrow" },
      h("h1", null, t("sitesTitle")),
      sites.length ? list : h("p", { class: "muted" }, t("noSites")),
      form,
      result,
    ),
    footer(),
  );
}

export function notFoundView(root: HTMLElement, user: User | null, ctx: Ctx) {
  root.replaceChildren(
    header({ user, rerender: ctx.rerender, navigate: ctx.navigate }),
    h(
      "main",
      { class: "wrap narrow" },
      h("h1", null, t("notFound")),
      user ? h("a", { href: "/" }, t("backToSites")) : h("a", { href: "/login" }, t("login")),
    ),
    footer(),
  );
}
