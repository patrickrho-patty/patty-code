// Patty Code Community client. Renders the forum from the forum.patty.io API and
// gates posting on the shared id.patty.io session (cookie sent cross-subdomain).
import { initTheme } from "./theme.js";

const FORUM = (import.meta.env.PUBLIC_FORUM_API || "https://forum.patty.io").replace(/\/$/, "");
const ACCOUNTS = (import.meta.env.PUBLIC_ACCOUNTS_API || "https://id.patty.io").replace(/\/$/, "");

const el = (id) => document.getElementById(id);
const qp = new URLSearchParams(location.search);

async function api(base, path, opts = {}) {
  const res = await fetch(base + path, {
    method: opts.method || "GET",
    credentials: "include",
    headers: opts.body ? { "content-type": "application/json" } : undefined,
    body: opts.body ? JSON.stringify(opts.body) : undefined,
  });
  let data = null;
  try { data = await res.json(); } catch {}
  if (!res.ok) {
    const err = new Error(data?.error?.message || "Something went wrong.");
    err.code = data?.error?.code;
    err.status = res.status;
    throw err;
  }
  return data;
}
const forum = (p, o) => api(FORUM, p, o);

// Anti-spam  gate errors are localized by code; unknown codes fall back to the
// server message.
const ERR = {
  email_unverified: "Confirm your email address before posting.",
  links_restricted: "New members can't post links yet — participate a little to unlock.",
  insufficient_trust: "You don't have access to post in this category yet.",
  silenced: "Your account is temporarily restricted from posting.",
  rate_limited: "You're posting too fast — take a short break.",
  daily_limit: "You've hit today's posting limit for your trust level.",
  closed: "This topic is closed to new replies.",
  self_flag: "You can't report your own post.",
  unauthorized: "Sign in to continue.",
};
const errText = (err) => (ERR[err.code] || err.message);

const CATS = {
  announcements: "Announcements",
  help: "Help & Support",
  skills: "Skills & Plugins",
  show: "Show & Tell",
  feedback: "Feedback & Ideas",
};
const CATDESC = {
  announcements: "Releases, roadmap, and community news.",
  help: "Stuck on setup, config, or cache behavior? Ask here.",
  skills: "Share, request, and review community skills and MCP servers.",
  show: "Built something with Patty Code? Show the community.",
  feedback: "Feature requests and product feedback.",
};
const catName = (slug, apiName) => (CATS[slug] || esc(apiName));
const catText = (slug, apiName) => (CATS[slug] || apiName);
const ROLES = { admin: "admin", moderator: "moderator" };

function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}
const AV_GRAD = [
  "var(--accent),var(--violet)",
  "var(--ok),oklch(0.62 0.15 200)",
  "var(--warm),var(--rose)",
  "var(--violet),var(--rose)",
  "oklch(0.6 0.15 200),var(--accent)",
];
function avatar(handle, size = "") {
  const h = handle || "?";
  let n = 0;
  for (const ch of h) n = (n + ch.charCodeAt(0)) % AV_GRAD.length;
  const initials = h.replace(/[^a-zA-Z0-9]/g, "").slice(0, 2).toUpperCase() || "?";
  return `<span class="av ${size}" style="background:linear-gradient(140deg,${AV_GRAD[n]})">${esc(initials)}</span>`;
}
function ago(iso) {
  if (!iso) return "";
  const s = Math.max(1, (Date.now() - new Date(iso).getTime()) / 1000);
  const u = [[86400, "d"], [3600, "h"], [60, "m"]];
  for (const [sec, unit] of u) if (s >= sec) { const v = Math.floor(s / sec); return `${v}${unit} ago`; }
  return "just now";
}
function md(body) {
  const parts = esc(body).split(/```/);
  let out = "";
  parts.forEach((chunk, i) => {
    if (i % 2 === 1) { out += `<pre>${chunk.replace(/^\n/, "")}</pre>`; return; }
    const paras = chunk.split(/\n{2,}/).map((p) => p.trim()).filter(Boolean);
    out += paras.map((p) => `<p>${p.replace(/`([^`]+)`/g, "<code>$1</code>").replace(/\n/g, "<br>")}</p>`).join("");
  });
  return out || "<p></p>";
}

const CAT_ICONS = { announcements: "📣", help: "🛟", skills: "🧩", show: "✨", feedback: "💡" };
const loginUrl = () => `/login/?next=${encodeURIComponent(location.pathname + location.search)}`;

let account = null;
async function loadAccount() {
  try { account = (await api(ACCOUNTS, "/me")).user; } catch { account = null; }
  const slot = el("nav-account");
  if (slot) {
    slot.innerHTML = account
      ? `<a href="/account/" title="${esc(account.email)}">${avatar(account.handle)}</a>`
      : `<a class="btn btn-ghost sm" href="${loginUrl()}">Sign in</a>`;
  }
}

/* ── home ─────────────────────────────────────────── */
function renderHome() {
  const catBox = el("cat-list");
  const topicBox = el("topic-list");
  const category = qp.get("category") || "";

  forum("/categories").then((d) => {
    const cats = d.categories;
    if (el("s-cats")) el("s-cats").textContent = cats.length;
    if (el("s-topics")) el("s-topics").textContent = cats.reduce((a, c) => a + (c.topicCount || 0), 0);
    catBox.innerHTML = cats.map((c) => `
      <a class="cat" href="/community/?category=${esc(c.slug)}">
        <span class="ico">${CAT_ICONS[c.slug] || "💬"}</span>
        <div><h3>${catName(c.slug, c.name)}</h3><p>${CATDESC[c.slug] || esc(c.description)}</p>
        <div class="meta">${c.topicCount || 0} topics${c.lastActivity ? " · " + ago(c.lastActivity) : ""}</div></div>
      </a>`).join("");
  }).catch(() => { catBox.innerHTML = `<div class="empty">Couldn't load categories.</div>`; });

  const loadTopics = (sort) => {
    topicBox.innerHTML = `<div class="skeleton"><div class="bar"></div><div class="bar short"></div></div>`.repeat(3);
    const q = new URLSearchParams();
    if (category) q.set("category", category);
    if (sort) q.set("sort", sort);
    forum("/topics?" + q).then((d) => {
      if (!d.topics.length) { topicBox.innerHTML = `<div class="empty">No topics yet — <a class="tag" href="/community/new/">start the first one</a>.</div>`; return; }
      topicBox.innerHTML = d.topics.map((tp) => `
        <div class="topic">
          ${avatar(tp.author)}
          <div class="main">
            <div class="title">
              ${tp.pinned ? `<span class="badge pinned">📌 Pinned</span>` : ""}
              ${tp.status === "solved" ? `<span class="badge solved">✓ Solved</span>` : ""}
              <a href="/community/topic/?id=${tp.id}">${esc(tp.title)}</a>
            </div>
            <div class="sub"><span class="cat-tag">${catName(tp.category, tp.categoryName)}</span> <span class="who">${esc(tp.author)}</span> · ${ago(tp.createdAt)}</div>
          </div>
          <div class="stat"><div class="n">${tp.replyCount}</div><div class="l">replies</div></div>
          <div class="last">${ago(tp.lastPostAt)}</div>
        </div>`).join("");
    }).catch(() => { topicBox.innerHTML = `<div class="empty">Couldn't load discussions.</div>`; });
  };
  loadTopics("latest");

  el("sort-tabs")?.addEventListener("click", (e) => {
    const b = e.target.closest("button[data-sort]");
    if (!b) return;
    el("sort-tabs").querySelectorAll("button").forEach((x) => x.classList.toggle("on", x === b));
    loadTopics(b.dataset.sort);
  });
}

/* ── thread ───────────────────────────────────────── */
let firstPostId = 0;
function postHtml(p, topic) {
  const answer = topic.acceptedPostId && topic.acceptedPostId === p.id;
  const cls = answer ? "post answer" : p.id === firstPostId ? "post op" : "post";
  const roleWord = ROLES[p.role] || "";
  const role = roleWord ? `<span class="badge role ${esc(p.role)}">${roleWord}</span>` : "";
  return `<article class="${cls}">
    ${avatar(p.handle || p.author, "lg")}
    <div>
      ${answer ? `<div class="answer-flag">✓ Accepted answer</div>` : ""}
      <div class="who"><span class="name">${esc(p.handle || p.author)}</span>${role}<span class="when">${ago(p.createdAt)}</span></div>
      <div class="body">${md(p.body)}</div>
      <div class="actions">
        <button class="react${p.liked ? " on" : ""}" data-like="${p.id}" data-liked="${p.liked ? "1" : "0"}" aria-pressed="${p.liked ? "true" : "false"}">👍 <span>${p.likeCount || 0}</span></button>
        <button class="link-act" data-flag="${p.id}">Report</button>
      </div>
    </div>
  </article>`;
}

async function renderThread() {
  const id = Number(qp.get("id"));
  if (!id) { location.href = "/community/"; return; }
  let data;
  try { data = await forum(`/topics/${id}`); }
  catch { el("posts").innerHTML = `<div class="empty">That discussion doesn't exist or was removed.</div>`; return; }
  const { topic, posts } = data;
  firstPostId = posts[0]?.id || 0;

  document.title = `${topic.title} — Patty Code Community`;
  el("crumb-cat").textContent = catText(topic.category, topic.category);
  el("crumb-title").textContent = topic.title;
  el("t-title").textContent = topic.title;
  el("t-meta").innerHTML =
    `${topic.status === "solved" ? `<span class="badge solved">✓ Solved</span>` : ""}
     <span>${topic.replyCount} replies · ${topic.viewCount} views · started ${ago(topic.createdAt)}</span>`;
  el("posts").innerHTML = posts.map((p) => postHtml(p, topic)).join("");

  const seen = new Set();
  el("parti").innerHTML = posts.filter((p) => !seen.has(p.author) && seen.add(p.author)).slice(0, 8).map((p) => avatar(p.handle || p.author)).join("");

  el("posts").addEventListener("click", async (e) => {
    const like = e.target.closest("[data-like]");
    if (like && account) {
      const wasLiked = like.dataset.liked === "1";
      like.disabled = true;
      try {
        const out = await forum(`/posts/${like.dataset.like}/likes`, { method: wasLiked ? "DELETE" : "POST" });
        like.dataset.liked = out.liked ? "1" : "0";
        like.setAttribute("aria-pressed", out.liked ? "true" : "false");
        like.classList.toggle("on", out.liked);
        like.querySelector("span").textContent = out.likeCount;
      } catch (err) { alert(errText(err)); }
      finally { like.disabled = false; }
      return;
    } else if (like) {
      location.href = loginUrl();
      return;
    }
    const flag = e.target.closest("[data-flag]");
    if (flag && account) {
      if (!confirm("Report this post as spam or abuse?")) return;
      try { await forum(`/posts/${flag.dataset.flag}/flags`, { method: "POST", body: { reason: "spam" } }); flag.innerHTML = "Reported ✓"; flag.disabled = true; }
      catch (err) { alert(errText(err)); }
    } else if (flag) { location.href = loginUrl(); }
  });

  const zone = el("reply-zone");
  if (!account) {
    zone.innerHTML = `<div class="composer"><div class="gate"><p>Sign in with your Patty Code account to reply.</p><a class="btn btn-primary" href="${loginUrl()}">Sign in</a></div></div>`;
    return;
  }
  zone.innerHTML = `
    <div class="msg error" id="reply-msg" hidden></div>
    <div class="composer">
      <textarea id="reply-body" placeholder="Write a reply… Markdown and \`\`\` code blocks supported."></textarea>
      <div class="foot"><span class="hint">Signed in as <b>${esc(account.handle)}</b></span><button class="btn btn-primary" id="reply-submit">Post reply</button></div>
    </div>`;
  el("reply-submit").addEventListener("click", async () => {
    const body = el("reply-body").value.trim();
    const msg = el("reply-msg");
    msg.hidden = true;
    if (body.length < 2) return;
    el("reply-submit").disabled = true;
    try {
      await forum(`/topics/${id}/posts`, { method: "POST", body: { body } });
      location.reload();
    } catch (err) {
      msg.textContent = errText(err); msg.hidden = false;
      el("reply-submit").disabled = false;
    }
  });
}

/* ── new topic ────────────────────────────────────── */
async function renderNew() {
  if (!account) {
    el("new-gate").hidden = false;
    el("gate-login").href = loginUrl();
    return;
  }
  el("new-form").hidden = false;
  const sel = el("f-category");
  try {
    const { categories } = await forum("/categories");
    for (const c of categories) {
      const o = document.createElement("option");
      o.value = c.id;
      o.textContent = catText(c.slug, c.name);
      sel.appendChild(o);
    }
    const pre = qp.get("category");
    if (pre) { const m = categories.find((c) => c.slug === pre); if (m) sel.value = m.id; }
  } catch {}

  el("f-submit").addEventListener("click", async () => {
    const msg = el("new-msg");
    msg.hidden = true;
    const categoryId = Number(sel.value);
    const title = el("f-title").value.trim();
    const body = el("f-body").value.trim();
    if (!categoryId) { msg.textContent = "Choose a category."; msg.hidden = false; return; }
    if (title.length < 6 || body.length < 10) { msg.textContent = "Add a title (6+ chars) and a bit more detail (10+ chars)."; msg.hidden = false; return; }
    el("f-submit").disabled = true;
    try {
      const { topic } = await forum("/topics", { method: "POST", body: { categoryId, title, body } });
      location.href = `/community/topic/?id=${topic.id}`;
    } catch (err) {
      msg.textContent = errText(err); msg.hidden = false;
      el("f-submit").disabled = false;
    }
  });
}

(async function () {
  initTheme();
  await loadAccount();
  if (el("topic-list")) renderHome();
  else if (el("posts")) renderThread();
  else if (el("new-form")) renderNew();
})();
