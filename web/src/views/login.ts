import { api, ApiError, type User } from "../api.ts";
import { h } from "../dom.ts";
import { t } from "../i18n.ts";
import { footer, header } from "../shell.ts";

export function loginView(root: HTMLElement, ctx: { rerender(): void; navigate(p: string): void }) {
  const email = h("input", { id: "email", type: "email", name: "email", autocomplete: "username", required: true });
  const password = h("input", { id: "password", type: "password", name: "password", autocomplete: "current-password", required: true });
  const error = h("p", { class: "form-error", role: "alert", hidden: true });
  const submit = h("button", { type: "submit", class: "primary" }, t("login"));

  const form = h(
    "form",
    { class: "login", novalidate: false },
    h("h1", null, t("loginTitle")),
    h("label", { for: "email" }, t("email")),
    email,
    h("label", { for: "password" }, t("password")),
    password,
    error,
    submit,
  );

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    error.hidden = true;
    submit.disabled = true;
    submit.textContent = t("loginBusy");
    try {
      await api<User>("/api/login", { method: "POST", body: { email: email.value, password: password.value } });
      ctx.navigate("/");
    } catch (err) {
      error.hidden = false;
      if (err instanceof ApiError && err.status === 401) error.textContent = t("loginWrong");
      else if (err instanceof ApiError && err.status === 429) error.textContent = t("loginLimited");
      else if (err instanceof ApiError && err.status === 0) error.textContent = t("errorOffline");
      else error.textContent = t("errorGeneric");
      password.select();
    } finally {
      submit.disabled = false;
      submit.textContent = t("login");
    }
  });

  root.replaceChildren(
    header({ user: null, hideAccount: true, rerender: ctx.rerender, navigate: ctx.navigate }),
    h("main", { class: "wrap narrow" }, form),
    footer(),
  );
  email.focus();
}
