// Giriş dışındaki hesap sayfaları: kayıt, e-posta doğrulama, şifremi unuttum, şifre sıfırlama.
import { api, ApiError } from "../api.ts";
import { h } from "../dom.ts";
import { lang, t } from "../i18n.ts";
import { footer, header } from "../shell.ts";
import { serverMessage } from "../errors.ts";

type Ctx = { rerender(): void; navigate(p: string): void };

export function errText(e: unknown): string {
  if (e instanceof ApiError && (e.status === 400 || e.status === 403)) {
    const m = serverMessage(e.message, lang);
    if (m) return m;
  }
  if (e instanceof ApiError && e.status === 429) return t("loginLimited");
  if (e instanceof ApiError && e.status === 0) return t("errorOffline");
  return t("errorGeneric");
}

function page(root: HTMLElement, ctx: Ctx, ...content: Node[]) {
  root.replaceChildren(
    header({ user: null, hideAccount: true, rerender: ctx.rerender, navigate: ctx.navigate }),
    h("main", { class: "wrap narrow" }, ...content),
    footer(),
  );
}

// Adresteki token'ı okur ve adresten siler: sayfa paylaşılırsa ya da tarayıcı
// geçmişinde kalırsa bağlantı başkasının eline geçmesin.
function takeToken(): string {
  const token = new URLSearchParams(location.search).get("token") ?? "";
  history.replaceState(null, "", location.pathname);
  return token;
}

function field(id: string, label: string, attrs: Record<string, string | boolean>) {
  const input = h("input", { id, name: id, required: true, ...attrs });
  return { input, nodes: [h("label", { for: id }, label), input] };
}

function submitting(btn: HTMLButtonElement, busy: string) {
  const idle = btn.textContent ?? "";
  btn.disabled = true;
  btn.textContent = busy;
  return () => {
    btn.disabled = false;
    btn.textContent = idle;
  };
}

export function signupView(root: HTMLElement, ctx: Ctx) {
  const email = field("email", t("email"), { type: "email", autocomplete: "email" });
  const password = field("password", t("password"), { type: "password", autocomplete: "new-password", minlength: "10" });
  const error = h("p", { class: "form-error", role: "alert", hidden: true });
  const submit = h("button", { type: "submit", class: "primary" }, t("signup"));
  const form = h(
    "form",
    { class: "login" },
    h("h1", null, t("signupTitle")),
    ...email.nodes,
    ...password.nodes,
    h("p", { class: "muted small" }, t("passwordRule")),
    error,
    submit,
    h("p", { class: "small alt" }, t("haveAccount"), " ", h("a", { href: "/login" }, t("login"))),
  );
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    error.hidden = true;
    const done = submitting(submit, t("signupBusy"));
    try {
      await api("/api/signup", { method: "POST", body: { email: email.input.value, password: password.input.value, locale: lang } });
      page(root, ctx, h("h1", null, t("checkInbox")), h("p", null, t("signupSent")));
    } catch (err) {
      error.hidden = false;
      error.textContent = errText(err);
      done();
    }
  });
  page(root, ctx, form);
  email.input.focus();
}

export async function verifyView(root: HTMLElement, ctx: Ctx) {
  const token = takeToken();
  const status = h("p", { role: "status" }, t("verifying"));
  page(root, ctx, h("h1", null, t("account")), status);
  try {
    await api("/api/verify", { method: "POST", body: { token } });
    status.textContent = t("verified");
    status.after(h("a", { class: "primary link-btn", href: "/" }, t("continue")));
  } catch (e) {
    status.className = "form-error";
    status.textContent = errText(e);
  }
}

export function forgotView(root: HTMLElement, ctx: Ctx) {
  const email = field("email", t("email"), { type: "email", autocomplete: "email" });
  const error = h("p", { class: "form-error", role: "alert", hidden: true });
  const submit = h("button", { type: "submit", class: "primary" }, t("sendLink"));
  const form = h("form", { class: "login" }, h("h1", null, t("forgotTitle")), h("p", { class: "muted" }, t("forgotHelp")), ...email.nodes, error, submit);
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    error.hidden = true;
    const done = submitting(submit, t("sending"));
    try {
      await api("/api/password/forgot", { method: "POST", body: { email: email.input.value } });
      page(root, ctx, h("h1", null, t("checkInbox")), h("p", null, t("forgotSent")), h("a", { href: "/login" }, t("login")));
    } catch (err) {
      error.hidden = false;
      error.textContent = errText(err);
      done();
    }
  });
  page(root, ctx, form);
  email.input.focus();
}

export function resetView(root: HTMLElement, ctx: Ctx) {
  const token = takeToken();
  const password = field("password", t("newPassword"), { type: "password", autocomplete: "new-password", minlength: "10" });
  const error = h("p", { class: "form-error", role: "alert", hidden: true });
  const submit = h("button", { type: "submit", class: "primary" }, t("savePassword"));
  const form = h("form", { class: "login" }, h("h1", null, t("resetTitle")), ...password.nodes, h("p", { class: "muted small" }, t("passwordRule")), error, submit);
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    error.hidden = true;
    const done = submitting(submit, t("sending"));
    try {
      await api("/api/password/reset", { method: "POST", body: { token, password: password.input.value } });
      page(root, ctx, h("h1", null, t("resetTitle")), h("p", null, t("resetDone")), h("a", { class: "primary link-btn", href: "/login" }, t("login")));
    } catch (err) {
      error.hidden = false;
      error.textContent = errText(err);
      if (err instanceof ApiError && err.status === 400 && /bağlantı|link/i.test(err.message)) {
        error.append(" ", h("a", { href: "/forgot" }, t("forgot")));
      }
      done();
    }
  });
  page(root, ctx, form);
  password.input.focus();
}
