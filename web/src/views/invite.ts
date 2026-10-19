// Davet bağlantısı (/invite?token=...): daveti gösterir; giriş yapılmışsa kabul ettirir,
// hesap yoksa şifre belirletip hesap açar, hesap varsa önce giriş ister.
import { api, ApiError, type User } from "../api.ts";
import { h } from "../dom.ts";
import { t, tryT } from "../i18n.ts";
import { footer, header } from "../shell.ts";
import { errText } from "./auth.ts";

type Ctx = { rerender(): void; navigate(p: string): void };
type Info = { domain: string; email: string; role: string; has_account: boolean };

// Token adresten silinir ama giriş sayfasına gidip dönerken kaybolmasın diye bu
// sekmede kısa süre saklanır (sessionStorage sekme kapanınca silinir).
const KEY = "nabiz-invite";

function token(): string {
  const fromUrl = new URLSearchParams(location.search).get("token");
  if (fromUrl) {
    try {
      sessionStorage.setItem(KEY, fromUrl);
    } catch {
      /* saklanamazsa giriş sonrası bağlantıyı tekrar açmak gerekir */
    }
    history.replaceState(null, "", location.pathname);
    return fromUrl;
  }
  try {
    return sessionStorage.getItem(KEY) ?? "";
  } catch {
    return "";
  }
}

export function pendingInvite(): boolean {
  try {
    return !!sessionStorage.getItem(KEY);
  } catch {
    return false;
  }
}

export async function inviteView(root: HTMLElement, user: User | null, ctx: Ctx) {
  const tok = token();
  const main = h("main", { class: "wrap narrow" }, h("h1", null, t("inviteTitle")));
  root.replaceChildren(header({ user, rerender: ctx.rerender, navigate: ctx.navigate }), main, footer());

  let info: Info;
  try {
    info = await api<Info>("/api/invites/info", { method: "POST", body: { token: tok } });
  } catch (e) {
    main.append(h("p", { class: "form-error" }, e instanceof ApiError && e.status === 400 ? t("badLink") : errText(e)));
    return;
  }
  const text = t("inviteText").replace("{domain}", info.domain).replace("{role}", (tryT(`role.${info.role}`) ?? info.role).toLocaleLowerCase());
  main.append(h("p", null, text), h("p", { class: "muted" }, t("inviteFor"), " ", h("b", null, info.email)));
  const error = h("p", { class: "form-error", role: "alert", hidden: true });

  async function accept(body: Record<string, string>) {
    error.hidden = true;
    try {
      const res = await api<{ domain: string }>("/api/invites/accept", { method: "POST", body: { token: tok, ...body } });
      try {
        sessionStorage.removeItem(KEY);
      } catch {
        /* önemli değil */
      }
      ctx.navigate(`/${res.domain}`);
    } catch (e) {
      error.hidden = false;
      error.textContent = errText(e);
    }
  }

  if (user && user.email !== info.email) {
    main.append(h("p", { class: "form-error" }, t("wrongAccount")));
    return;
  }
  if (user) {
    main.append(h("button", { type: "button", class: "primary", onclick: () => accept({}) }, t("accept")), error);
    return;
  }
  if (info.has_account) {
    main.append(h("p", null, t("loginToAccept")), h("a", { class: "primary link-btn", href: "/login" }, t("login")));
    return;
  }
  // Hesap yok: şifre belirle, hesap açılsın ve davet kabul edilsin.
  const who = h("input", { type: "email", autocomplete: "username", value: info.email, hidden: true, readonly: true, tabindex: -1 });
  const pass = h("input", { id: "password", type: "password", autocomplete: "new-password", required: true, minlength: 10 });
  const form = h("form", { class: "login" }, who, h("label", { for: "password" }, t("password")), pass, h("p", { class: "muted small" }, t("passwordRule")), error, h("button", { type: "submit", class: "primary" }, t("acceptCreate")));
  form.addEventListener("submit", (e) => {
    e.preventDefault();
    accept({ password: pass.value });
  });
  main.append(form);
  pass.focus();
}
