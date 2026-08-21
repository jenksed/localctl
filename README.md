# localctl

`localctl` is a small Go CLI for learning how local LLM runtimes actually behave and for turning everyday local-model experiments into durable, comparable evidence.

It is deliberately a **glass box**: the easy learner workflow sits on top of explicit OS-process, HTTP, llama.cpp, GGUF, and runtime-state boundaries that can still be inspected directly.

The project is not trying to become another chat UI, agent framework, model marketplace, or opaque “best model” router.

Its core questions are:

```text
What actually exists?
What is actually running?
What configuration is actually active?
What happened during inference?
Did the model satisfy a bounded task?
What evidence has accumulated over time?
```

## Architecture

```text
learner / shell
      |
      +-----------------------------+
      |                             |
      v                             v
LocalCTL lab commands        LocalCTL runtime commands
check / models               start / stop
exercises / baseline         status / inspect / infer
runs / compare               glass-box diagnostics
      |                             |
      +-------------+---------------+
                    |
                    v
             Go localctl process
                    |
                    | start/stop + HTTP
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

Every lab run
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

Discover GGUF models under LM Studio's model directory:

```bash
./localctl models
```

See the core learner exercises:

```bash
./localctl exercises
```

Inspect one exercise before running it:

```bash
./localctl exercise show go-slice-alias-v1
```

Run one exercise against a model using an unambiguous model reference:

```bash
./localctl exercise run go-slice-alias-v1 granite
```

Run the core baseline suite:

```bash
./localctl baseline granite
```

Run the extended catalog:

```bash
./localctl baseline granite --all
```

Run only one category:

```bash
./localctl baseline granite --category=coding
```

Then repeat with another model and compare saved evidence:

```bash
./localctl baseline ministral
./localctl compare granite ministral
```

See [`docs/LAB.md`](docs/LAB.md) for the learner workflow and [`docs/PSEUDOCODE.md`](docs/PSEUDOCODE.md) for the directional systems/learning map.

## Learner-facing commands

```text
localctl check
localctl models
localctl exercises [category] [--all]
localctl exercise show <exercise-id>
localctl exercise run <exercise-id> [model]
localctl try <model> <prompt>
localctl baseline [model] [--all] [--category=...]
localctl runs
localctl show <run-id>
localctl judge <run-id> <good|partial|bad> [reason]
localctl compare <model-a> <model-b>
```

The lab commands manage repeated runtime setup for the learner while preserving the underlying evidence.

A freeform prompt also becomes a saved run:

```bash
./localctl try granite "Explain why process started is not the same as server ready."
```

## Glass-box runtime commands

The lower-level runtime surface remains available:

```text
localctl runtime start [model]
localctl runtime stop
localctl runtime status
localctl runtime inspect
localctl runtime infer <prompt>
```

### Managed lifecycle

`runtime start` launches `llama-server`, records enough state to identify the managed process later, waits for `/health`, and then returns control to the shell while the child continues running.

State currently records information such as:

```text
PID
llama-server executable
model artifact path
model ID
runtime URL
start time
```

`runtime stop` reads that state, checks that the PID exists, verifies the process command line still matches the recorded executable and model, sends `SIGTERM`, observes shutdown, and removes the state file.

LocalCTL refuses to claim ownership of an already-responsive runtime for which it has no managed state.

### Status

```bash
./localctl runtime status
```

uses:

```text
GET /health
```

with an explicit one-second status timeout.

This demonstrated:

```text
unreachable != reachable but stalled
```

### Inspection

```bash
./localctl runtime inspect
```

uses:

```text
GET /v1/models
```

and reports observed model/runtime facts such as runtime owner, model ID, context, training context, parameter count, and artifact size when supplied by llama.cpp.

### Inference

```bash
./localctl runtime infer "What is 2 + 2? Answer briefly."
```

uses:

```text
POST /v1/chat/completions
```

LocalCTL retains the completion `finish_reason`. Lab runs additionally capture token usage, elapsed time, and llama.cpp timing fields when the installed runtime reports them.

## Baseline exercise lab

The built-in catalog intentionally mixes different classes of work instead of pretending one score describes a model.

Current exercise areas include:

```text
instruction following
structured JSON output
Go execution reasoning
Go compiler/error diagnosis
Go concurrency reasoning
code and systems review
shell reasoning
SQL generation
arithmetic and constraint reasoning
fact extraction
classification
source grounding
contradiction detection
failure classification
uncertainty discipline
summarization
rewriting
technical explanation
bounded planning
```

The core baseline favors short, repeatable exercises. The extended catalog adds harder or human-judged tasks.

### Evaluation modes

LocalCTL currently has four explicit evaluation modes:

```text
exact
contains_all
json_exact
manual
```

The first three produce deterministic pass/fail evidence.

`manual` produces `pending` rather than allowing LocalCTL to manufacture a correctness score for a subjective task.

That distinction is intentional:

```text
fluent answer != correct answer
```

## Durable evidence

Evidence collection is the default for learner/lab runs.

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

An append-only index is maintained at:

```text
~/.localctl/runs/index.ndjson
```

The observation schema currently captures facts including:

```text
run ID and timestamps
exercise ID/category/difficulty
model artifact ID/name/path/size
runtime kind and URL
requested context/temperature/max tokens
inference status
finish reason
token usage when reported
elapsed time
prompt/generation throughput when reported
deterministic evaluation result
```

Unknown measurements are not invented.

Human judgment is deliberately separate from machine observation, so a later opinion never rewrites what actually happened.

## History and comparison

List recent runs:

```bash
./localctl runs
```

Inspect a run:

```bash
./localctl show <run-id>
```

Attach human judgment:

```bash
./localctl judge <run-id> good "accurate and useful"
./localctl judge <run-id> partial "correct result but missed an important edge"
./localctl judge <run-id> bad "plausible diagnosis contradicted the source"
```

Compare models from accumulated saved observations:

```bash
./localctl compare granite ministral
```

The comparison currently reports evidence such as:

```text
run counts
auto-scored counts
auto-scored pass rate
manual tasks awaiting judgment
median elapsed time
median generation throughput when available
auto-scored pass rates by category
```

It intentionally does **not** claim a universal winner.

A model can be faster but less reliable for one workload, or stronger on structured output and weaker on coding. The point is to accumulate enough comparable evidence to make bounded, practical decisions.

## Model/configuration identity

A model name alone is not a useful capability identity.

A meaningful future identity is closer to:

```text
model
+ artifact
+ artifact digest
+ quantization
+ runtime/build
+ machine
+ context configuration
+ offload configuration
+ sampling configuration
+ prompt/contract
+ workload class
```

LocalCTL already records part of this identity and will tighten it as experiments show which facts materially affect comparison.

The desired future claim is not:

```text
"Model X is good at coding."
```

It is closer to:

```text
this exact model/runtime/configuration
on this machine
demonstrated this bounded result
under this workload
within this observed operating envelope
```

## Relationship to Invariant

LocalCTL remains an independent learning/runtime project.

It does not currently implement Bench, Manifold, or Kiln.

A useful conceptual boundary is:

```text
LocalCTL-like runtime layer
    What exists?
    What is running?
    What configuration was observed?
    What happened during the run?
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

Manifold is therefore a reference architecture and possible future consumer of lessons from LocalCTL, not a dependency of this Go implementation.

## Source structure

The implementation is still intentionally flat enough to follow while learning:

```text
main.go
  process entry/exit

cli.go
  command dispatch

runtime.go
  llama-server HTTP protocol and inference

runtime_process.go
  managed llama-server lifecycle and state

model.go
  GGUF discovery and learner-friendly model references

exercise.go
  exercise/evaluation contracts

exercise_catalog.go
  built-in baseline and extended exercise catalog

evidence.go
  immutable observations, history index, human judgments

lab_cli.go
  learner-facing lab workflows and comparisons

*_test.go
  behavior/property tests
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

The real `llama-server` integration tests remain opt-in because they require the local runtime/model environment:

```bash
LOCALCTL_INTEGRATION=1 go test -count=1 -v -run '^TestLlamaServer'
```

## Current limitations

The lab is useful now, but it is not a qualification system.

Current limitations include:

- automatic model discovery currently targets `~/.lmstudio/models`;
- the `llama-server` executable path is currently the Homebrew path used during development;
- runtime URL/port and context are still mostly fixed defaults;
- complete artifact digest, runtime build identity, and machine fingerprint are not yet recorded;
- inference timeout/cancellation policy is not yet deliberately bounded like status;
- memory/resource-pressure measurement is not yet captured;
- human judgments are stored but not yet folded into `compare` aggregates;
- comparison assumes the learner ran reasonably comparable exercise sets and does not yet detect every confounder;
- the built-in catalog is a baseline measurement surface, not a universal benchmark;
- LocalCTL does not perform qualification, routing, authorization, or model selection policy.

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

The unit of progress is not merely “feature completed.” It is:

```text
property understood
+ property demonstrated
+ evidence retained
```

The lab is designed so ordinary use now contributes to that evidence base automatically.

## Next direction

The immediate next step is not a larger Go architecture.

Use the lab against multiple local models and configurations, inspect failures, judge subjective runs, and accumulate comparable evidence. That evidence should tell us which identity fields, workloads, resource measurements, and comparison rules actually matter.

Only after that evidence exists should the next higher-level layer—potentially an Elixir coordination/knowledge layer—be designed around real needs rather than guessed abstractions.
