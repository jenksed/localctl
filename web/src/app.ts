import { capabilityLabel, escapeHTML, formatDate, stateClass, terminalJob } from "./viewmodel.js";

type Obj = Record<string, any>;
type Page = "MAP" | "MODELS" | "EXPLORE" | "HISTORY" | "USE" | "SETTINGS";

const root = document.querySelector<HTMLDivElement>("#app");
if (!root) throw new Error("missing #app");

let csrf = "";
let bootstrap: Obj | null = null;
let activeJob = "";
let events: EventSource | null = null;

const pages: Page[] = ["MAP", "MODELS", "EXPLORE", "HISTORY", "USE", "SETTINGS"];

function page(): Page {
  const raw = location.hash.replace(/^#\/?/, "").toUpperCase();
  return pages.includes(raw as Page) ? raw as Page : "MAP";
}

async function getJSON<T = any>(path: string): Promise<T> {
  const response = await fetch(path, { headers: { Accept: "application/json" } });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body?.error?.message ?? `HTTP ${response.status}`);
  return body as T;
}

async function postJSON<T = any>(path: string, body: Obj = {}): Promise<T> {
  const response = await fetch(path, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      "X-LocalCTL-CSRF": csrf,
    },
    body: JSON.stringify(body),
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(payload?.error?.message ?? `HTTP ${response.status}`);
  return payload as T;
}

function machineLine(): string {
  const m = bootstrap?.machine;
  if (!m) return "LOCAL ONLY";
  const machine = m.machine ?? {};
  const runtime = m.runtime ?? {};
  const parts = [machine.chip || `${machine.os ?? "?"}/${machine.architecture ?? "?"}`];
  parts.push(`${m.models ?? 0} model${m.models === 1 ? "" : "s"}`);
  parts.push(`${m.evidence_runs ?? 0} runs`);
  parts.push(runtime.ready ? `runtime ${runtime.model_id ?? "ready"}` : "runtime stopped");
  return parts.map(escapeHTML).join(" · ");
}

function shell(content: string): void {
  const current = page();
  root!.innerHTML = `
    <div class="shell">
      <header class="topbar">
        <a class="brand" href="#/MAP"><span>LCTL</span><div><strong>LocalCTL</strong><small>LOCAL AI CAPABILITY LAB</small></div></a>
        <div class="machine">${machineLine()}</div>
      </header>
      <nav>${pages.map(p => `<a class="${p === current ? "active" : ""}" href="#/${p}">${p}</a>`).join("")}</nav>
      ${activeJob ? `<div class="jobbar"><span class="pulse"></span>Experiment active <code>${escapeHTML(activeJob)}</code><button data-open-job="${escapeHTML(activeJob)}">Inspect</button></div>` : ""}
      <main>${content}</main>
      <footer>observation ≠ derived capability ≠ recommendation ≠ authority</footer>
    </div>`;
  wireActions();
}

function statePill(state: unknown, freshness: unknown): string {
  const label = capabilityLabel(state, freshness);
  return `<span class="pill ${stateClass(label)}">${escapeHTML(label)}</span>`;
}

function fail(title: string, error: unknown): void {
  shell(`<section class="hero"><p class="eyebrow">LOCAL STATE UNAVAILABLE</p><h1>${escapeHTML(title)}</h1><p class="error">${escapeHTML(error instanceof Error ? error.message : error)}</p><button data-reload>Retry</button></section>`);
}

function loading(title: string): void {
  shell(`<section class="hero"><p class="eyebrow">READING DURABLE LOCAL STATE</p><h1>${escapeHTML(title)}</h1><div class="skeleton"></div></section>`);
}

async function renderMap(): Promise<void> {
  loading("What has this machine actually demonstrated?");
  try {
    const territory = await getJSON<Obj>("/api/v1/territory");
    const cells = territory.cells ?? [];
    shell(`
      <section class="hero"><p class="eyebrow">YOUR LOCAL AI TERRITORY</p><h1>What can this machine actually do?</h1><p>Installed is inventory. Tested is evidence. Demonstrated capability requires comparable LocalCTL observations.</p></section>
      <section class="grid territory">${cells.map((c: Obj) => `
        <article class="card">
          <div class="cardhead"><h2>${escapeHTML(c.title)}</h2>${statePill(c.state, c.freshness)}</div>
          <p>${escapeHTML(c.question)}</p>
          <dl><div><dt>Evidence</dt><dd>${escapeHTML(c.strength ?? "NONE")}</dd></div><div><dt>Freshness</dt><dd>${escapeHTML(c.freshness ?? "UNKNOWN")}</dd></div><div><dt>Comparable runs</dt><dd>${escapeHTML(c.comparable_runs ?? 0)}</dd></div></dl>
          ${c.evidence_leader ? `<p class="leader">Evidence leader: <strong>${escapeHTML(c.evidence_leader.model_name)}</strong></p>` : `<p class="muted">No evidence leader established.</p>`}
          ${(c.limitations ?? []).map((x: string) => `<p class="note">${escapeHTML(x)}</p>`).join("")}
        </article>`).join("")}</section>`);
  } catch (e) { fail("Territory could not be loaded", e); }
}

async function renderModels(): Promise<void> {
  loading("Installed models");
  try {
    const models = await getJSON<Obj[]>("/api/v1/models");
    shell(`<section class="hero"><p class="eyebrow">INVENTORY, NOT A LEADERBOARD</p><h1>Models on this machine</h1><p>A model being installed does not mean LocalCTL has demonstrated it is useful.</p></section>
      <section class="grid">${models.length ? models.map(m => `<article class="card"><div class="cardhead"><h2>${escapeHTML(m.name)}</h2>${m.active_runtime ? `<span class="pill state-supported">ACTIVE</span>` : ""}</div><p>${escapeHTML(m.size)} · ${escapeHTML(m.quantization || "quantization unknown")}</p><dl><div><dt>Status</dt><dd>${m.demonstrated ? "DEMONSTRATED" : m.tested ? "TESTED" : "UNTESTED"}</dd></div><div><dt>Saved runs</dt><dd>${escapeHTML(m.saved_runs)}</dd></div><div><dt>Freshness</dt><dd>${escapeHTML(m.freshness)}</dd></div></dl>${m.tested ? `<div class="tags">${Object.entries(m.capability_states ?? {}).map(([k,v]) => `<span class="${stateClass(v)}">${escapeHTML(k)} · ${escapeHTML(v)}</span>`).join("")}</div>` : `<p class="muted">No LocalCTL evidence yet.</p>`}</article>`).join("") : `<article class="empty"><h2>No GGUF models found</h2><p>Install a GGUF under the configured LM Studio model directory, then refresh LocalCTL.</p></article>`}</section>`);
  } catch (e) { fail("Models could not be loaded", e); }
}

async function renderExplore(): Promise<void> {
  loading("What is worth learning next?");
  try {
    const [missions, scout, models] = await Promise.all([
      getJSON<Obj[]>("/api/v1/missions"), getJSON<Obj[]>("/api/v1/scout"), getJSON<Obj[]>("/api/v1/models")
    ]);
    const model = models[0]?.id ?? "";
    const modelName = models[0]?.name ?? model;
    shell(`<section class="hero"><p class="eyebrow">EXPLORE</p><h1>Turn unknowns into evidence</h1><p>Missions use LocalCTL's existing bounded packs. Scout finds gaps; it does not invent model quality.</p></section>
      <h2 class="section-title">Missions</h2><section class="grid">${missions.map(m => `<article class="card"><h2>${escapeHTML(m.title)}</h2><p>${escapeHTML(m.question)}</p><p class="muted">${escapeHTML(m.description)}</p>${model ? `<button class="primary" data-start-mission="${escapeHTML(m.id)}" data-model="${escapeHTML(model)}">Run with ${escapeHTML(modelName)}</button>` : `<p class="note">Install a model before running a mission.</p>`}</article>`).join("")}</section>
      <h2 class="section-title">Scout</h2><section class="list">${scout.length ? scout.map(s => `<article><div><strong>${escapeHTML(s.title)}</strong><p>${escapeHTML(s.why)}</p></div><span>${s.runnable ? "RUNNABLE" : "INFORMATION"}</span></article>`).join("") : `<p class="muted">No additional gaps surfaced.</p>`}</section>`);
  } catch (e) { fail("Explore could not be loaded", e); }
}

async function renderHistory(): Promise<void> {
  loading("Evidence history");
  try {
    const [history, jobs] = await Promise.all([getJSON<Obj>("/api/v1/history?limit=50"), getJSON<Obj[]>("/api/v1/jobs?limit=20")]);
    shell(`<section class="hero"><p class="eyebrow">WHY DOES LOCALCTL BELIEVE THAT?</p><h1>History & evidence</h1><p>Runs are durable observations. Capability snapshots are derived separately and remain traceable to source run IDs.</p></section>
      <h2 class="section-title">Jobs</h2><section class="list">${jobs.length ? jobs.map(j => `<article><div><strong>${escapeHTML(j.mission_title || j.mission_id)}</strong><p>${escapeHTML(j.model_name)} · ${escapeHTML(j.state)} · ${formatDate(j.updated_at)}</p></div><button data-open-job="${escapeHTML(j.id)}">Inspect</button></article>`).join("") : `<p class="muted">No GUI jobs yet.</p>`}</section>
      <h2 class="section-title">Recent runs</h2><section class="list">${(history.runs ?? []).slice(0,30).map((r: Obj) => `<article><div><strong>${escapeHTML(r.exercise_id || r.run_id)}</strong><p>${escapeHTML(r.model?.id || r.model_id || "model")} · ${escapeHTML(r.evaluation?.status || "UNKNOWN")} · ${formatDate(r.started_at || r.timestamp)}</p></div><button data-open-run="${escapeHTML(r.run_id)}">Evidence</button></article>`).join("") || `<p class="muted">No durable runs yet.</p>`}</section>`);
  } catch (e) { fail("History could not be loaded", e); }
}

async function renderUse(): Promise<void> {
  loading("Bounded use recommendations");
  try {
    const items = await getJSON<Obj[]>("/api/v1/use");
    shell(`<section class="hero"><p class="eyebrow">USE</p><h1>What can reasonably move local?</h1><p>Only current evidence that reaches LocalCTL's supported threshold appears as supported. These are advisory previews, not execution authority.</p></section><section class="grid">${items.map(x => `<article class="card"><div class="cardhead"><h2>${escapeHTML(x.title)}</h2>${statePill(x.state, "CURRENT")}</div>${x.leader ? `<p>Evidence leader: <strong>${escapeHTML(x.leader.model_name)}</strong></p>` : `<p class="muted">No supported evidence leader.</p>`}<p>${escapeHTML(x.evidence_runs ?? 0)} source runs</p>${(x.limitations ?? []).map((l:string)=>`<p class="note">${escapeHTML(l)}</p>`).join("")}</article>`).join("")}</section>`);
  } catch (e) { fail("Use recommendations could not be loaded", e); }
}

async function renderSettings(): Promise<void> {
  loading("Local boundaries");
  try {
    const s = await getJSON<Obj>("/api/v1/settings");
    shell(`<section class="hero"><p class="eyebrow">SETTINGS & BOUNDARIES</p><h1>Local by construction</h1><p>The web app is a projection over LocalCTL. It does not become the source of capability truth.</p></section><section class="grid"><article class="card"><h2>Network</h2><dl><div><dt>Bind</dt><dd>${escapeHTML(s.bind_address)}</dd></div><div><dt>Runtime</dt><dd>${escapeHTML(s.runtime_url)}</dd></div></dl>${(s.security ?? []).map((x:string)=>`<p class="ok">✓ ${escapeHTML(x)}</p>`).join("")}</article><article class="card"><h2>Evidence</h2><p class="path">${escapeHTML(s.evidence_root)}</p><p>Read index: <span class="path">${escapeHTML(s.read_index)}</span></p><p class="muted">${escapeHTML(s.read_index_authority)}</p><button data-rebuild>Rebuild read index</button></article></section>`);
  } catch (e) { fail("Settings could not be loaded", e); }
}

async function render(): Promise<void> {
  switch (page()) {
    case "MODELS": return renderModels();
    case "EXPLORE": return renderExplore();
    case "HISTORY": return renderHistory();
    case "USE": return renderUse();
    case "SETTINGS": return renderSettings();
    default: return renderMap();
  }
}

async function startMission(mission: string, model: string): Promise<void> {
  try {
    const result = await postJSON<Obj>("/api/v1/jobs", { mission_id: mission, model_id: model, profile_id: "default" });
    const job = result.job;
    activeJob = job.id;
    watchJob(job.id);
    await render();
    await showJob(job.id);
  } catch (e) { alert(`Mission not started: ${e instanceof Error ? e.message : e}`); }
}

function watchJob(id: string): void {
  events?.close();
  events = new EventSource(`/api/v1/events?job_id=${encodeURIComponent(id)}`);
  events.onmessage = () => void 0;
  events.addEventListener("job.completed", () => finishJob());
  events.addEventListener("job.failed", () => finishJob());
  events.addEventListener("job.cancelled", () => finishJob());
  events.addEventListener("job.interrupted", () => finishJob());
}

function finishJob(): void {
  events?.close(); events = null; activeJob = ""; void render();
}

async function showJob(id: string): Promise<void> {
  try {
    const j = await getJSON<Obj>(`/api/v1/jobs/${encodeURIComponent(id)}`);
    modal(`<div class="modalhead"><div><p class="eyebrow">DURABLE JOB</p><h2>${escapeHTML(j.mission_title || j.mission_id)}</h2></div><button data-close>Close</button></div><p>${escapeHTML(j.model_name)} · ${statePill(j.state, "CURRENT")}</p>${j.error ? `<p class="error">${escapeHTML(j.error)}</p>` : ""}<div class="timeline">${(j.events ?? []).slice(-30).map((e:Obj)=>`<div><span>${escapeHTML(e.sequence)}</span><strong>${escapeHTML(e.type)}</strong><small>${formatDate(e.at)}</small></div>`).join("")}</div>${!terminalJob(j.state) ? `<button class="danger" data-cancel="${escapeHTML(j.id)}">Cancel experiment</button>` : ""}`);
  } catch (e) { alert(e instanceof Error ? e.message : String(e)); }
}

async function showRun(id: string): Promise<void> {
  try {
    const r = await getJSON<Obj>(`/api/v1/runs/${encodeURIComponent(id)}`);
    const o = r.observation ?? {};
    modal(`<div class="modalhead"><div><p class="eyebrow">SOURCE EVIDENCE</p><h2>${escapeHTML(id)}</h2></div><button data-close>Close</button></div><dl><div><dt>Model</dt><dd>${escapeHTML(o.model?.id || o.model_id || "—")}</dd></div><div><dt>Exercise</dt><dd>${escapeHTML(o.exercise_id || "—")}</dd></div><div><dt>Profile</dt><dd>${escapeHTML(o.profile?.id || o.profile_id || "—")}</dd></div><div><dt>Evaluation</dt><dd>${escapeHTML(o.evaluation?.status || "—")}</dd></div></dl>${r.prompt ? `<h3>Prompt</h3><pre>${escapeHTML(r.prompt)}</pre>` : ""}${r.response ? `<h3>Response</h3><pre>${escapeHTML(r.response)}</pre>` : ""}`);
  } catch (e) { alert(e instanceof Error ? e.message : String(e)); }
}

function modal(content: string): void {
  document.querySelector("#modal")?.remove();
  const el = document.createElement("div"); el.id = "modal"; el.className = "backdrop";
  el.innerHTML = `<section class="modal" role="dialog" aria-modal="true">${content}</section>`;
  document.body.append(el); wireActions();
}

function wireActions(): void {
  document.querySelectorAll<HTMLElement>("[data-reload]").forEach(x => x.onclick = () => void boot());
  document.querySelectorAll<HTMLElement>("[data-start-mission]").forEach(x => x.onclick = () => void startMission(x.dataset.startMission || "", x.dataset.model || ""));
  document.querySelectorAll<HTMLElement>("[data-open-job]").forEach(x => x.onclick = () => void showJob(x.dataset.openJob || ""));
  document.querySelectorAll<HTMLElement>("[data-open-run]").forEach(x => x.onclick = () => void showRun(x.dataset.openRun || ""));
  document.querySelectorAll<HTMLElement>("[data-close]").forEach(x => x.onclick = () => document.querySelector("#modal")?.remove());
  document.querySelectorAll<HTMLElement>("[data-cancel]").forEach(x => x.onclick = async () => { await postJSON(`/api/v1/jobs/${encodeURIComponent(x.dataset.cancel || "")}/cancel`); document.querySelector("#modal")?.remove(); });
  document.querySelectorAll<HTMLElement>("[data-rebuild]").forEach(x => x.onclick = async () => { const r = await postJSON<Obj>("/api/v1/index/rebuild"); alert(`Rebuilt read index from ${r.records} canonical observations.`); });
}

async function boot(): Promise<void> {
  try {
    bootstrap = await getJSON<Obj>("/api/v1/bootstrap");
    csrf = bootstrap.csrf_token ?? "";
    const jobs = await getJSON<Obj[]>("/api/v1/jobs?limit=10");
    const running = jobs.find(j => !terminalJob(j.state));
    activeJob = running?.id ?? "";
    if (activeJob) watchJob(activeJob);
    await render();
  } catch (e) { fail("Could not read the local lab", e); }
}

window.addEventListener("hashchange", () => void render());
void boot();
