// Hesap ayarları: şifre, e-posta, hesabı silme.
import { api, type User } from "../api.ts";
import { h } from "../dom.ts";
import { t } from "../i18n.ts";
import { footer, header } from "../shell.ts";
import { errText } from "./auth.ts";

type Ctx = { rerender(): void; navigate(p: string): void };

function result() {
  return h("p", { class: "small", role: "status", hidden: true });
}

function show(el: HTMLElement, text: string, ok: boolean) {
  el.hidden = false;
  el.className = ok ? "small ok" : "form-error";
  el.textContent = text;
}

export function accountView(root: HTMLElement, user: User, ctx: Ctx) {
  // Şifre yöneticileri hangi hesabın şifresi olduğunu bilsin diye her formda gizli kullanıcı adı.
  const who = () => h("input", { type: "email", autocomplete: "username", value: user.email, hidden: true, readonly: true, tabindex: -1 });
  // Şifre değiştir
  const cur = h("input", { type: "password", autocomplete: "current-password", required: true, "aria-label": t("currentPassword"), placeholder: t("currentPassword") });
  const next = h("input", { type: "password", autocomplete: "new-password", required: true, minlength: 10, "aria-label": t("newPassword"), placeholder: t("newPassword") });
  const pwOut = result();
  const pwForm = h("form", { class: "stack" }, who(), cur, next, h("p", { class: "muted small" }, t("passwordRule")), h("button", { type: "submit", class: "secondary" }, t("changePassword")), pwOut);
  pwForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await api("/api/account/password", { method: "POST", body: { current: cur.value, new: next.value } });
      pwForm.reset();
      show(pwOut, t("passwordChanged"), true);
    } catch (err) {
      show(pwOut, errText(err), false);
    }
  });

  // E-posta değiştir
  const newEmail = h("input", { type: "email", autocomplete: "email", required: true, "aria-label": t("newEmail"), placeholder: t("newEmail") });
  const emPass = h("input", { type: "password", autocomplete: "current-password", required: true, "aria-label": t("currentPassword"), placeholder: t("currentPassword") });
  const emOut = result();
  const emForm = h("form", { class: "stack" }, newEmail, emPass, h("button", { type: "submit", class: "secondary" }, t("changeEmail")), emOut);
  emForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await api("/api/account/email", { method: "POST", body: { email: newEmail.value, password: emPass.value } });
      emForm.reset();
      show(emOut, t("emailChangeSent"), true);
    } catch (err) {
      show(emOut, errText(err), false);
    }
  });

  // Hesabı sil
  const delPass = h("input", { type: "password", autocomplete: "current-password", required: true, "aria-label": t("currentPassword"), placeholder: t("currentPassword") });
  const delOut = result();
  const delForm = h("form", { class: "stack" }, who(), h("p", null, t("deleteAccountHelp")), delPass, h("button", { type: "submit", class: "danger" }, t("deleteAccountBtn")), delOut);
  delForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await api("/api/account", { method: "DELETE", body: { password: delPass.value } });
      ctx.navigate("/login");
    } catch (err) {
      show(delOut, errText(err), false);
    }
  });

  root.replaceChildren(
    header({ user, rerender: ctx.rerender, navigate: ctx.navigate }),
    h(
      "main",
      { class: "wrap narrow" },
      h("h1", null, t("accountTitle")),
      h("p", { class: "muted" }, t("currentEmail"), " ", h("b", null, user.email)),
      h("section", { class: "settings-block" }, h("h2", null, t("changePassword")), pwForm),
      h("section", { class: "settings-block" }, h("h2", null, t("changeEmail")), emForm),
      h("section", { class: "settings-block" }, h("h2", null, t("deleteAccount")), delForm),
    ),
    footer(),
  );
}

// verifyBanner, e-postası doğrulanmamış kullanıcıya sayfanın üstünde hatırlatma gösterir.
export function verifyBanner(user: User | null): HTMLElement | null {
  if (!user || user.verified) return null;
  const btn = h("button", { type: "button", class: "text-btn" }, t("resend"));
  btn.addEventListener("click", async () => {
    btn.disabled = true;
    try {
      await api("/api/verify/resend", { method: "POST" });
      btn.textContent = t("resent");
    } catch (e) {
      btn.textContent = errText(e);
    }
  });
  return h("div", { class: "banner", role: "status" }, h("div", { class: "wrap" }, h("span", null, t("verifyBanner")), btn));
}
