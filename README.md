# localctl

`localctl` is a small Go CLI for learning how local LLM runtimes actually behave and for turning everyday local-model work into durable, comparable evidence.

It is deliberately a **glass box**: an easy learner/capability-lab workflow sits on top of explicit OS-process, HTTP, llama.cpp, GGUF, runtime-state, evaluation, and evidence boundaries that can still be inspected directly.

The project is not trying to become another chat UI, agent framework, model marketplace, or opaque “best model” router.

Its core questions are:

```text
What actually exists?
What is actually running?
What configuration is actually active?
What happened during inference?
Did the model satisfy a bounded task?
Was a failure wrong reasoning, bad contract following, empty output, or runtime failure?
What evidence has accumulated over time?
Where is this local model actually useful?
```

## Architecture

```text
learner / shell
      |
      +-----------------------------------+
      |                                   |
      v                                   v
LocalCTL capability lab            LocalCTL runtime controls
check / models / explore           start / stop
exercises / suite / baseline       status / inspect / infer
runs / compare / insights          glass-box diagnostics
judge / evidence audit
      |                                   |
      +----------------+------------------+
                       |
                       v
                Go localctl process
                       |
                       | process control + HTTP
                       v
                llama-server process
                       |
                       v
                   llama.cpp
                       |
                       v
                  GGUF model
                       |
                       v
             Metal / Apple Silicon

Every managed inference / lab scenario
                       |
                       v
               schema-versioned evidence
                 ~/.localctl/runs/
```

Important boundaries:

```text
localctl process != llama-server process
process started != runtime ready
runtime ready != inference succeeded
inference succeeded != answer correct
correct answer != output contract followed
observation != qualification
qualification != selection
selection != authority
```

## Quick start

Build:

```bash
go build
```

Check the machine and runtime environment:

```bash
./localctl check
```

Discover installed GGUF models under LM Studio's model directory:

```bash
./localctl models
```

See the fast 18-exercise core baseline:

```bash
./localctl exercises
./localctl baseline granite
```

Inspect and run one exercise:

```bash
./localctl exercise show go-slice-alias-v1
./localctl exercise run go-slice-alias-v1 granite
```

Run deeper real-world capability suites:

```bash
./localctl suite developer granite
./localctl suite summarization granite
./localctl suite linux granite
./localctl suite docker granite
./localctl suite kubernetes granite
```

The infrastructure investigation pack contains exactly:

```text
Linux        34
Docker       33
Kubernetes   33
----------------
Total       100
```

Explore newer candidate models for an M1 Pro / 16 GB machine class:

```bash
./localctl explore
./localctl explore --live
```

Audit the historical evidence corpus and inspect what has been learned:

```bash
./localctl evidence audit
./localctl insights granite
```

Then compare accumulated evidence:

```bash
./localctl baseline ministral
./localctl compare granite ministral
```

See [`docs/LAB.md`](docs/LAB.md) for the original learner workflow, [`docs/CAPABILITY_LAB.md`](docs/CAPABILITY_LAB.md) for the broad real-world lab, and [`docs/PSEUDOCODE.md`](docs/PSEUDOCODE.md) for the directional systems/learning map.

## Learner-facing commands

```text
localctl check
localctl models
localctl explore [--live]
localctl exercises [category] [--all]
localctl exercise show <exercise-id>
localctl exercise run <exercise-id> [model]
localctl suite <category> [model]
localctl try <model> <prompt>
localctl baseline [model] [--all] [--category=...]
localctl runs
localctl show <run-id>
localctl judge <run-id> <good|partial|bad> [reason]
localctl compare <model-a> <model-b>
localctl insights [model]
localctl evidence <audit|rebuild-index|path>
```

The lab commands own repeated runtime setup for the learner. The active model stays warm and is reused until another model is requested; switching models is automatic. Low-level lifecycle commands remain available for deliberate systems experiments.

A freeform prompt also becomes a saved run:

```bash
./localctl try granite "Explain why process started is not the same as server ready."
```

## Glass-box runtime commands

```text
localctl runtime start [model]
localctl runtime stop
localctl runtime status
localctl runtime inspect
localctl runtime infer <prompt>
```

`runtime start` launches `llama-server`, records managed state, waits for `/health`, and returns while the separate server process remains alive.

State records information such as:

```text
PID
llama-server executable
model artifact path
model ID
runtime URL
start time
```

`runtime stop` reads that state, verifies process identity, sends `SIGTERM`, observes disappearance, and removes managed state.

LocalCTL refuses to claim ownership of an already-responsive runtime for which it has no managed state.

### Status

```bash
./localctl runtime status
```

uses `GET /health` with an explicit one-second timeout.

### Inspection

```bash
./localctl runtime inspect
```

uses `GET /v1/models` and reports observed model/runtime facts supplied by llama.cpp, including model ID, context, training context, parameter count, and model size when present.

### Inference

```bash
./localctl runtime infer "What is 2 + 2? Answer briefly."
```

uses `POST /v1/chat/completions`.

When this command targets a LocalCTL-managed runtime, it now saves the inference as historical evidence too. Fake/test endpoints remain side-effect free.

## Capability catalog

The fast baseline remains 18 short deterministic exercises so it can be run repeatedly without becoming a multi-hour job.

The extended catalog probes where local models are useful in real work, including:

```text
instruction following
structured JSON and API-style contracts
Go semantics, debugging, concurrency, and process boundaries
Python / JavaScript / TypeScript / Elixir semantics
reasoning and resource constraints
fact extraction and data transformation
source grounding and uncertainty
operations and failure-boundary classification
security judgment
context retrieval under distractors
PR review
commit-message generation
PR descriptions
diff summarization
bug triage
regression-test planning
incident and technical summarization
support responses
release notes
bounded planning
Linux investigation
Docker investigation
Kubernetes investigation
```

List everything:

```bash
./localctl exercises --all
```

List a category:

```bash
./localctl exercises linux --all
./localctl exercises developer --all
```

Run a complete category with the learner-friendly suite command:

```bash
./localctl suite linux granite
./localctl suite developer granite
```

## Evaluation modes and failure intelligence

LocalCTL uses four explicit evaluation modes:

```text
exact
contains_all
json_exact
manual
```

`exact`, `contains_all`, and `json_exact` produce deterministic pass/fail evidence.

`manual` produces `pending` rather than manufacturing a correctness score for subjective work such as PR reviews, summaries, investigation plans, and support writing.

For new schema-v2 runs, deterministic failures are further classified where possible:

```text
incorrect_answer
contract_extra_output
empty_output
invalid_json
structured_mismatch
missing_required_concepts
inference_error
```

That makes an important distinction visible:

```text
expected: NO
model:    NO + a long explanation
```

may be correct reasoning but is still an exact-output contract failure.

Likewise:

```text
wrong answer
!=
right answer with forbidden extra prose
!=
empty visible answer
!=
runtime/inference failure
```

Older schema-v1 runs remain historical evidence. Their old observations are not rewritten to invent a failure subtype that was not recorded at the time.

## Durable evidence

Evidence collection is the default for learner/lab runs and direct inference against a LocalCTL-managed runtime.

Runs are stored under:

```text
~/.localctl/runs/YYYY/MM/DD/<run-id>/
```

A successful run contains:

```text
observation.json
prompt.txt
response.txt
```

A later human judgment is stored separately:

```text
judgment.json
```

The derivative index lives at:

```text
~/.localctl/runs/index.ndjson
```

Schema-v2 observations capture facts including:

```text
run ID and timestamps
exercise ID/category/difficulty
prompt SHA-256
model artifact ID/name/path/size
artifact metadata key (path + size identity, not a content digest)
runtime kind / URL / managed PID / executable / runtime start time
requested context / temperature / max tokens
inference status
finish reason
token usage when reported
elapsed time
prompt/generation throughput when reported
response SHA-256
visible character count
empty-visible-output marker
deterministic evaluation result
failure kind when classified
```

Unknown measurements are not invented.

The project does **not** yet claim that `artifact_metadata_key` is a cryptographic model-artifact digest. Complete model content digesting, runtime-build identity, machine fingerprinting, and additional configuration identity remain future evidence improvements.

Human judgment remains separate from machine observation, so changing an opinion never rewrites what happened.

## Evidence audit and repair

Check the evidence corpus itself:

```bash
./localctl evidence audit
```

The audit compares authoritative run directories with the NDJSON index and reports:

```text
observation files
index entries
unindexed observations
duplicate index rows
missing prompt artifacts
missing response artifacts
human judgments
schema-version history
runs by model
```

If the derivative index is damaged or missing, rebuild it from `observation.json` files:

```bash
./localctl evidence rebuild-index
```

This does not rewrite the historical observation files.

See the evidence root:

```bash
./localctl evidence path
```

## Historical intelligence

List and inspect runs:

```bash
./localctl runs
./localctl show <run-id>
```

Attach a human judgment:

```bash
./localctl judge <run-id> good "accurate and useful"
./localctl judge <run-id> partial "useful but missed an important edge"
./localctl judge <run-id> bad "confident diagnosis contradicted the evidence"
```

Summarize accumulated history:

```bash
./localctl insights
./localctl insights granite
```

Insights currently report:

```text
run counts
successful inference / errors
auto pass / fail
manual pending
empty visible output
median elapsed time
median generation throughput
pass rate by category
human judgments
failure-mode counts
evidence schema mix
```

Compare two installed models from saved observations:

```bash
./localctl compare granite ministral
```

Comparison intentionally reports bounded observations rather than declaring a universal winner.

## Model exploration

Installed model discovery remains local:

```bash
./localctl models
```

The curated M1 Pro / 16 GB radar is:

```bash
./localctl explore
```

It currently highlights a small investigation queue spanning:

```text
priority candidates
speed/control candidate
larger stretch candidates
```

The list is deliberately conservative about memory claims: published GGUF file size is not total runtime memory use, and context/KV cache/runtime overhead still require headroom.

A live discovery surface is also available:

```bash
./localctl explore --live
```

It queries recently updated Hugging Face GGUF repositories and applies a conservative name-based 3B–14B filter.

The live view is **discovery only**:

```text
recent != good
repository naming != parameter truth
GGUF exists != current llama.cpp supports it
model-file size != runtime memory use
loads once != useful
useful on one workload != universal capability
```

The intended flow is discovery -> install -> observe -> test -> judge -> compare.

## Finding the edge of usefulness

A useful local model is not simply the model with the largest aggregate pass rate.

Look for a workload-specific operating envelope:

```text
Correctness
Contract reliability
Grounding
Investigation quality
Compression/summarization
Developer utility
Latency
Generation throughput
Repeatability
Resource fit
```

A good conclusion is:

```text
On this machine, with this artifact/runtime/configuration,
Model X has demonstrated strong evidence for workload Y
and weak evidence for workload Z.
```

Not:

```text
Model X is best.
```

## Relationship to Invariant

LocalCTL remains an independent learning/runtime project.

It does not currently implement Bench, Manifold, or Kiln.

A useful conceptual boundary remains:

```text
LocalCTL-like runtime/evidence layer
    What exists?
    What is running?
    What configuration was observed?
    What happened during the run?
    What bounded task behavior was demonstrated?
        |
        v
      evidence
       /   \
      v     v
   Bench  Manifold
 qualify   select
       \   /
        v v
        Kiln
      authority
```

The responsibilities remain distinct:

```text
observation != qualification
qualification != selection
selection != execution
selection != authorization
capability != authority
```

The future Elixir layer should therefore be designed from the evidence we accumulate, not from guessed abstractions.

## Source structure

The implementation remains intentionally flat enough to follow while learning:

```text
main.go
cli.go
runtime.go
runtime_process.go
model.go
model_explore.go
exercise.go
exercise_catalog.go
exercise_catalog_expanded.go
exercise_catalog_devwork.go
exercise_catalog_linux.go
exercise_catalog_docker.go
exercise_catalog_kubernetes.go
evidence.go
evidence_cli.go
insights.go
lab_cli.go
suite_cli.go
*_test.go
```

No third-party Go dependencies are currently required.

## Tests

Run the normal suite:

```bash
go test -count=1 ./...
```

GitHub CI enforces:

```text
gofmt-clean Go source
full Go test suite
```

Catalog tests protect the fast 18-exercise baseline, uniqueness of exercise IDs, broad category coverage, and the exact 34 Linux + 33 Docker + 33 Kubernetes infrastructure scenario count.

The real `llama-server` integration tests remain opt-in because they require the local runtime/model environment:

```bash
LOCALCTL_INTEGRATION=1 go test -count=1 -v -run '^TestLlamaServer'
```

## Current limitations

The capability lab is useful now, but it is not a qualification system.

Current limitations include:

- installed-model discovery currently targets `~/.lmstudio/models`;
- `llama-server` executable path is currently the Homebrew path used during development;
- runtime URL/port and context remain mostly fixed defaults;
- complete artifact content digest, runtime build identity, and machine fingerprint are not yet recorded;
- inference timeout/cancellation policy is not yet deliberately bounded like status;
- memory/resource-pressure measurement is not yet captured;
- human judgments are stored and summarized but not yet fully integrated into every comparison aggregate;
- comparison does not yet detect every experimental confounder;
- live model discovery uses conservative repository-name heuristics and is not a compatibility checker;
- built-in scenarios are a growing practical measurement surface, not a universal benchmark;
- LocalCTL does not perform qualification, routing, authorization, or model-selection policy.

These are explicit boundaries rather than hidden claims.

## Development principle

The project continues to use:

```text
mental model
    -> property
    -> prediction
    -> experiment
    -> evidence
    -> comparison
    -> updated mental model
    -> minimum implementation
```

The unit of progress is:

```text
property understood
+ property demonstrated
+ evidence retained
```

Ordinary use now contributes to that evidence base automatically.

## Next direction

Use the lab against multiple models and configurations. Run the quick baseline, then the workload suites that correspond to actual local-model use: developer work, summarization, Linux/Docker/Kubernetes investigation, structured output, and other repeated tasks.

Inspect failures. Judge subjective runs. Re-run important workloads. Audit the historical corpus. Use `insights` to find emerging patterns.

Only after that evidence exists should the higher-level Elixir coordination/knowledge layer be designed around demonstrated needs such as longitudinal state, freshness, experiment scheduling, requalification, and capability reasoning.
