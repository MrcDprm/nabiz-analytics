// Metin sayfaları: Nasıl çalışır, Gizlilik ve KVKK, Kullanım koşulları.
import type { User } from "../api.ts";
import { h } from "../dom.ts";
import { lang, pub, t } from "../i18n.ts";
import { footer, header } from "../shell.ts";
import { pages, type Block, type PageKey } from "../content/pages.ts";

type Ctx = { rerender(): void; navigate(p: string): void };

// Metindeki adresleri tıklanabilir bağlantıya çevirir (textContent ile; HTML yok).
function linkify(text: string): Node[] {
  const out: Node[] = [];
  let last = 0;
  for (const m of text.matchAll(/https:\/\/[^\s)]+/g)) {
    out.push(document.createTextNode(text.slice(last, m.index)));
    const url = m[0].replace(/[.,]$/, "");
    out.push(h("a", { href: url, rel: "noopener" }, url));
    last = (m.index ?? 0) + url.length;
  }
  out.push(document.createTextNode(text.slice(last)));
  return out;
}

function block(b: Block): HTMLElement {
  if ("h2" in b) return h("h2", null, b.h2);
  if ("p" in b) return h("p", null, ...linkify(b.p));
  if ("ul" in b) return h("ul", null, ...b.ul.map((li) => h("li", null, li)));
  if ("table" in b) {
    return h(
      "table",
      { class: "kv" },
      h("thead", null, h("tr", null, h("th", { scope: "col" }, b.head[0]), h("th", { scope: "col" }, b.head[1]))),
      h("tbody", null, ...b.table.map(([a, c]) => h("tr", null, h("td", null, a), h("td", null, c)))),
    );
  }
  const btn = h("button", { type: "button", class: "text-btn copy" }, t("copy"));
  btn.addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText(b.copy);
      btn.textContent = t("copied");
    } catch {
      /* izin yoksa metin elle seçilebilir */
    }
  });
  return h("div", { class: "copy-block" }, h("div", { class: "copy-head" }, h("b", null, b.label), btn), h("pre", null, b.copy));
}

export function textPage(root: HTMLElement, key: PageKey, user: User | null, ctx: Ctx) {
  const page = pages[key][lang];
  document.title = `${page.title} | Nabız`;
  root.replaceChildren(
    header({ user, rerender: ctx.rerender, navigate: ctx.navigate, langHref: pub("/" + key, lang === "tr" ? "en" : "tr") }),
    h(
      "main",
      { class: "wrap narrow prose" },
      h("h1", null, page.title),
      h("p", { class: "lead" }, page.lead),
      ...page.blocks.map(block),
      h("p", { class: "muted small" }, page.updated),
    ),
    footer(),
  );
}
