# LocalCTL Web Lab

LocalCTL now has a local web operator interface over the same experiment, evidence, capability, recommendation, and runtime machinery used by the CLI.

The product loop is:

```text
MAP → EXPLORE → USE
        │
        ├── HISTORY
        ├── MODELS
        └── SETTINGS
```

The GUI is not a chat client, benchmark leaderboard, model launcher, or shell wrapper. It is an operating surface over LocalCTL's evidence system.

## Run it

Build LocalCTL normally, then start the web lab:

```bash
go build -o localctl .
./localctl lab web
```

The server binds only to:

```text
http://127.0.0.1:7331
```

A different loopback port can be selected explicitly:

```bash
./localctl lab web --port=7441
```

`localctl lab` without `web` retains the existing text learner front door.

The packaged Go binary embeds the built frontend assets. Node is not required to run the resulting LocalCTL binary.

## The application boundary

The web UI does not shell out to `localctl` or parse CLI output.

```text
                         CLI
                          │
                          ▼
                 LocalCTL application
                 operations/projections
                          ▲
                          │
                    HTTP / SSE
                          ▲
                          │
                       Web UI
```

The first extraction deliberately remains inside the existing Go package. Moving files into an `application/` package before the contracts stabilized would add churn without strengthening the boundary. The important property is that CLI formatting and HTTP presentation no longer define machine/evidence meaning.

`runV04Lab` now consumes the same `InspectMachine` operation used by HTTP bootstrap. The web API uses application operations for territory, models, missions, history, Scout, recommendations, settings, and read-index rebuild.

## Authority model

The existing authority ladder remains intact:

```text
OBSERVATION
exact durable run evidence

        ↓

DERIVED CAPABILITY INTELLIGENCE
deterministic + versioned
exact source run IDs retained

        ↓

ADVISORY RECOMMENDATION
evidence-relative decision support

        ↓

FUTURE / OPTIONAL ANALYST
hypotheses and explanation only
```

The GUI must not collapse these layers.

In particular:

```text
observation != derived intelligence

derived intelligence != qualification

recommendation != execution authority

presentation != truth
```

No LLM analyst is used to derive Territory state or Scout priorities in this release.

## Source evidence and read index

LocalCTL already had the safer storage shape requested for the GUI.

Canonical observations remain individual immutable source artifacts:

```text
~/.localctl/runs/YYYY/MM/DD/<run-id>/observation.json
```

The existing query index remains:

```text
~/.localctl/runs/index.ndjson
```

The NDJSON file is a rebuildable projection. It is not the authority for historical truth.

The Settings screen can rebuild it by walking the durable `observation.json` files. Deleting the index does not delete evidence.

Derived capability snapshots remain separate:

```text
~/.localctl/intelligence/capability/YYYY/MM/DD/<snapshot-id>.json
```

They retain `rule_version` and exact `source_run_ids`.

SQLite was intentionally not added. The repository already had a rebuildable local read model, and adding another database would create a second migration/recovery surface without solving a demonstrated query problem at the current evidence volume.

## MAP — Territory

The Map asks:

> What has this machine actually demonstrated with an installed local model?

Initial territories are mapped to existing canonical packs:

| Territory | Canonical source |
| --- | --- |
| Coding | `developer-core` |
| Structured output | `structured-output` |
| Reasoning | `reasoning-analysis` |
| Linux troubleshooting | `linux-investigation` |
| Docker troubleshooting | `docker-investigation` |
| Kubernetes troubleshooting | `kubernetes-investigation` |
| Writing | `writing-summarization` |
| Long context | no dedicated pack yet → `UNKNOWN` |
| Agent / tool use | no dedicated pack yet → `UNKNOWN` |

For each mapped area, LocalCTL compares installed models using the existing v0.4 recommendation ordering:

1. capability state;
2. evidence strength;
3. pass rate;
4. deterministic coverage;
5. stable model-name tie break.

That produces a machine-level evidence leader only when a model has actual `SUPPORTED` or `STRONG` evidence. There is no aggregate AI score.

A Map cell exposes:

- state;
- strength;
- freshness;
- comparable run count;
- evidence leader when established;
- rule and pack version;
- exact source run IDs;
- derivation limitations.

`Why?` resolves into exact source observations in History.

### Long context and tool use

LocalCTL has a `long-context` runtime profile, but a profile is not a capability definition. There is no versioned long-context pack yet, so the GUI does not borrow unrelated results and call long context proven.

Likewise, developer/structured/reasoning success is useful prerequisite evidence for a coding agent but is not proof of tool use. Agent/tool-use remains `UNKNOWN` until LocalCTL has a real canonical experiment for it.

## EXPLORE — Missions and Scout

A Mission is a deterministic evidence plan, not a finding.

The application planner resolves a human question into current LocalCTL packs and checks each prerequisite against current comparable evidence.

A pack is skipped when it is already `SUPPORTED` or `STRONG` with `CURRENT` freshness. Stale, aging, unknown, weak, mixed, review-required, or runtime-unreliable prerequisites remain runnable work.

The initial mission catalog includes:

- local coding;
- systems investigation;
- move bounded work local;
- coding-agent prerequisites.

The coding-agent mission deliberately contains an unresolved non-pack requirement for `agent-tool-use`. It can improve measurable prerequisites, but it cannot mark the mission complete from proxy evidence.

Scout is deterministic. It can surface:

- stale/aging evidence;
- unknown/weak/mixed areas;
- untested installed models;
- selected remote candidates;
- coding-agent prerequisite gaps;
- capability areas for which no canonical experiment exists yet.

It does not call an LLM or promote a hypothesis into canonical state.

## Durable jobs

GUI-triggered missions are represented by durable job records under:

```text
~/.localctl/jobs/<job-id>.json
```

Jobs are operational truth about GUI execution. They are not capability evidence.

Lifecycle states are:

```text
PLANNED
QUEUED
STARTING_RUNTIME
RUNNING
EVALUATING
PERSISTING
COMPLETED

FAILED
CANCELLED
INTERRUPTED
```

Key properties:

- the browser is not the owner of experiment completion;
- reloading the browser does not lose job state;
- a LocalCTL restart marks any non-terminal prior job `INTERRUPTED` rather than silently pretending to resume it;
- duplicate submission of the same active mission/model/profile returns the existing active job;
- distinct GUI execution jobs are serialized in this release because the existing observation scope and managed llama.cpp runtime are process-global/single-owner;
- deterministic exercise failures are evidence, not job-process failures;
- runtime startup, inference, evaluation/derivation, and persistence failures retain distinct error classes.

### Cancellation semantics

In-flight inference uses a context-bound HTTP request. Cancelling a job aborts that request.

An operator cancellation is not model behavior, so LocalCTL does **not** persist the cancelled request as an inference failure observation. Doing so would incorrectly lower runtime-reliability/capability state because a human pressed Cancel.

The existing runtime startup path is blocking. A cancellation requested during `STARTING_RUNTIME` is honored as soon as startup returns; LocalCTL leaves the reconciled shared runtime in place rather than killing a runtime that other CLI work may own. This is an explicit current limitation.

## SSE progress

The browser receives progress over Server-Sent Events.

Events include:

```text
job.planned
job.queued
runtime.starting
runtime.ready
experiment.started
exercise.started
measurement.recorded
observation.persisted
exercise.completed
experiment.completed
capability.updated
job.failed
job.cancelled
job.completed
job.interrupted
```

The event stream is not durable truth. The job JSON record is.

If the stream drops, the browser reconnects/reads the job state; experiment completion does not depend on the connection staying open.

## HISTORY — inspectability

History exposes recent:

- sessions;
- experiments;
- runs;
- failures through run/job status;
- human judgments;
- capability snapshot summaries.

A run detail resolves the exact `observation.json` and, when present, its prompt, response, and human judgment. It exposes model/artifact identity, machine, profile, pack, validation authority, result, and evaluation.

The GUI does not create a new claim provenance system. Territory cells keep the existing source-run IDs and link back to these source observations.

## MODELS — inventory, not a leaderboard

The Models screen distinguishes:

```text
INSTALLED
TESTED
DEMONSTRATED
```

A local GGUF scan establishes installation only.

`TESTED` means LocalCTL has saved run evidence for that artifact/model identity.

`DEMONSTRATED` means at least one current pack assessment is `SUPPORTED` or `STRONG`.

Remote candidate selection remains separate. A selected candidate is not called installed, compatible, tested, or good.

To avoid a hostile first-load cost, the GUI does not hash every untested GGUF just to render inventory. Exact artifact hashing is performed when a tested model's evidence must be compared. Runs already compute/cache artifact identity, so this usually reuses the existing digest cache.

## USE — conservative configuration handoff

Use is workload-oriented, not model-oriented.

A configuration preview is produced only when a workload has a current `SUPPORTED` or `STRONG` evidence leader.

The preview contains the local OpenAI-compatible endpoint, exact installed model path/ID, selected LocalCTL profile, and a human-readable LocalCTL runtime launch preview.

Initial actions are intentionally bounded:

- preview configuration;
- copy configuration;
- export configuration JSON;
- copy the LocalCTL launch preview.

The GUI does not automatically write editor/agent configuration files and does not expose host mutation endpoints.

## Security model

Localhost is treated as a real security boundary.

The web lab implements:

- hard-coded `127.0.0.1` binding;
- Host validation allowing only `127.0.0.1`/`localhost`;
- same-origin Origin validation on mutations;
- an ephemeral per-process CSRF token required in `X-LocalCTL-CSRF` for mutations;
- cross-site fetch rejection;
- no CORS headers and rejected preflight requests;
- 64 KiB mutation body limit;
- strict JSON decoding with unknown-field rejection;
- CSP, frame denial, referrer suppression, and MIME-sniffing protection;
- fixed API operations rather than generic filesystem or command execution;
- no arbitrary shell-command HTTP endpoint.

The CSRF token is obtained from same-origin bootstrap. Same-origin policy prevents a remote page from reading it, while the custom mutation header prevents simple cross-origin form submission.

This is a single-user local tool, not a remote/multi-user service. It should not be rebound to `0.0.0.0` without designing a different authentication and threat model.

## Frontend development

The browser code is TypeScript with no runtime framework dependency.

```bash
cd web
npm install
npm test
npm run build
```

`npm run build` emits the assets embedded by Go under:

```text
web/dist/
```

The built assets are committed so an end user building/running Go does not need Node just to serve the interface. CI rebuilds them to catch source/dist drift through the normal TypeScript compilation and frontend tests.

## Current seam for a future analyst

A later analyst can consume:

- Territory projections;
- mission state;
- Scout items;
- run evidence;
- capability snapshots.

Its output must be separately attributed as a hypothesis/explanation layer.

It must not write canonical capability state, change deterministic derivation rules implicitly, grant execution authority, or turn generated prose into evidence.

That seam is intentionally left unused in this release.
