# LocalCTL

```text
 _                     _  ____ _____ _
| |    ___   ___ __ _| |/ ___|_   _| |
| |   / _ \ / __/ _` | | |     | | | |
| |__| (_) | (_| (_| | | |___  | | | |___
|_____\___/ \___\__,_|_|\____| |_| |_____|

          LOCAL MODEL CAPABILITY LAB
```

```text
╭──────────────────────────────────────────────────────────────────╮
│  LocalCTL v0.4 — KNOW THE TERRITORY                             │
│                                                                  │
│  DISCOVER → TEST → VERIFY → MAP → RECOMMEND → REFRESH           │
╰──────────────────────────────────────────────────────────────────╯
```

**Your Mac already owns compute. LocalCTL helps you learn which AI work that compute can actually handle — with evidence instead of vibes.**

Local models are easy to download and cheap to run. The expensive part is uncertainty.

Which model is actually useful for coding? Which one follows strict JSON contracts? Which one can reason through Linux or Docker failures? Which one only looked good once? Which evidence is six weeks old? Which model deserves a local-first workload, and which work still deserves a premium frontier model?

LocalCTL turns those questions into repeatable experiments and an inspectable capability map.

> **This is not another benchmark leaderboard.**
>
> LocalCTL measures the exact models you have, on the machine you actually own, under explicit runtime conditions, and remembers the evidence that supports every derived capability claim.

---

## The payoff

Without an evidence system, local-model decisions tend to sound like this:

```text
"Granite felt pretty good."
"That Qwen model is supposed to be strong at coding."
"I think this one is faster."
"Pretty sure it handled Docker logs last time."
```

LocalCTL is trying to get you to this instead:

```text
"This exact GGUF, on this machine, under the default profile,
has current comparable evidence for developer-core and structured-output.
Its Linux evidence is promising but incomplete.
Its Kubernetes evidence is stale.
Here are the exact run IDs behind those claims."
```

That changes the local-AI question from:

> **Can I run a model?**

into:

> **What has this model actually demonstrated, how current is that evidence, and where is it rational to use next?**

That is the wedge.

---

# Start in 60 seconds

LocalCTL currently targets macOS on Apple Silicon with Homebrew `llama.cpp` and GGUF models stored under LM Studio's model directory.

From the repo:

```bash
go build -o localctl .

./localctl check
./localctl models
```

Pick one discovered model reference and audition it:

```bash
./localctl audition granite
```

Then ask LocalCTL what the accumulated evidence supports:

```bash
./localctl capability granite
```

See the whole local fleet:

```bash
./localctl matrix
```

Ask a practical workload-placement question:

```bash
./localctl recommend developer-core
```

And find out what needs fresh evidence next:

```bash
./localctl requalify granite
```

That is the v0.4 loop:

```text
      ┌──────────┐
      │ DISCOVER │
      └────┬─────┘
           ▼
      ┌──────────┐
      │   TEST   │
      └────┬─────┘
           ▼
      ┌──────────┐
      │  VERIFY  │
      └────┬─────┘
           ▼
      ┌──────────┐
      │   MAP    │
      └────┬─────┘
           ▼
      ┌──────────┐
      │ RECOMMEND│
      └────┬─────┘
           ▼
      ┌──────────┐
      │ REFRESH  │
      └────┴─────┘
           ▲
           └──────── evidence keeps accumulating
```

---

# What changed in v0.4

v0.3 made LocalCTL a durable experiment workbench.

v0.4 gives that evidence a deterministic intelligence layer.

| | v0.3 — Find the Edges | v0.4 — Know the Territory |
| --- | --- | --- |
| Core question | What happened? | What does the accumulated evidence support now? |
| Unit | run / experiment | capability assessment |
| Identity | model + artifact + runtime + profile + machine | same boundary, enforced for comparison |
| Memory | durable observations | durable derived capability snapshots |
| Repeatability | `verify` | contributes to evidence strength |
| Comparison | controlled `headtohead` | fleet-wide `matrix` and workload `recommend` |
| Time | observation timestamp | explicit `CURRENT` / `AGING` / `STALE` |
| Gaps | `gaps` | `requalify` turns gaps/freshness into a next-test plan |
| Authority | none | still none |

The key change is not a prettier score.

It is this:

```text
RAW EVIDENCE
    │
    ▼
DETERMINISTIC DERIVATION
    │
    ▼
TRACEABLE CAPABILITY CLAIM
```

Every persisted v0.4 capability snapshot records both the **rule version** and the exact **source run IDs** behind each pack assessment.

No AI grades its own homework.

---

# The five commands that make v0.4 click

## 1. `audition` — find the edges

```bash
./localctl audition granite
```

An audition does more than send one prompt. It runs a bounded characterization session across core contracts and adaptive capability probes while preserving run evidence.

Use it when you have a model and your first question is simply:

> What kind of thing might this model be good for?

An audition can produce early evidence. It cannot produce universal trust.

---

## 2. `capability` — turn matching evidence into a map

```bash
./localctl capability granite
```

The capability map asks a stricter question:

> Which historical runs are actually comparable to the model, artifact, profile, pack version, and machine I am asking about right now?

Then it derives:

- evidence strength;
- freshness;
- capability state;
- deterministic exercise coverage;
- pass/fail shape;
- runtime reliability;
- pending human review;
- candidate roles;
- excluded historical evidence;
- exact source run provenance.

The result is also persisted separately under:

```text
~/.localctl/intelligence/capability/YYYY/MM/DD/
```

That separation matters:

```text
observation.json                 derived capability snapshot
---------------                  ---------------------------
what happened                    what the rules infer from it
immutable source evidence        regenerable derived knowledge
schema v3                        capability schema v1
                                 rule_version: v1
                                 source_run_ids: [...]
```

---

## 3. `matrix` — see your local fleet

```bash
./localctl matrix
```

The matrix compresses your installed models into a capability-oriented view.

Conceptually it looks like this:

```text
ILLUSTRATIVE SHAPE ONLY — THESE ARE NOT MEASURED RESULTS

MODEL                    DEV        LINUX      DOCKER     K8S        WRITING    STRUCT     REASON
model-a                  SUPPORTED  PROMISING  MIXED      ?          SUPPORTED  STRONG     MIXED
model-b                  MIXED      SUPPORTED  SUPPORTED  PROMISING  PROMISING  SUPPORTED  STRONG
model-c                  ?          ?          ?          ?          MIXED      PROMISING  PROMISING
```

Your actual output is generated from your own LocalCTL evidence corpus.

The matrix is not asking “which model wins?”

It is asking:

> **Where does each model currently have evidence?**

That is much more useful for local-first workload placement.

---

## 4. `recommend` — ask where a workload should go

```bash
./localctl recommend developer-core
./localctl recommend linux-investigation
./localctl recommend structured-output --profile=fast
```

`recommend` ranks **installed** models using only comparable current evidence for the requested pack/profile/machine/artifact boundary.

It uses a visible lexicographic order:

```text
capability state
    ↓
evidence strength
    ↓
pass rate
    ↓
coverage
    ↓
stable name tie-breaker
```

There is no hidden “AI quality = 87.3” magic score.

If the leader reaches `SUPPORTED` or `STRONG`, LocalCTL may call it the **evidence leader** for that comparison set.

That means:

> Among the installed models LocalCTL can responsibly compare here, this one currently has the strongest bounded evidence.

It does **not** mean:

- best model in the world;
- safe autonomous agent;
- correct on unseen work;
- permanent winner;
- authorized to execute anything.

Recommendation is decision support, not authority.

---

## 5. `requalify` — stop stale knowledge from becoming folklore

```bash
./localctl requalify granite
```

A capability claim should age.

v0.4 makes that explicit:

```text
0–14 days      CURRENT
15–45 days     AGING
46+ days       STALE
```

These are versioned LocalCTL heuristics, not laws of machine learning.

The important part is that “we tested this once a while ago” no longer silently means “we know this now.”

`requalify` produces a deterministic next-test plan based on stale, unknown, unreliable, weak, mixed, or aging evidence.

By default it changes nothing:

```bash
./localctl requalify granite
```

If you explicitly ask it to execute:

```bash
./localctl requalify granite --run
```

it runs only the single highest-priority pack from the plan.

Bounded on purpose.

---

# Read the capability states like an operator

v0.4 has three different dimensions. Do not collapse them into one grade.

## Evidence strength

How much comparable deterministic evidence exists?

```text
NONE
  ↓
EARLY
  ↓
DEVELOPING
  ↓
SUPPORTED
  ↓
STRONG
```

`HUMAN_ONLY` is a separate signal for evidence that exists but is not deterministically scored.

## Freshness

How recent is the newest comparable evidence?

```text
CURRENT   0–14 days
AGING     15–45 days
STALE     >45 days
UNKNOWN   no comparable evidence
```

## Capability state

What does the combination responsibly support?

| State | Plain-language reading |
| --- | --- |
| `UNKNOWN` | LocalCTL does not have comparable evidence yet. |
| `PROMISING` | Positive signal exists, but support is still early/developing. |
| `MIXED` | The model passes some deterministic work and fails enough to matter. |
| `WEAK` | Current deterministic pass evidence is below the v1 support floor. |
| `RUNTIME_UNRELIABLE` | Too many comparable inference attempts failed at the runtime boundary. |
| `REVIEW_REQUIRED` | Evidence is primarily manual and still needs human judgment. |
| `SUPPORTED` | Current comparable evidence clears the bounded v1 support rules. |
| `STRONG` | Current, broad, repeated deterministic evidence plus high pass rate. |
| `STALE` | The newest comparable evidence is too old under the current rule. |

None of these states means “authorized.”

---

# Why LocalCTL excludes some of your own history

This is a feature.

Suppose you have 40 old runs for “Granite.” Some used a different GGUF. Some used `fast`; some used `long-context`. The Linux pack changed. A few runs were from private incident logs. Some were produced before schema v3 recorded enough provenance.

A naive benchmark tool might throw all 40 into an average.

LocalCTL v0.4 asks whether each run belongs in the claim you are making now.

For canonical capability promotion, the v1 boundary requires:

```text
schema v3+
AND canonical input
AND exact pack ID
AND exact pack version
AND exact profile
AND comparable machine fingerprint
AND exact installed artifact SHA-256 when available
```

Anything else is counted as excluded evidence instead of quietly contaminating the result.

That is why a smaller number of clean runs can be more valuable than a larger pile of vaguely related history.

---

# Real work still matters

Canonical exercises are useful because they are comparable.

Real work is useful because it is real.

LocalCTL keeps both without pretending they are the same thing.

Pipe private material into a bounded work template:

```bash
git diff | ./localctl work pr-review granite

cat incident.log | ./localctl work linux-triage granite

cat docker-debug.txt | ./localctl work docker-triage granite

cat k8s-events.txt | ./localctl work kubernetes-triage granite

cat notes.md | ./localctl work summarize granite
```

Those runs are stored as `private` evidence and can be judged by a human:

```bash
./localctl judge <run-id> good "useful and grounded"
./localctl judge <run-id> partial "right diagnosis, weak next step"
./localctl judge <run-id> bad "invented a fact not present in the input"
```

But v0.4 does not let uncontrolled private work silently inflate canonical capability state.

```text
PRIVATE REAL WORK
      │
      ├── durable evidence      yes
      ├── human judgment        yes
      └── canonical promotion   no
```

That boundary is deliberate.

---

# Local-first does not mean cloud-hostile

The goal is not “never pay for AI again.”

The goal is to stop paying premium intelligence prices for work your own machine has already demonstrated it can handle well enough.

Think in workload classes:

```text
                         NEEDS FRONTIER / PREMIUM
                                  ▲
                                  │
      novel architecture ─────────┤
      ambiguous high-risk work ───┤
      difficult synthesis ─────────┤
                                  │
──────────────────────────────────┼─────────────────────────────
                                  │
      formatting / extraction ─────┤
      bounded summaries ────────────┤
      routine code explanation ─────┤
      known ops investigation ───────┤
                                  │
                                  ▼
                          PLAUSIBLY LOCAL
```

The line is not fixed.

LocalCTL exists to move that line using evidence from **your** hardware and **your** models.

A frontier model may still be the correct choice. LocalCTL just makes “because I never checked whether local was enough” a weaker reason.

---

# Capability packs: test the work, not the reputation

v0.4 reasons over versioned capability packs.

List them:

```bash
./localctl packs
```

Current pack families include:

| Pack | What it probes |
| --- | --- |
| `core-baseline` | fast cross-capability smoke test |
| `developer-core` | coding semantics, commits, diffs, PRs, regression thinking, bug triage |
| `linux-investigation` | processes, filesystems, networking, resources, services, incident reasoning |
| `docker-investigation` | lifecycle, images, storage, networking, builds, troubleshooting |
| `kubernetes-investigation` | workloads, scheduling, services, probes, storage, RBAC, networking, rollouts |
| `writing-summarization` | faithful summaries, concise rewriting, technical explanation |
| `structured-output` | strict output contracts, JSON, extraction, classification |
| `reasoning-analysis` | constraints, uncertainty, grounding, contradiction, evidence distinctions |

Inspect one:

```bash
./localctl pack show developer-core
```

Run it directly:

```bash
./localctl pack run developer-core granite
```

A pack has a version because the test definition is part of the evidence identity.

```text
"passed developer-core"
```

is incomplete.

```text
"passed developer-core/v1 under this profile and artifact"
```

is a claim LocalCTL can reason about later.

---

# Want a fair two-model fight?

Use `headtohead` instead of eyeballing unrelated history.

```bash
./localctl headtohead granite ministral
```

The important part is not the word “versus.”

It is that LocalCTL deliberately creates a comparison session under the same machine, capability pack, and profile rather than pretending two arbitrary piles of runs are controlled.

For deeper historical shape afterward:

```bash
./localctl capability granite
./localctl capability ministral
./localctl matrix
```

---

# Repeatability before confidence

A model can get an answer right once by chance, prompt sensitivity, or ordinary probabilistic variation.

Use:

```bash
./localctl verify go-slice-alias-v1 granite
```

Repeatability labels include:

```text
STABLE_PASS
USUALLY_PASS
FLAKY
USUALLY_FAIL
STABLE_FAIL
UNKNOWN
```

The repeatability evidence flows into the same durable run history that v0.4 later derives capability state from.

One success is an observation.

Repeated evidence is a pattern.

Neither is automatic authority.

---

# The intelligence layer can be audited too

Raw evidence has its own integrity checks:

```bash
./localctl evidence audit
```

v0.4 adds a separate audit for derived knowledge:

```bash
./localctl intelligence list
./localctl intelligence show <snapshot-id>
./localctl intelligence audit
```

A derived capability snapshot contains:

```text
rule_version
model identity
artifact SHA-256 when available
profile
machine fingerprint
pack assessments
source_run_ids[]
```

`intelligence audit` checks that the derivation retains its source chain.

This means a future system can ask:

> Why does LocalCTL currently say this model is supported for developer work?

and trace the answer back through:

```text
CAPABILITY SNAPSHOT
       │
       ├── rules: v1
       │
       └── source_run_ids
                │
                ▼
         OBSERVATION FILES
                │
                ▼
       PROMPTS / RESPONSES / TELEMETRY
```

That is the foundation for a trustworthy analyst layer later.

---

# Evidence lives on disk, not in a vibe cache

Raw run evidence:

```text
~/.localctl/runs/
└── YYYY/
    └── MM/
        └── DD/
            └── run_.../
                ├── observation.json
                ├── prompt.txt
                ├── response.txt
                └── judgment.json      # when a human judges it
```

Derived capability intelligence:

```text
~/.localctl/intelligence/
└── capability/
    └── YYYY/
        └── MM/
            └── DD/
                └── cap_...json
```

List recent raw runs:

```bash
./localctl runs
```

Inspect one:

```bash
./localctl show <run-id>
```

Historical characterization remains available too:

```bash
./localctl report granite
./localctl report granite --json
./localctl gaps granite
```

The distinction is intentional:

```text
report       = historical characterization
capability   = current strict comparable-evidence derivation
matrix       = current fleet projection
recommend    = advisory workload comparison
```

---

# Profiles are part of the claim

List profiles:

```bash
./localctl profiles
```

Built-ins currently include:

```text
default       context 2048   temperature 0   max tokens 512
fast          context 2048   temperature 0   max tokens 256
long-context  context 8192   temperature 0   max tokens 1024
```

Use one explicitly:

```bash
./localctl audition granite --profile=fast
./localctl capability granite --profile=fast
./localctl matrix --profile=fast
./localctl recommend structured-output --profile=fast
```

Create your own:

```bash
./localctl profile create coding-long \
  --context=8192 \
  --max-tokens=1536 \
  --temperature=0
```

If a managed runtime is already running with a different profile, LocalCTL restarts it instead of silently pretending the experiment conditions stayed the same.

---

# The glass box is still there

The learner UX is intentionally simple. The underlying system remains inspectable.

```text
shell
  │
  │ starts
  ▼
localctl executable / Go process
  │
  ├── process control
  ├── evidence capture
  ├── experiment grouping
  └── deterministic capability derivation
  │
  │ HTTP
  ▼
llama-server process
  │
  │ contains llama.cpp inference runtime
  ▼
Metal
  │
  ▼
Apple GPU / unified memory
  ▲
  │
GGUF model artifact
```

Important boundary:

`llama-server` is the OS process. The llama.cpp inference machinery runs inside it; llama.cpp is not another child process underneath the server.

Inspect the runtime directly:

```bash
./localctl runtime status
./localctl runtime inspect
./localctl runtime infer "Reply exactly RUNTIME_OK"
./localctl runtime stop
```

The higher-level commands automate lifecycle work, but the glass-box commands remain available because automation is easier to trust when you can inspect beneath it.

---

# What LocalCTL observes

Depending on what `llama-server` exposes, a schema-v3 observation can include:

- run ID;
- session ID;
- experiment ID and kind;
- exercise ID/version/category/difficulty;
- capability pack ID/version;
- canonical vs private input class;
- machine OS/architecture/chip/memory;
- LocalCTL build/version identity;
- model ID/name/path/file size;
- GGUF SHA-256;
- quantization parsed from the artifact name when available;
- runtime PID/executable/version;
- profile identity;
- requested context/temperature/max tokens;
- observed managed-runtime context;
- point-in-time `llama-server` process RSS when available;
- prompt and response hashes;
- finish reason;
- token counts;
- prompt and generation throughput;
- client-observed elapsed time;
- deterministic evaluation result;
- failure classification;
- validation authority.

Missing telemetry stays missing. LocalCTL does not invent measurements to make reports look complete.

One subtle example:

```text
runtime RSS != peak memory
runtime RSS != total Metal memory
runtime RSS != total unified-memory pressure
```

If LocalCTL only observed process RSS at one point in time, that is what it calls it.

---

# The proof ladder

LocalCTL deliberately refuses to turn one successful call into a broad capability claim.

```text
discovered
    ↓
loadable
    ↓
ready
    ↓
responsive
    ↓
contract compliant
    ↓
task capable
    ↓
repeatable
    ↓
evidence supported
    ↓
current
    ↓
candidate for a bounded role
```

Even that last step is still not execution authority.

Some distinctions LocalCTL tries hard not to blur:

```text
installed != running
running != ready
ready != reachable
reachable != responsive
responsive != inference succeeded
inference succeeded != contract followed
contract followed once != reliable
artifact exists != model loaded
requested config != observed config
observation != inference
inference != qualification
qualification != selection
recommendation != authority
capability != authority
```

Those are not philosophical slogans. They determine what the code is allowed to claim.

---

# No AI grades its own homework

v0.4 capability intelligence is deliberately deterministic.

The model does not look at its own run history and decide:

```text
"I am 92% qualified for Docker."
```

LocalCTL derives state from inspectable rules over recorded evidence.

A future AI analyst may be useful for questions like:

- Why does model A appear stronger than model B for this workload?
- What failure patterns keep repeating?
- Which new experiment would discriminate between two hypotheses?
- What does this capability map imply for my actual development workflow?

But when that layer arrives, its analysis should have separate provenance.

The deterministic map remains the map.

The analyst interprets the map.

---

# The road from experiments to a local-model operating system

```text
v0.1 / v0.2
  runtime truth + durable exercise evidence
        │
        ▼
v0.3 — FIND THE EDGES
  sessions
  experiments
  profiles
  capability packs
  repeatability
  private real work
  evidence schema v3
        │
        ▼
v0.4 — KNOW THE TERRITORY
  strict comparability
  evidence strength
  freshness
  capability state
  candidate roles
  fleet matrix
  advisory recommendations
  requalification
  derived-intelligence provenance
        │
        ▼
v0.5 — ANALYZE THE TERRITORY     future
  bounded AI analyst
  evidence synthesis
  comparative explanations
  uncertainty narration
        │
        ▼
later
  explicit qualification / eligibility / selection
  only after the evidence model earns it
```

The architecture can become sophisticated later without making the truth layer vague now.

---

# What LocalCTL will not do in v0.4

LocalCTL does **not** currently:

- automatically route your real work to a model;
- grant `DELEGATE`, `ASSIST`, or autonomous execution authority;
- let an LLM assign its own capability state;
- run arbitrary model-generated code on the host;
- mutate your repo because a model passed a coding pack;
- claim benchmark success equals production qualification;
- mix private real-work evidence into canonical scores;
- claim published GGUF size equals actual runtime memory use;
- claim one machine's results universally describe another machine;
- claim local models eliminate the need for frontier models;
- hide stale or incompatible evidence just to preserve a nice score.

Those omissions are part of the design.

---

# A practical learning session

If you want to learn the system rather than merely run it, use this sequence:

```bash
./localctl check
./localctl models
```

Pick a model.

Before running anything, predict what you think it will be good at.

```bash
./localctl audition granite
```

Then inspect what was saved:

```bash
./localctl runs
./localctl show <run-id>
```

Now derive the map:

```bash
./localctl capability granite
```

Inspect the derivation:

```bash
./localctl intelligence list
./localctl intelligence show <snapshot-id>
```

Find the weakest knowledge:

```bash
./localctl requalify granite
```

Compare the fleet:

```bash
./localctl matrix
./localctl recommend developer-core
```

Then ask yourself:

1. Which claims are observations?
2. Which claims are deterministic derivations?
3. Which runs were excluded from the capability map?
4. What changed because of freshness?
5. Would the recommendation change under `fast`?
6. What evidence would have to exist before you would trust the local model with more consequential work?

That is the learning loop LocalCTL is built to make cheap.

---

# Command deck

## Front door

```text
localctl lab
localctl check
localctl models
```

## Characterize models

```text
localctl audition <model> [--profile=default]
localctl headtohead <model-a> <model-b> [--profile=default]
localctl verify <exercise> <model> [--profile=default]
```

## Capability intelligence

```text
localctl capability <model> [--profile=default] [--json]
localctl matrix [--profile=default]
localctl recommend <pack> [--profile=default] [--json]
localctl requalify <model> [--profile=default] [--run]
localctl intelligence list
localctl intelligence show <snapshot-id>
localctl intelligence audit
```

## Capability packs and missions

```text
localctl packs
localctl pack show <pack>
localctl pack run <pack> <model> [--profile=default]
localctl missions
localctl mission show <mission>
localctl mission run <mission> <model> [--profile=default]
```

## Real private work

```text
<input> | localctl work pr-review <model>
<input> | localctl work summarize <model>
<input> | localctl work commit <model>
<input> | localctl work linux-triage <model>
<input> | localctl work docker-triage <model>
<input> | localctl work kubernetes-triage <model>
```

## Profiles

```text
localctl profiles
localctl profile show <profile>
localctl profile create <name> [--context=N] [--max-tokens=N] [--temperature=N]
```

## Historical evidence

```text
localctl runs
localctl show <run-id>
localctl judge <run-id> <good|partial|bad> [reason]
localctl report <model> [--json]
localctl gaps <model> [--json]
localctl insights [model]
localctl compare <model-a> <model-b>
localctl evidence audit
localctl evidence rebuild-index
```

## Exercise lab

```text
localctl exercises [category] [--all]
localctl exercise show <exercise-id>
localctl exercise run <exercise-id> [model]
localctl suite <category> [model]
localctl try <model> <prompt>
localctl baseline [model] [--all] [--category=...]
```

## Runtime glass box

```text
localctl runtime start [model] [--profile=default]
localctl runtime status
localctl runtime inspect
localctl runtime infer <prompt>
localctl runtime stop
```

---

# Current platform assumptions

v0.4 is intentionally narrow while the operating model is still being proven.

Current assumptions include:

- macOS;
- Apple Silicon;
- Homebrew `llama-server` at `/opt/homebrew/bin/llama-server`;
- managed runtime at `http://127.0.0.1:8080`;
- GGUF discovery under `~/.lmstudio/models`;
- one LocalCTL-managed `llama-server` at a time;
- local durable evidence under `~/.localctl`.

If your environment differs, treat the current code as a transparent narrow implementation rather than a generic cross-platform abstraction that does not exist yet.

---

# Current limitations worth knowing

Some boundaries are especially important when reading v0.4 output:

- capability thresholds are explicit v1 heuristics, not statistical guarantees;
- machine matching is conservative and does not yet model hardware-equivalence classes;
- recommendation is based on current installed models, not the universe of available models;
- point-in-time process RSS is not a peak-memory profiler;
- open-ended manual tasks still require human judgment;
- generated code is not compiled/executed in a safety sandbox yet;
- historical `compare` remains a broad historical roll-up and is not the same thing as v0.4 strict `recommend`;
- model discovery is currently LM Studio-directory-specific;
- freshness is based on the newest comparable run in the capability assessment;
- a capability snapshot is a derivation at a point in time, not a fact that rewrites old evidence.

Read [`docs/V0.4.md`](docs/V0.4.md) for the exact derivation contract.

Read [`docs/V0.3.md`](docs/V0.3.md) for the evidence foundation underneath it.

---

# Why Go?

LocalCTL's current job is close to the machine:

- find artifacts;
- own a process boundary;
- make HTTP requests;
- measure what comes back;
- persist evidence;
- derive small deterministic state machines from that evidence;
- fail loudly when the facts are insufficient.

Go is a good fit for that truth layer.

A later long-lived orchestration or analyst layer may have different needs. The point is not to force every future problem into Go because the first layer is written in Go.

For now:

```text
GO
  machine truth
  runtime lifecycle
  experiment evidence
  deterministic capability intelligence

LATER, IF EARNED
  richer orchestration
  analyst workflows
  longitudinal knowledge interfaces
  explicit selection policy
```

---

# The rule

```text
DO NOT MERELY RUN LOCAL MODELS.

MAKE THEIR BEHAVIOR MEASURABLE ENOUGH
THAT YOU KNOW WHEN THEY ARE WORTH USING.
```

And in v0.4:

```text
DO NOT MERELY COLLECT EVIDENCE.

KNOW WHICH CLAIMS THAT EVIDENCE SUPPORTS,
HOW OLD THOSE CLAIMS ARE,
AND EXACTLY WHAT THEY CAME FROM.
```

That is LocalCTL.
