// Kurulum satırı: siteye eklenecek <script> etiketi ve kopyala düğmesi.
import { h } from "../dom.ts";
import { t } from "../i18n.ts";

function copyButton(text: string) {
  const btn = h("button", { type: "button", class: "text-btn copy" }, t("copy"));
  btn.addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText(text);
      btn.textContent = t("copied");
      setTimeout(() => (btn.textContent = t("copy")), 2000);
    } catch {
      /* izin yoksa kullanıcı metni elle seçebilir */
    }
  });
  return btn;
}

export function snippet(domain: string): HTMLElement {
  const origin = location.origin;
  const code = `<script defer src="${origin}/js/nabiz.js" data-site="${domain}"></script>`;
  return h(
    "div",
    { class: "snippet" },
    h("p", null, t("snippetHelp")),
    h("div", { class: "code-row" }, h("code", null, code), copyButton(code)),
    h("p", { class: "muted" }, t("snippetCsp")),
    h("div", { class: "code-row" }, h("code", null, origin), copyButton(origin)),
  );
}
