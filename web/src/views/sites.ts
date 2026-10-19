import type { Site, User } from "../api.ts";
import { h } from "../dom.ts";
import { t } from "../i18n.ts";
import { footer, header } from "../shell.ts";

export function sitesView(root: HTMLElement, user: User, sites: Site[], ctx: { rerender(): void; navigate(p: string): void }) {
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
  root.replaceChildren(
    header({ user, rerender: ctx.rerender, navigate: ctx.navigate }),
    h(
      "main",
      { class: "wrap narrow" },
      h("h1", null, t("sitesTitle")),
      sites.length ? list : h("p", { class: "muted" }, t("noSites")),
    ),
    footer(),
  );
}

export function notFoundView(root: HTMLElement, user: User | null, ctx: { rerender(): void; navigate(p: string): void }) {
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
