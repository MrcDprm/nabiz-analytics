// Ayarlar sayfasındaki "Takım" bölümü: üyeler, roller, davetler, ayrılma ve sahiplik devri.
import { api, type Site, type User } from "../api.ts";
import { clear, h } from "../dom.ts";
import { t, tryT } from "../i18n.ts";
import { errText } from "../views/auth.ts";

type Member = { email: string; role: string };
type Invite = { email: string; role: string; expires_at: string };

export function teamSection(site: Site, user: User, onLeft: () => void): HTMLElement {
  const canManage = site.role === "owner" || site.role === "admin";
  const list = h("ul", { class: "goal-list team-list" });
  const out = h("p", { class: "small", role: "status", hidden: true });
  const base = `/api/sites/${site.id}`;
  const enc = encodeURIComponent;
  const roleName = (r: string) => tryT(`role.${r}`) ?? r;
  let members: Member[] = [];

  function say(text: string, ok: boolean) {
    out.hidden = false;
    out.className = ok ? "small ok" : "form-error";
    out.textContent = text;
  }

  async function act(fn: () => Promise<unknown>, okText?: string) {
    try {
      await fn();
      if (okText) say(okText, true);
      else out.hidden = true;
      load();
    } catch (e) {
      say(errText(e), false);
    }
  }

  async function load() {
    const data = await api<{ members: Member[]; invites: Invite[] }>(`${base}/members`);
    members = data.members;
    clear(list);
    for (const m of data.members) {
      const me = m.email === user.email;
      const cells: Node[] = [h("span", { class: "v" }, m.email, me ? h("span", { class: "muted" }, ` (${t("you")})`) : null)];
      if (canManage && m.role !== "owner" && !me) {
        const sel = h("select", { "aria-label": `${t("changeRole")}: ${m.email}` }, h("option", { value: "admin" }, roleName("admin")), h("option", { value: "viewer" }, roleName("viewer")));
        sel.value = m.role;
        sel.addEventListener("change", () => act(() => api(`${base}/members/${enc(m.email)}`, { method: "PATCH", body: { role: sel.value } }), t("saved")));
        cells.push(sel, h("button", { type: "button", class: "text-btn", "aria-label": `${t("removeMember")}: ${m.email}`, onclick: () => act(() => api(`${base}/members/${enc(m.email)}`, { method: "DELETE" })) }, "✕"));
      } else {
        cells.push(h("span", { class: "kind" }, roleName(m.role)));
        if (me && m.role !== "owner") {
          cells.push(h("button", {
            type: "button",
            class: "text-btn",
            onclick: async () => {
              try {
                await api(`${base}/members/${enc(m.email)}`, { method: "DELETE" });
                onLeft();
              } catch (e) {
                say(errText(e), false);
              }
            },
          }, t("leave")));
        }
      }
      list.append(h("li", null, ...cells));
    }
    for (const inv of data.invites) {
      list.append(
        h(
          "li",
          { class: "pending" },
          h("span", { class: "v" }, inv.email, h("span", { class: "muted" }, ` (${t("pending")})`)),
          h("span", { class: "kind" }, roleName(inv.role)),
          canManage ? h("button", { type: "button", class: "text-btn", "aria-label": `${t("revoke")}: ${inv.email}`, onclick: () => act(() => api(`${base}/invites/${enc(inv.email)}`, { method: "DELETE" })) }, "✕") : null,
        ),
      );
    }
    renderTransfer();
  }

  // Davet formu
  const email = h("input", { type: "email", required: true, placeholder: "ornek@example.com", "aria-label": t("email") });
  const role = h("select", { "aria-label": t("changeRole") }, h("option", { value: "viewer" }, roleName("viewer")), h("option", { value: "admin" }, roleName("admin")));
  const inviteForm = h("form", { class: "inline", hidden: !canManage }, email, role, h("button", { type: "submit", class: "secondary" }, t("invite")));
  inviteForm.addEventListener("submit", (e) => {
    e.preventDefault();
    act(() => api(`${base}/invites`, { method: "POST", body: { email: email.value, role: role.value } }), t("inviteSent")).then(() => (email.value = ""));
  });

  // Sahiplik devri (sadece sahip, başka üye varsa)
  const transferBox = h("div", { class: "transfer" });
  function renderTransfer() {
    clear(transferBox);
    const others = members.filter((m) => m.role !== "owner");
    if (site.role !== "owner" || !others.length) return;
    const who = h("select", { "aria-label": t("transfer") }, ...others.map((m) => h("option", { value: m.email }, m.email)));
    const pass = h("input", { type: "password", required: true, autocomplete: "current-password", placeholder: t("currentPassword"), "aria-label": t("currentPassword") });
    const form = h("form", { class: "stack" }, h("h3", null, t("transfer")), h("p", { class: "muted small" }, t("transferHelp")), who, pass, h("button", { type: "submit", class: "danger" }, t("transferBtn")));
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      try {
        await api(`${base}/transfer`, { method: "POST", body: { email: who.value, password: pass.value } });
        site.role = "admin";
        say(t("transferDone"), true);
        load();
      } catch (err) {
        say(errText(err), false);
      }
    });
    transferBox.append(form);
  }

  load().catch((e) => say(errText(e), false));
  return h("section", { class: "settings-block" }, h("h2", null, t("team")), h("p", { class: "muted small" }, t("teamHelp")), list, inviteForm, out, transferBox);
}
