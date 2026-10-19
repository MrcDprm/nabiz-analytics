// Site ayarları: herkese açık panel, kurulum satırı, hedefler ve siteyi silme.
import { api, ApiError, type Goal, type Site, type User } from "../api.ts";
import { clear, h } from "../dom.ts";
import { t } from "../i18n.ts";
import { footer, header } from "../shell.ts";
import { snippet } from "../components/snippet.ts";

type Ctx = { rerender(): void; navigate(p: string): void };

function errText(e: unknown): string {
  if (e instanceof ApiError && e.status === 400 && e.message) return e.message;
  if (e instanceof ApiError && e.status === 0) return t("errorOffline");
  return t("errorGeneric");
}

export async function settingsView(root: HTMLElement, site: Site, user: User, ctx: Ctx) {
  const canEdit = site.role === "owner" || site.role === "admin";

  // --- Görünürlük ---
  const publicBox = h("input", { type: "checkbox", id: "public", checked: site.public, disabled: !canEdit });
  const publicStatus = h("span", { class: "muted small", "aria-live": "polite" });
  const publicLink = h("p", { class: "small", hidden: !site.public }, t("publicLink"), " ", h("a", { href: `/${site.domain}` }, `${location.origin}/${site.domain}`));
  publicBox.addEventListener("change", async () => {
    publicBox.disabled = true;
    try {
      const updated = await api<Site>(`/api/sites/${site.id}`, { method: "PATCH", body: { public: publicBox.checked } });
      site.public = updated.public;
      publicLink.hidden = !site.public;
      publicStatus.textContent = t("saved");
    } catch (e) {
      publicBox.checked = site.public;
      publicStatus.textContent = errText(e);
    } finally {
      publicBox.disabled = false;
    }
  });

  // --- Hedefler ---
  const goalList = h("ul", { class: "goal-list" });
  const goalError = h("p", { class: "form-error", role: "alert", hidden: true });
  async function loadGoals() {
    const { goals } = await api<{ goals: Goal[] }>(`/api/sites/${site.id}/goals`);
    clear(goalList);
    if (!goals.length) goalList.append(h("li", { class: "muted" }, t("noGoals")));
    for (const g of goals) {
      goalList.append(
        h(
          "li",
          null,
          h("span", { class: "kind" }, g.kind === "page" ? t("goalKindPage") : t("goalKindEvent")),
          h("span", { class: "v" }, g.value),
          canEdit
            ? h(
                "button",
                {
                  type: "button",
                  class: "text-btn",
                  "aria-label": `${t("removeGoal")}: ${g.value}`,
                  onclick: async () => {
                    try {
                      await api(`/api/sites/${site.id}/goals/${g.id}`, { method: "DELETE" });
                      loadGoals();
                    } catch (e) {
                      goalError.hidden = false;
                      goalError.textContent = errText(e);
                    }
                  },
                },
                "✕",
              )
            : null,
        ),
      );
    }
  }
  const kind = h("select", { "aria-label": t("goals") }, h("option", { value: "page" }, t("goalKindPage")), h("option", { value: "event" }, t("goalKindEvent")));
  const value = h("input", { placeholder: "/iletisim", "aria-label": t("goalValue"), required: true });
  kind.addEventListener("change", () => (value.placeholder = kind.value === "page" ? "/iletisim" : "CV indir"));
  const goalForm = h("form", { class: "inline goal-form", hidden: !canEdit }, kind, value, h("button", { type: "submit", class: "text-btn" }, t("addGoal")));
  goalForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    goalError.hidden = true;
    try {
      await api(`/api/sites/${site.id}/goals`, { method: "POST", body: { kind: kind.value, value: value.value } });
      value.value = "";
      loadGoals();
    } catch (err) {
      goalError.hidden = false;
      goalError.textContent = errText(err);
    }
  });

  // --- Silme (sadece sahip) ---
  let danger: HTMLElement | null = null;
  if (site.role === "owner") {
    const confirmIn = h("input", { "aria-label": t("domain"), placeholder: site.domain, autocomplete: "off" });
    const del = h("button", { type: "submit", class: "danger", disabled: true }, t("deleteSite"));
    const delError = h("p", { class: "form-error", role: "alert", hidden: true });
    confirmIn.addEventListener("input", () => (del.disabled = confirmIn.value.trim().toLowerCase() !== site.domain));
    const form = h("form", null, h("p", null, t("deleteHelp")), h("div", { class: "inline" }, confirmIn, del), delError);
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      try {
        await api(`/api/sites/${site.id}`, { method: "DELETE" });
        ctx.navigate("/");
      } catch (err) {
        delError.hidden = false;
        delError.textContent = errText(err);
      }
    });
    danger = h("section", { class: "settings-block" }, h("h2", null, t("dangerZone")), form);
  }

  root.replaceChildren(
    header({ user, middle: [h("a", { class: "site-name", href: `/${site.domain}` }, site.domain)], rerender: ctx.rerender, navigate: ctx.navigate }),
    h(
      "main",
      { class: "wrap narrow" },
      h("h1", null, t("settingsTitle")),
      h("section", { class: "settings-block" }, h("h2", null, t("snippetTitle")), snippet(site.domain)),
      h(
        "section",
        { class: "settings-block" },
        h("h2", null, t("visibility")),
        h("label", { class: "check" }, publicBox, t("publicLabel")),
        h("p", { class: "muted small" }, t("publicHelp")),
        publicLink,
        publicStatus,
      ),
      h("section", { class: "settings-block" }, h("h2", null, t("goals")), h("p", { class: "muted small" }, t("goalsHelp")), goalList, goalForm, goalError),
      danger,
    ),
    footer(),
  );
  loadGoals().catch((e) => {
    goalError.hidden = false;
    goalError.textContent = errText(e);
  });
}
