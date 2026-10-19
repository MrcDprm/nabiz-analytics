// Panelin girişi: adres → görünüm. Sayfa yenilenmeden gezinmek için history API.
import "@fontsource-variable/atkinson-hyperlegible-next";
import "./styles.css";
import { api, ApiError, type Site, type User } from "./api.ts";
import { h } from "./dom.ts";
import { t } from "./i18n.ts";
import { applyTheme, setCloud } from "./shell.ts";
import { billingView } from "./views/billing.ts";
import { dashboardView } from "./views/dashboard.ts";
import { loginView } from "./views/login.ts";
import { notFoundView, sitesView } from "./views/sites.ts";
import { settingsView } from "./views/settings.ts";
import { forgotView, resetView, signupView, verifyView } from "./views/auth.ts";
import { accountView } from "./views/account.ts";
import { inviteView, pendingInvite } from "./views/invite.ts";

const root = document.getElementById("app")!;
let leave: (() => void)[] = [];

function navigate(path: string) {
  history.pushState(null, "", path);
  render();
}

async function currentUser(): Promise<User | null> {
  try {
    return await api<User>("/api/me");
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return null;
    throw e;
  }
}

async function render() {
  for (const fn of leave) fn();
  leave = [];
  const ctx = { rerender: render, navigate, onLeave: (fn: () => void) => leave.push(fn) };
  const path = decodeURIComponent(location.pathname).replace(/\/+$/, "") || "/";

  try {
    if (path === "/login") return loginView(root, ctx);
    if (path === "/signup") return signupView(root, ctx);
    if (path === "/verify") return verifyView(root, ctx);
    if (path === "/forgot") return forgotView(root, ctx);
    if (path === "/reset") return resetView(root, ctx);
    const user = await currentUser();
    const sites = user ? (await api<{ sites: Site[] }>("/api/sites")).sites : [];

    if (path === "/invite") return inviteView(root, user, ctx);
    if (path === "/billing") {
      if (!user) return navigate("/login");
      return billingView(root, user, ctx);
    }
    if (path === "/account") {
      if (!user) return navigate("/login");
      return accountView(root, user, ctx);
    }
    if (path === "/") {
      if (!user) return navigate("/login");
      // Davet bağlantısından giriş yapmaya gelindiyse davete dön.
      if (pendingInvite()) return navigate("/invite");
      return sitesView(root, user, sites, ctx);
    }
    if (path.startsWith("/settings/")) {
      const site = sites.find((st) => st.domain === path.slice("/settings/".length).toLowerCase());
      if (!user) return navigate("/login");
      if (!site) return notFoundView(root, user, ctx);
      document.title = `${site.domain} | Nabız`;
      return settingsView(root, site, user, ctx);
    }
    const domain = path.slice(1).toLowerCase();
    if (!/^[a-z0-9.-]+$/.test(domain)) return notFoundView(root, user, ctx);
    // Site erişilebilir mi? Erişilemiyorsa (yok ya da izin yok) sunucu 404 der.
    try {
      await api(`/api/stats/${encodeURIComponent(domain)}/live`);
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) return notFoundView(root, user, ctx);
      throw e;
    }
    document.title = `${domain} | Nabız`;
    dashboardView(root, domain, user, sites, ctx);
  } catch (e) {
    root.replaceChildren(
      h("main", { class: "wrap narrow" }, h("p", { class: "form-error" }, e instanceof ApiError && e.status === 0 ? t("errorOffline") : t("errorGeneric"))),
    );
  }
}

// Panel içindeki bağlantılar sayfayı yenilemeden açılır.
document.addEventListener("click", (e) => {
  const a = (e.target as Element).closest?.("a");
  if (!a || a.target || a.origin !== location.origin || e.ctrlKey || e.metaKey || e.shiftKey || e.button !== 0) return;
  e.preventDefault();
  navigate(a.pathname + a.search);
});
addEventListener("popstate", render);

applyTheme();
api<{ mode: string }>("/api/config")
  .then((c) => setCloud(c.mode === "cloud"))
  .catch(() => {})
  .finally(render);
