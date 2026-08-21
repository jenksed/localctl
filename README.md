# LocalCTL

```text
 _                     _  ____ _____ _
| |    ___   ___ __ _| |/ ___|_   _| |
| |   / _ \ / __/ _` | | |     | | | |
| |__| (_) | (_| (_| | | |___  | | | |___
|_____\___/ \___\__,_|_|\____| |_| |_____|

          LOCAL MODEL CAPABILITY LAB
```

**Local models are cheap to run. The hard part is knowing which ones you can actually trust with useful work. LocalCTL helps you find out.**

LocalCTL is a small Go CLI that turns local-model experimentation into durable evidence on the machine you actually use.

It can:

- find GGUF models you already have through LM Studio;
- start and reuse `llama-server` for you;
- put models through coding, Linux, Docker, Kubernetes, reasoning, structured-output, writing, summarization, PR, Git, and troubleshooting workloads;
- distinguish wrong answers from contract failures, empty output, invalid JSON, and runtime failures;
- repeat uncertain tests to expose flaky behavior;
- remember every inference and its evidence locally;
- compare two models under the same controlled experiment;
- show which parts of a model are well tested and which parts are still unknown;
- let you try your own private diffs, logs, and technical material without pretending those runs are canonical benchmarks.

The practical goal is not to prove that cloud or frontier models are unnecessary. They are extremely useful.

The goal is to answer a more useful question:

> **Which work can the machine I already own handle locally, reliably enough that I do not need to spend premium model usage on it?**

Use expensive intelligence where it earns its cost. Find the rest.

---

## The 30-second mental model

You have a Mac. You download local models. Some are surprisingly capable. Some look good in a model card and fall apart on your actual work. Some are excellent at one thing and bad at another.

LocalCTL treats that as an experiment instead of a vibe:

```text
MODEL
  ↓
RUN A BOUNDED WORKLOAD
  ↓
OBSERVE WHAT ACTUALLY HAPPENED
  ↓
SAVE THE EVIDENCE
  ↓
REPEAT / COMPARE / FIND THE GAPS
  ↓
BUILD A MAP OF WHAT IS ACTUALLY USEFUL
```

Every time you test a model, LocalCTL remembers what happened.

Today, without evidence, you might say:

```text
"Granite seemed pretty good."
```

After enough LocalCTL runs, you can say something bounded and inspectable instead:

```text
"On this machine and profile, Granite has repeated evidence for
these workloads, these failure patterns, and this observed speed.
These other workloads are still under-tested."
```

That is a much better basis for deciding what should stay local, what still needs human review, and what still deserves a frontier model.

---

# Quick start

LocalCTL v0.3 is currently built for the development environment used by this project: macOS on Apple Silicon, Homebrew `llama-server`, and GGUF models under LM Studio's model directory.

You need Go and `llama-server` available first.

From the repository:

```bash
go build -o localctl .
```

Then ask the lab what it can see:

```bash
./localctl check
./localctl models
./localctl lab
```

`localctl lab` is the learner-facing front door. It shows the machine, discovered models, accumulated evidence, managed runtime state, and the next useful commands.

## The coolest first thing to try

Pick one model name from:

```bash
./localctl models
```

Then audition it:

```bash
./localctl audition granite
```

The audition:

1. checks whether the runtime/model can be used under the selected profile;
2. runs the 18-test core baseline;
3. samples useful capability areas;
4. spends additional tests where the early signal is mixed or weak;
5. points out failures worth repeatability testing;
6. leaves behind a historical characterization you can inspect later.

The runtime stays warm. Repeated work against the same model/profile reuses it instead of constantly stopping and starting it.

If you request another model, LocalCTL switches automatically.

## Put two models head-to-head

If you already have two models, this is the fastest way to make the project feel real:

```bash
./localctl headtohead granite ministral
```

Or choose a deeper workload pack:

```bash
./localctl headtohead granite ministral --pack=linux-investigation
```

The two sides run in one LocalCTL session with the same machine, pack version, and requested profile. That is intentionally different from casually comparing unrelated historical runs.

The command finishes with a controlled comparison of observed pass rate, manual-review count, latency, and generation throughput where available.

It still does **not** claim a universal winner. It tells you what happened on the workload you actually tested.

---

# What does it look like?

The exact values depend on your machine and model. The structure below comes from the native v0.3 command paths; dynamic IDs, timings, and scores are intentionally shown as placeholders rather than invented measurements.

```text
$ ./localctl audition granite

LocalCTL model audition
model: granite-...gguf
profile: default

Stage 1/5 — runtime fit
runtime: reusing ... with profile default (PID ...)
artifact: sha256:...
runtime: warm managed runtime reused
session: session_...
experiment: exp_...

Stage 2/5 — core contracts
[  1/ 18] exact-output-v1                    PASS     ...
[  2/ 18] lowercase-only-v1                  PASS     ...
...

Stage 3/5 — adaptive capability probes
...

Stage 4/5 — repeatability candidates
suggested verify: localctl verify ...

Stage 5/5 — early characterization
...

What this supports: an early bounded capability map for this exact model/profile/machine.
What this does not prove: general production reliability, safe autonomy, or capability outside tested workloads.
```

A failed deterministic task also records *how* it failed when LocalCTL can tell:

```text
contract_extra_output
incorrect_answer
empty_output
invalid_json
structured_mismatch
missing_required_concepts
inference_error
```

That matters. A model that knows the right answer but refuses to follow an exact-output contract has a different problem from a model that confidently reasons to the wrong answer.

---

# Why would I use this?

### “I want to use local models for coding.”

Start with:

```bash
./localctl mission run developer granite
```

Or inspect the underlying versioned pack:

```bash
./localctl pack show developer-core
./localctl pack run developer-core granite
```

The developer surface includes coding semantics, bug triage, diff/PR reasoning, commit-message work, test planning, structured output, and technical writing.

### “Could a small model actually help with Linux troubleshooting?”

```bash
./localctl pack run linux-investigation granite
```

The infrastructure catalog contains 100 scenarios across:

```text
Linux        34
Docker       33
Kubernetes   33
----------------
Total       100
```

They emphasize investigation boundaries—processes, sockets, storage, resource pressure, container/runtime behavior, probes, scheduling, Services, RBAC, networking, rollouts, and evidence-first triage—not just certification trivia.

### “I downloaded five models and have no idea which one matters.”

Use a fast controlled comparison:

```bash
./localctl headtohead granite ministral
```

Then inspect each accumulated history:

```bash
./localctl report granite
./localctl report ministral
```

### “I don't want to burn premium AI usage on simple work.”

Run the local-first mission:

```bash
./localctl mission run local-first granite
```

That samples bounded high-frequency work such as structured output, developer tasks, summarization, and reasoning.

It does **not** declare that a local model can replace a frontier model. It gives you evidence about where substitution might be rational to investigate.

### “I saw a new GGUF and everybody says it is amazing.”

Start with:

```bash
./localctl explore
./localctl explore --live
```

Then install one sensible quant, confirm LocalCTL sees it:

```bash
./localctl models
```

and audition it:

```bash
./localctl audition <model-reference>
```

The point is to move from hype to observed behavior quickly.

### “I want to try my actual work.”

Pipe private material into a bounded work template:

```bash
git diff | ./localctl work pr-review granite
```

```bash
cat incident.log | ./localctl work linux-triage granite
```

```bash
kubectl describe pod my-pod | ./localctl work kubernetes-triage granite
```

Real-work inputs are explicitly marked `private` and `non-canonical` in the v0.3 evidence model. They are useful for learning whether benchmark behavior transfers to your work without mixing those inputs into canonical pack identity.

The v0.3 input limit for `work` is 2 MiB.

---

# Repeatability: one pass is not reliability

LLMs are probabilistic systems. Even with temperature zero, runtime/model behavior can vary.

If an exercise matters, verify it:

```bash
./localctl verify go-slice-alias-v1 granite
```

Or sample deterministic exercises from a category:

```bash
./localctl verify linux granite --runs=7
```

LocalCTL reports repeatability using deliberately plain labels:

```text
STABLE_PASS
USUALLY_PASS
FLAKY
USUALLY_FAIL
STABLE_FAIL
UNKNOWN
```

Verification stops early after three identical deterministic outcomes when the evidence is already one-sided, rather than mindlessly burning cycles.

This distinction is central to the project:

```text
passed once != reliable
```

---

# Profiles: test models *and* configurations

A model name alone is not a complete experiment.

LocalCTL v0.3 introduces runtime profiles:

```bash
./localctl profiles
```

Built-in profiles currently include:

```text
default       context 2048   temperature 0   max tokens 512
fast          context 2048   temperature 0   max tokens 256
long-context  context 8192   temperature 0   max tokens 1024
```

Run an audition under another profile:

```bash
./localctl audition granite --profile=long-context
```

Create a custom profile:

```bash
./localctl profile create my-4096 --context=4096 --max-tokens=768 --temperature=0
```

Then use it:

```bash
./localctl pack run developer-core granite --profile=my-4096
```

Changing the profile changes the runtime configuration. LocalCTL restarts the same model when necessary rather than silently pretending a different context/configuration is the same experiment.

---

# Versioned capability packs

A benchmark only remains useful historically if you know what test definition produced the result.

List the built-in packs:

```bash
./localctl packs
```

Current v0.3 packs include:

```text
core-baseline/v1
developer-core/v1
linux-investigation/v1
docker-investigation/v1
kubernetes-investigation/v1
writing-summarization/v1
structured-output/v1
reasoning-analysis/v1
```

Inspect one:

```bash
./localctl pack show kubernetes-investigation
```

Every pack declares what it is trying to measure and, just as importantly, what it does **not** prove.

The old category-friendly command still works:

```bash
./localctl suite linux granite
./localctl suite docker granite
./localctl suite kubernetes granite
```

In v0.3 those suite runs are grouped as explicit experiments instead of appearing as an unrelated pile of historical observations.

---

# Missions: start from the question, not the taxonomy

If you do not care what a “pack” is yet, use a mission:

```bash
./localctl missions
```

Then:

```bash
./localctl mission run developer granite
./localctl mission run ops granite
./localctl mission run local-first granite
```

Missions are learner-oriented experiment plans composed from the same versioned packs. They do not create another hidden evaluator.

---

# What LocalCTL learns over time

Every normal model-use surface saves evidence automatically.

These create historical model observations:

```text
localctl audition ...
localctl headtohead ...
localctl pack run ...
localctl suite ...
localctl baseline ...
localctl verify ...
localctl exercise run ...
localctl try ...
localctl work ...
localctl runtime infer ...    # when the runtime is LocalCTL-managed
```

These inspect or organize existing evidence and do not create inference observations by themselves:

```text
localctl lab
localctl models
localctl explore
localctl profiles
localctl packs
localctl experiments
localctl report
localctl gaps
localctl runs
localctl show
localctl compare
localctl insights
localctl evidence audit
```

## Characterize a model

```bash
./localctl report granite
```

The report summarizes:

- total, canonical, and private runs;
- experiment count;
- category coverage;
- deterministic pass/fail observations;
- pending human-review work;
- failure kinds;
- median inference latency;
- median reported generation throughput;
- observed runtime RSS samples where available;
- evidence schema history;
- profiles represented in the corpus.

Machine-readable output is available for the later capability-intelligence layers:

```bash
./localctl report granite --json
```

## Find what we still do not know

```bash
./localctl gaps granite
```

`gaps` is deliberately deterministic in v0.3. It looks for under-tested capability packs and recommends a high-information next experiment based on coverage.

No LLM is interpreting its own test results here.

## Audit the history itself

```bash
./localctl evidence audit
```

A healthy corpus ends with:

```text
history status: COMPLETE
```

The audit verifies run files against the derivative index and, for schema-v3 observations, checks that the provenance fields v0.4/v0.5 will depend on are actually present.

If the NDJSON index is lost or damaged:

```bash
./localctl evidence rebuild-index
```

The index is rebuilt from authoritative observation files. LocalCTL does not rewrite old observations to make them look newer or more complete than they were.

---

# Where is the evidence?

Ask LocalCTL:

```bash
./localctl evidence path
```

By default, runs live under:

```text
~/.localctl/runs/YYYY/MM/DD/<run-id>/
```

A successful run contains:

```text
observation.json
prompt.txt
response.txt
```

A later human judgment is separate:

```text
judgment.json
```

Experiments and sessions are also persisted separately under `~/.localctl/`.

The important idea comes first:

> **The answer can be reinterpreted later. What happened during the run should not be rewritten.**

---

# Evidence schema v3

You do not need to understand this section to use the lab.

For people who do care about provenance, new v0.3 observations can include:

```text
run / session / experiment identity
exercise version
pack ID + version
canonical vs private input class
prompt SHA-256
machine OS / architecture / memory / available chip identity
LocalCTL version + VCS build identity when available
model path / size / SHA-256 / quantization parsed from artifact name
llama-server process / executable / version metadata when observable
runtime profile
requested and observed context identity
inference status / finish reason
token counts when llama.cpp reports them
elapsed time
prompt and generation throughput when reported
response SHA-256
visible-output diagnostics
point-in-time llama-server RSS when the OS exposes it
validation authority
evaluation result and failure classification
```

Unknown values stay unknown.

A point-in-time process RSS sample is **not** labeled peak memory or total Apple unified-memory/Metal usage. The distinction is intentional.

Older v1/v2 observations remain readable historical evidence. LocalCTL does not rewrite them to invent model hashes, failure causes, machine identities, or other facts that were not recorded at the time.

---

# Human judgment stays human

Some tasks can be evaluated mechanically:

```text
exact output
required concepts
semantic JSON equality
```

Other tasks—PR reviews, technical summaries, investigation plans, support writing—are not honestly reducible to one deterministic string comparison.

Those runs are saved as `pending`.

Judge them explicitly:

```bash
./localctl runs
./localctl show <run-id>
./localctl judge <run-id> good "accurate and useful"
./localctl judge <run-id> partial "useful but missed an important edge"
./localctl judge <run-id> bad "confident claim contradicted the evidence"
```

Human judgment is stored separately from the original observation.

```text
observation != judgment
```

---

# Historical compare vs controlled head-to-head

Two comparison modes exist for different purposes.

## Controlled experiment

Prefer this when you want to learn which of two models performs better *now* under one bounded setup:

```bash
./localctl headtohead granite ministral
```

This intentionally shares the same LocalCTL session, machine, pack version, and requested profile.

## Historical roll-up

Use this to look across everything you have previously saved:

```bash
./localctl compare granite ministral
```

Historical evidence may contain different dates, profiles, pack mixes, and older schema versions. It is valuable history, but it is not automatically a controlled benchmark.

That distinction is one of the reasons v0.3 introduces sessions, experiments, profiles, and versioned packs.

---

# Explore newer models

Show the curated local radar:

```bash
./localctl explore
```

Show recently updated GGUF repositories for investigation:

```bash
./localctl explore --live
```

The live surface is discovery, not recommendation:

```text
new != good
GGUF exists != current llama.cpp compatibility proven
file fits on disk != model fits comfortably in memory
model loads != model is useful
one successful task != reliable capability
```

The intended loop is:

```text
discover
  ↓
install one sensible quant
  ↓
localctl models
  ↓
localctl audition <model>
  ↓
verify interesting edges
  ↓
head-to-head against what you already have
  ↓
keep / specialize / reject / investigate further
```

---

# A few terms, in practical language

### GGUF

A model file format used by `llama.cpp`. If you have downloaded models through LM Studio, you may already have GGUF files on your machine.

### llama.cpp

The local inference runtime project doing the low-level model work here.

### llama-server

A server executable built from the llama.cpp project. LocalCTL starts it as a separate OS process and talks to it over HTTP.

### inference

Asking the loaded model to produce an output from an input prompt.

### token / tokens per second

Models read and generate text in tokens rather than directly in words. Generation tokens/second is one useful performance observation, but it is not a quality score.

### context

The amount of tokenized input/history the runtime is configured to make available to a request. Bigger context can require substantially more memory.

### profile

LocalCTL's named runtime/request configuration, such as context size and output limit.

### model artifact

The exact GGUF file, not merely a family name like “Qwen.” v0.3 content-hashes the artifact so later evidence can distinguish files more reliably.

### deterministic evaluation

A machine-checkable rule such as exact output or semantic JSON equality.

### human judgment

An explicit human assessment for work whose quality cannot be honestly reduced to the deterministic evaluator.

---

# Glass-box runtime

The easy workflow does not hide the actual system.

```text
shell / learner
      │
      ▼
localctl Go process
      │
      │ process control + HTTP
      ▼
llama-server process
      │
      ▼
llama.cpp runtime
      │
      ▼
Metal / Apple Silicon
      │
      ▼
GGUF model data
```

Important distinctions:

```text
localctl process != llama-server process
process started != runtime ready
runtime ready != inference succeeded
inference succeeded != answer correct
correct answer != contract followed
contract followed once != reliable
observation != qualification
qualification != selection
selection != authority
capability != authority
```

The low-level controls remain available:

```bash
./localctl runtime start granite
./localctl runtime status
./localctl runtime inspect
./localctl runtime infer "What is 2 + 2?"
./localctl runtime stop
```

Learner-facing commands automatically reuse, restart, switch, or reconcile LocalCTL-managed runtime state as needed. The manual runtime commands remain available when process/runtime behavior itself is what you are trying to learn.

LocalCTL refuses to silently claim ownership of an already-responsive runtime that it did not start/manage.

---

# Command map

## Start here

```text
localctl check
localctl models
localctl lab
localctl audition <model> [--profile=...]
localctl headtohead <model-a> <model-b> [--pack=...] [--profile=...]
```

## Capability work

```text
localctl packs
localctl pack show <pack>
localctl pack run <pack> <model> [--profile=...]
localctl suite <category> [model] [--profile=...]
localctl baseline [model] [--all] [--category=...] [--profile=...]
localctl verify <exercise-or-category> <model> [--runs=N] [--profile=...]
```

## Learner missions

```text
localctl missions
localctl mission show <mission>
localctl mission run <mission> <model> [--profile=...]
```

## Real work

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
localctl profile create <name> --context=N --max-tokens=N --temperature=N
```

## Evidence

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
localctl evidence path
```

## Manual experiments

```text
localctl experiment start <name> [model] [--profile=...] [--pack=...]
localctl experiment status
localctl experiment finish
localctl experiment list
```

## Model discovery

```text
localctl explore
localctl explore --live
```

## Glass-box runtime

```text
localctl runtime start [model] [--profile=...]
localctl runtime stop
localctl runtime status
localctl runtime inspect
localctl runtime infer <prompt>
```

---

# Current evaluation boundaries

LocalCTL v0.3 deliberately prefers deterministic evaluation where a bounded machine rule exists and explicit human judgment where it does not.

Built-in evaluator modes currently include:

```text
exact
contains_all
json_exact
manual
```

The first three are machine-evaluated. `manual` stays pending until a human judges it.

LocalCTL does **not** execute arbitrary model-generated programs on your host simply to get a prettier benchmark score. Generated-code execution needs an explicit isolation/authority boundary before it belongs in the product.

---

# Tests and development

Run the full Go test suite:

```bash
go test -count=1 ./...
```

Build:

```bash
go build -o localctl .
```

GitHub CI enforces:

```text
gofmt-clean source
full Go test suite on Linux
Darwin/arm64 cross-compile
```

The Darwin compile gate exists because Go treats filename suffixes such as `_linux.go` as build constraints. LocalCTL already learned that lesson the hard way; CI now explicitly protects the Apple Silicon target.

Real `llama-server` integration tests remain opt-in because they require the runtime and models:

```bash
LOCALCTL_INTEGRATION=1 go test -count=1 -v -run '^TestLlamaServer'
```

The project intentionally remains dependency-light and easy to inspect while learning Go and systems boundaries.

---

# Current limitations

LocalCTL v0.3 is a capability laboratory, not a finished qualification or routing system.

Current boundaries include:

- installed model discovery is currently centered on `~/.lmstudio/models`;
- the development runtime path is the Homebrew `llama-server` path;
- the managed runtime URL is currently `127.0.0.1:8080`;
- runtime profiles control context/temperature/output limits but do not yet expose every llama.cpp option;
- point-in-time process RSS is useful evidence but is not peak memory or complete Metal/unified-memory accounting;
- time-to-first-token is not yet separately captured;
- inference timeout/cancellation still needs a deliberate policy comparable to the bounded health check;
- human judgments are explicit but subjective by design;
- `gaps` measures evidence coverage, not capability quality;
- historical `compare` can include mixed experiments/configurations; use `headtohead` for a controlled current comparison;
- live model discovery is a candidate radar, not a compatibility checker;
- the built-in packs are practical LocalCTL workloads, not a universal benchmark suite;
- LocalCTL does not currently perform qualification, routing, authorization, or autonomous remediation;
- LocalCTL does not allow an LLM to rewrite observations or grade itself into authority.

These are visible boundaries, not missing caveats.

---

# Where LocalCTL is headed

v0.3 is about **making trustworthy evidence easy to produce**.

The next planned layers intentionally depend on the evidence v0.3 creates rather than guessing ahead of it:

```text
v0.3 — Find the Edges

experiment
measure
repeat
retain provenance
learn what is still unknown
        │
        ▼
v0.4 — Know What To Use

derive capability state
evidence strength
freshness
model roles
requalification
        │
        ▼
v0.5 — Put Local Models To Work

allow qualified local models to analyze bounded evidence
cluster failures
audit conclusions
propose experiments
draft useful work
identify build opportunities
```

The future analyst boundary matters:

```text
AI analysis != observation
AI hypothesis != fact
AI recommendation != qualification
AI proposal != authority
```

If v0.3 and v0.4 do their jobs, v0.5 can dogfood LocalCTL honestly: local models can help analyze the evidence and propose what to test or build next, while LocalCTL remains responsible for provenance, deterministic checks, and explicit human decisions.

See [`docs/LAB.md`](docs/LAB.md) for the original learner workflow, [`docs/CAPABILITY_LAB.md`](docs/CAPABILITY_LAB.md) for the expanded scenario system, and [`docs/PSEUDOCODE.md`](docs/PSEUDOCODE.md) for the longer systems-learning map.

---

# The rule

```text
Don't merely run local models.

Make their behavior measurable enough that you know
when they are actually worth using.
```
