// Tanıtım sayfası (/, giriş yapmamış ziyaretçiye): en üstte gerçek bir sitenin
// canlı nabız şeridi; altında ne toplandığı, kurulum satırı ve kendi sunucunda kurma.
import { api, type Live } from "../api.ts";
import { h } from "../dom.ts";
import { locale, t } from "../i18n.ts";
import { num } from "../format.ts";
import { config, footer, header } from "../shell.ts";
import { createStrip } from "../components/strip.ts";

type Ctx = { rerender(): void; navigate(p: string): void; onLeave(fn: () => void): void };

const REPO = "https://github.com/MrcDprm/nabiz-analytics";

export function landingView(root: HTMLElement, ctx: Ctx) {
  const cfg = config();
  document.title = `Nabız | ${t("landingTitle")}`;

  // --- Canlı şerit: tanıtım sitesinin son 5 dakikası ---
  const strip = createStrip();
  const liveText = h("p", { class: "landing-live", "aria-live": "polite" });
  if (cfg.demo) {
    const demoLink = h("a", { href: `/${cfg.demo}` }, t("openLiveDemo"));
    const load = async () => {
      try {
        const live = await api<Live>(`/api/stats/${encodeURIComponent(cfg.demo)}/live`);
        strip.setBeats(live.beats);
        liveText.replaceChildren(
          h("b", { class: "landing-n" }, num(live.visitors, locale())),
          " ",
          t("liveOnDemo").replace("{site}", cfg.demo),
          " ",
          demoLink,
        );
      } catch {
        liveText.replaceChildren(demoLink);
      }
    };
    load();
    const timer = setInterval(load, 15_000);
    ctx.onLeave(() => {
      clearInterval(timer);
      strip.destroy();
    });
  } else {
    strip.setBeats([]);
    ctx.onLeave(() => strip.destroy());
  }

  const actions = h(
    "div",
    { class: "landing-actions" },
    cfg.signup ? h("a", { class: "primary link-btn", href: "/signup" }, t("signupFree")) : null,
    h("a", { class: "secondary link-btn", href: "/how-it-works" }, t("howItWorks")),
  );

  const hero = h(
    "section",
    { class: "landing-hero" },
    h("div", { class: "strip landing-strip" }, strip.el),
    h(
      "div",
      { class: "wrap landing-copy" },
      h("h1", null, t("landingTitle")),
      h("p", { class: "lead" }, t("landingLead")),
      liveText,
      actions,
    ),
  );

  const collects = h(
    "section",
    { class: "wrap landing-cols" },
    h("div", null, h("h2", null, t("weCount")), h("ul", null, ...["countPages", "countSources", "countPlaces", "countEvents"].map((k) => h("li", null, t(k as "countPages"))))),
    h("div", null, h("h2", null, t("weDont")), h("ul", { class: "nots" }, ...["noCookies", "noIp", "noProfile", "noThirdParty"].map((k) => h("li", null, t(k as "noCookies"))))),
  );

  const origin = location.origin;
  const install = h(
    "section",
    { class: "wrap landing-install" },
    h("h2", null, t("installTitle")),
    h("p", null, t("installLead")),
    h("pre", { class: "install-line" }, `<script defer src="${origin}/js/nabiz.js" data-site="example.com"></script>`),
  );

  const selfhost = h(
    "section",
    { class: "wrap landing-self" },
    h("h2", null, t("selfTitle")),
    h("p", null, t("selfLead")),
    h("a", { href: REPO, rel: "noopener" }, t("selfLink")),
  );

  root.replaceChildren(
    header({ user: null, rerender: ctx.rerender, navigate: ctx.navigate }),
    h("main", null, hero, collects, install, selfhost),
    footer(),
  );
}
