# localctl

`localctl` is a hands-on systems project for understanding, measuring, and eventually operating local LLM inference for real software work.

The immediate implementation is intentionally small:

```text
shell
  ↓
Go executable (`localctl`)
  ↓
HTTP
  ↓
`llama-server`
  ↓
`llama.cpp`
  ↓
Metal / Apple Silicon
  ↓
GGUF model
```

The long-term goal is larger: build a local-model operating layer that can discover what models and configurations are actually useful on a machine, preserve the evidence that supports those claims, route bounded work according to demonstrated capability, manage constrained local compute, and escalate work when local inference has not demonstrated that it can do the job reliably.

This repository starts at the bottom of that stack on purpose. It is not an attempt to build the entire control plane in Go.

For the implementation-shaped project map, see [`docs/PSEUDOCODE.md`](docs/PSEUDOCODE.md).

## Project thesis

Local models are already capable of useful software work. The practical problem is that using them well remains operationally messy.

A model may be good at reviewing a diff but poor at planning a multi-file change. Another may be fast and predictable with structured output but weak at tool use. A configuration that works well with one runtime, quantization, context size, or chat template may behave differently under another. A smaller model may be the correct choice for a narrow task because it is fast, fits in memory, and reliably follows the required output contract.

Today the developer usually remembers these distinctions manually.

The system should remember them instead.

The eventual user-facing abstraction should move from:

```text
Use model X.
```

toward:

```text
I need:
- capability: diff_review
- structured output: required
- latency: under 5 seconds
- memory: under 6 GB
```

The system can then choose among configurations that have actually demonstrated the requested property.

The project is therefore not primarily a model leaderboard. It is an attempt to construct usable engineering knowledge about local inference.

## What is being qualified

A model name alone is not enough information.

The relevant execution identity is closer to:

```text
model
+
runtime
+
quantization
+
context configuration
+
prompt structure
+
reasoning behavior
+
sampling
+
available tools
+
input preparation
+
output contract
```

A qualification statement should eventually mean something like:

```text
Capability:
fast_test_diagnosis

Configuration:
model: qwen-family-model
runtime: llama.cpp
quantization: Q4 variant
context: 8192
profile: fast-diagnosis-v2

Evidence:
20 fixture runs
18 correct
20 structurally valid
median latency: measured value

Machine:
Apple Silicon host with recorded environment facts

Decision:
qualified under explicit criteria
```

That is materially different from saying that a model is "good at coding."

## Why Go

Go is being used here deliberately, not because the final system must be a Go application.

The first phase is a machine-facing CLI and diagnostic tool. That gives the project a legitimate reason to learn and use:

- executable/process behavior
- CLI design
- `net/http`
- `context.Context`
- timeouts and cancellation
- `os/exec`
- signals and process lifecycle
- files and durable evidence
- JSON boundaries
- concurrency
- resource observation
- testing around external processes
- single-binary distribution

The Go component should remain useful after the larger architecture grows.

The current working name is `localctl`.

Early:

```text
localctl
   ↓
llama-server
   ↓
llama.cpp
```

Later:

```text
localctl
   ↓
control plane
   ↓
runtime adapter
   ↓
llama.cpp
```

The direct-runtime path can remain useful for diagnostics even after a control plane exists.

## Why `llama.cpp` first

The first runtime is `llama.cpp`, exposed through `llama-server` over HTTP.

We are intentionally not starting with a managed desktop application because the point of this phase is to understand more of the runtime boundary directly: model files, server startup, readiness, configuration, context, runtime flags, latency, process lifecycle, structured output, and eventually concurrency/resource behavior.

We are also intentionally not starting by embedding the native `llama.cpp` API through cgo.

The first boundary is:

```text
Go
 ↓ HTTP
llama-server
 ↓
llama.cpp
```

not:

```text
Go
 ↓ cgo
llama.cpp C/C++ API
```

Native integration is a future experiment only if evidence shows that the server boundary is materially limiting the system.

Current upstream `llama-server` exposes a lightweight HTTP server with OpenAI-compatible routes and native monitoring/health surfaces. The exact API behavior used by `localctl` must still be verified against the installed runtime version rather than assumed forever from documentation.

Useful upstream references:

- https://github.com/ggml-org/llama.cpp
- https://github.com/ggml-org/llama.cpp/tree/master/tools/server

## The mental model

The first implementation should be understood as a chain of responsibility.

### Shell

The shell:

- resolves the executable path
- provides command-line arguments and environment variables
- creates the `localctl` process
- observes its exit status
- may redirect stdin/stdout/stderr

The shell does not own inference.

### `localctl`

The Go process initially owns:

- CLI argument interpretation
- configuration resolution
- HTTP requests to `llama-server`
- request deadlines/cancellation
- parsing runtime responses
- local validation of response contracts
- local evidence capture
- eventually child-process lifecycle when we deliberately add it

It does **not** initially own:

- token generation
- model loading internals
- KV-cache implementation
- Metal kernels
- tensor execution
- arbitrary authority to execute model-generated commands

### `llama-server`

`llama-server` owns the serving process and the runtime-facing HTTP boundary. Depending on how it is launched, it owns configuration such as the model path, context, listening address, runtime options, batching/parallel behavior, and other server/runtime flags.

A healthy HTTP process is not automatically proof that the loaded model is useful for a requested task.

### `llama.cpp`

`llama.cpp` owns the inference machinery beneath the server: loading supported model data, tokenization/runtime execution, sampling, compute/backend interaction, cache behavior, and related inference mechanisms.

### Metal / hardware

On Apple Silicon, Metal is part of the hardware acceleration path used by the runtime. It is below the application boundary we are initially implementing.

### GGUF model

A GGUF file is an artifact consumed by the runtime. Its presence on disk proves only that a file exists.

```text
file exists
≠
valid model artifact
≠
loadable by this runtime
≠
responsive
≠
contract compliant
≠
capable of the task
≠
operationally acceptable
```

That distinction is foundational to the project.

## Core engineering rules

### Installed is not qualified

Never treat discovery as proof of capability.

The intended progression is:

```text
present
→ loadable
→ ready
→ responsive
→ contract-compliant
→ task-capable
→ operationally acceptable
→ qualified under explicit criteria
```

Each arrow requires evidence.

### Model output is untrusted input

A model response is data, not authority.

A model saying:

```text
I fixed the bug.
```

proves nothing about the repository.

Future coding tasks may produce patches, commands, diagnoses, or tool calls, but the system must keep proposal separate from authority and verification.

### Evidence and qualification are different

A measurement is a fact about an observed run.

Example:

```text
18 of 20 fixture runs satisfied the required property.
```

A qualification decision is a judgment based on explicit criteria.

Example:

```text
This configuration is qualified for fast_test_diagnosis.
```

Do not collapse the two.

### Preserve raw evidence

Do not normalize away useful facts too early.

Historical evidence should retain enough information to answer later questions such as:

- What exact runtime produced this result?
- Which model file and profile were used?
- What input was supplied?
- What raw output came back?
- What parser or validator interpreted it?
- What machine/environment facts matter?
- Was the run cold or warm?
- What failed?

Derived summaries can be rebuilt. Lost raw evidence cannot.

### Routing should initially be boring

The eventual system may choose among configurations, but initial routing should use explicit deterministic rules over measured evidence.

Do not introduce another LLM whose job is to guess which LLM should be used.

### Capability is not authority

A model may be capable of producing shell commands, patches, or tool calls. That does not authorize execution.

Early `localctl` should not casually acquire authority to mutate repositories or execute arbitrary model-generated commands.

### Observe before controlling

For resource behavior, concurrency, process lifecycle, model switching, and scheduling:

```text
observe
→ measure
→ model
→ control
```

Do not build a scheduler because one may eventually be useful.

## Learning and development method

This repository is intentionally developed as a conversational learning project rather than a code-generation exercise.

The normal cycle is:

```text
1. Establish the mental model.
2. Identify one property we need.
3. Learn only the Go/runtime concepts required for it.
4. Predict behavior where useful.
5. Implement a small piece by hand.
6. Run a concrete validation command.
7. Inspect the observed result.
8. Debug discrepancies before proceeding.
9. Record what is proven and what remains unproven.
10. Choose the smallest next experiment.
```

The project should resist two failure modes:

1. allowing an AI coding agent to generate the entire repository before the operator understands it;
2. designing future control-plane abstractions before runtime evidence gives them a reason to exist.

## Scope

### In scope for the Go phase

- a real Go CLI
- direct interaction with `llama-server`
- explicit configuration
- runtime health/readiness inspection
- runtime/model metadata inspection
- bounded inference requests
- structured output validation
- evidence capture
- repeatable fixture execution
- configuration profiles
- repeated measurements
- explicit qualification decisions
- resource observation
- controlled concurrency experiments
- deterministic capability selection
- a clean language-neutral boundary for a future control plane

### Explicitly out of scope at the beginning

- building another editor
- building another Claude Code/OpenCode-style harness
- distributed scheduling
- frontier-provider orchestration
- autonomous shell execution
- arbitrary repository mutation by models
- cgo/native integration
- a plugin framework
- premature multi-runtime abstraction
- a database before durable evidence requires one
- an LLM-based router
- pretending synthetic benchmarks prove real coding capability

## Milestone roadmap

The milestone numbers are directional. Reality is allowed to change the plan.

### M0 — Smallest real Go executable

Goal: understand the executable before wrapping any inference runtime.

Learn:

- Go module
- package
- `package main`
- `func main()`
- imports
- `os.Args`
- `go run`
- `go build`
- `go test`
- exit status

Candidate behavior:

```bash
localctl version
```

Must be understood before proceeding:

```text
source
→ compiler
→ executable
→ process
→ exit status
```

### M1 — Manual `llama-server`

Goal: understand the runtime before hiding it behind Go.

Manually establish:

- where `llama-server` comes from
- where a GGUF model lives
- how the server is started
- which configuration is passed via arguments
- which port it listens on
- what loading/readiness looks like
- how it exits

No Go process management yet.

### M2 — Runtime probe

Candidate command:

```bash
localctl runtime status
```

The command should distinguish at least:

- cannot connect
- HTTP reachable
- loading/not ready
- ready
- unexpected response
- timeout/cancellation

Current upstream `llama-server` documents `GET /health` (and `/v1/health`) with `503` while loading and `200` when ready. That behavior should be verified against the actual installed build before becoming an assumed contract.

### M3 — Runtime/model inspection

Inspect actual API responses before inventing an internal model schema.

Questions:

- What model identity does the server expose?
- Which fields are runtime-specific?
- Which facts should be preserved raw?
- What belongs in transport structs versus internal structures?

Current upstream also exposes model-listing routes such as `/v1/models`; installed behavior remains the authority for our implementation.

### M4 — First bounded inference

Send the smallest useful request possible.

Example property:

```text
Return exactly: PONG
```

Capture:

- request
- runtime/model identity
- start/end timestamps
- duration
- raw response
- transport error
- instruction-following result

Distinguish:

```text
transport success
≠
inference success
≠
requested property satisfied
```

### M5 — Process ownership

Only after manual operation is understood should Go start `llama-server`.

Learn:

- `os/exec`
- child-process ownership
- stdout/stderr
- readiness
- PIDs
- signals
- cancellation
- startup failure
- shutdown semantics

Explicit decisions are required for:

- attached versus detached lifetime
- whether server lifetime may exceed CLI lifetime
- startup timeout
- readiness proof
- crash handling
- stale PID/state behavior

### M6 — Structured output contract

Introduce machine-verifiable output.

Example:

```json
{
  "classification": "bug",
  "confidence": 0.8
}
```

Exercise:

- valid JSON
- malformed JSON
- markdown-wrapped JSON
- missing fields
- wrong types
- extra chatter
- empty content

The system must distinguish transport success from contract success.

### M7 — Evidence records

Begin durable evidence capture.

Candidate facts:

- run ID
- timestamp
- `localctl` version
- machine/environment identity
- runtime version
- model/GGUF identity
- runtime/profile configuration
- task/fixture identity
- input
- raw output
- parsed output
- duration
- contract result
- validation result
- error

Storage is intentionally undecided until requirements become real.

Possible progression:

```text
individual JSON / JSONL
→ SQLite only when querying/transactional needs justify it
```

### M8 — Repeatable fixtures

Create small task fixtures representing useful software work.

Candidate categories:

- structured extraction
- error classification
- test-failure diagnosis
- small code reasoning
- diff review
- tool selection

A fixture must identify the property it tests.

Avoid:

```text
produce a good answer
```

Prefer:

```text
response must identify symbol X
response must satisfy schema Y
response must choose one allowed class
response must identify the demonstrated failing boundary
```

### M9 — Repeated measurement

Run the same configuration enough times to measure behavior rather than remember anecdotes.

Potential measurements:

- success count
- failure count
- contract compliance
- latency distribution
- runtime errors
- variance

### M10 — Profiles

Represent reusable inference configurations without rewriting history.

A profile may eventually include observed/controlled settings such as:

- model/GGUF
- context
- runtime flags
- sampling
- chat template behavior
- thread/backend settings where meaningful

Evidence must retain the profile/version that actually produced it.

### M11 — Qualification

Introduce explicit qualification criteria.

A configuration can be qualified for one capability and rejected for another.

Avoid global statements such as "model X is qualified for coding."

### M12 — Resource observation

Measure before scheduling:

- cold startup
- warm inference
- model load/unload time
- memory pressure
- model switching cost
- quantization effects
- context effects

### M13 — Controlled Go concurrency

Only now deliberately explore:

- goroutines
- channels
- `sync.WaitGroup`
- mutexes
- semaphores
- cancellation
- bounded concurrency
- backpressure
- race detection

Experiments should be tied to a real constrained resource.

Example:

```text
10 callers
1 runtime
1 loaded model
different deadlines
one cancellation
one runtime failure
```

Use `go test -race ./...` where applicable.

### M14 — Deterministic capability selection

Once evidence exists, implement a simple explainable selector.

Example request:

```text
capability: diff_review
latency ceiling: 5s
memory ceiling: 6 GB
structured contract: required
```

The selector must be able to explain why candidate A beat candidate B using explicit rules and recorded evidence.

### M15 — Language-neutral control-plane contract

Only after the direct-runtime system teaches us what is actually needed should we specify the external service contract for a larger control plane.

Possible concepts:

- runtime status
- models
- execute
- profiles
- capabilities
- qualification
- evidence
- system status

The contract must not expose Go- or Elixir-specific implementation concepts.

### M16 — Control-plane handoff

The likely future architecture becomes:

```text
localctl
    ↓
Elixir/OTP control plane
    ↓
runtime adapter
    ↓
llama.cpp
```

At that point Elixir may legitimately own:

- long-lived state ownership
- request coordination
- queues
- runtime lifecycle policy
- resource policy
- backpressure
- qualification state
- event streams
- recovery
- routing
- escalation

The Go CLI remains a useful independent client and diagnostic tool.

## Future runtime experiments

`llama.cpp` is the first runtime, not the definition of the system.

Later candidates include:

- LM Studio as a managed-runtime comparison
- an MLX-native path for Apple-specific comparison
- another runtime if it answers a real question

A useful experiment is eventually:

```text
same conceptual model
same task
similar profile

llama.cpp/GGUF
vs
managed runtime
vs
MLX-native
```

This lets the project test whether runtime choice materially changes latency, memory use, contract adherence, behavior, or reliability.

## Native boundary: deliberately deferred

A future native bridge may be justified if measurement proves that:

```text
Go → HTTP → llama-server
```

creates an important limitation.

Only then should we investigate something like:

```text
Go
 ↓
cgo / small C or Zig bridge
 ↓
llama.cpp
```

Native complexity must answer a demonstrated problem.

## Evidence model: conceptual shape

The exact schema is intentionally not committed yet, but the system should preserve distinctions like these:

```text
RunEvidence
├── identity
│   ├── run_id
│   ├── timestamp
│   └── localctl_version
├── machine
│   ├── os
│   ├── arch
│   └── relevant hardware facts
├── runtime
│   ├── name
│   ├── version
│   ├── endpoint/process identity
│   └── runtime configuration
├── model
│   ├── model identity
│   ├── artifact identity
│   ├── quantization if known
│   └── profile identity
├── task
│   ├── fixture/capability identity
│   └── input
├── observation
│   ├── raw response
│   ├── parsed response
│   ├── timing
│   ├── transport result
│   ├── contract result
│   └── validation result
└── error/failure evidence
```

A later qualification record should reference evidence rather than overwrite it.

## Failure model

Failures are part of the curriculum and part of the product.

We should intentionally exercise cases such as:

- `llama-server` unavailable
- invalid model path
- model load failure
- wrong port
- loading never becomes ready
- request timeout
- request cancellation
- malformed/unexpected server response
- malformed model output
- model ignores output contract
- runtime crash
- partial output
- child process receives a signal
- evidence write fails
- stale runtime state
- runtime version changes
- concurrent requests contend

For meaningful failures, ask:

```text
Who detected it?
Who owns recovery?
What state may be stale?
What evidence survived?
Can the operation safely retry?
Could retry duplicate work?
What should the operator see?
```

## Go design policy

The project should teach idiomatic Go rather than recreating another language in Go syntax.

Default preferences:

- plain structs
- explicit errors
- small packages
- small interfaces only where a real behavioral boundary exists
- composition
- `context.Context` across cancellable external operations
- simple constructors when construction invariants exist
- standard library before dependencies
- clear resource/process ownership

Avoid premature architecture such as:

```text
internal/domain/services/repositories/adapters/ports/...
```

before the code demonstrates a need for it.

Do not create interfaces solely so a test can mock something.

## Dependency policy

Start mostly with the Go standard library.

Before adding a dependency, answer:

1. What real problem does it solve?
2. What would the standard-library version require?
3. What coupling and maintenance does the dependency add?
4. Is the trade worth it now?

A CLI framework may eventually be justified. It is not automatically justified for M0.

## Configuration policy

Configuration should remain simple until the system has enough settings to justify more machinery.

When multiple sources exist, precedence must be explicit and tested. A likely shape might eventually be:

```text
CLI flag
→ environment
→ config file
→ default
```

but this is not committed until a real configuration requirement appears.

Important runtime configuration must never be silently guessed when that guess would make evidence ambiguous.

## Testing doctrine

A passing test is not automatically proof of the intended property.

For consequential tests, identify:

```text
claimed property
mechanism exercised
evidence produced
remaining gap
```

Useful tools may include:

- `go test ./...`
- `httptest`
- integration tests against a controlled `llama-server`
- fixtures
- failure injection
- repeated runs
- `go test -race ./...`

Do not maximize test count. Test the property at the boundary where it can actually fail.

## Repository structure

Do not create this entire structure immediately. It is a directional map, not a scaffolding instruction.

A plausible mature shape might become:

```text
localctl/
├── README.md
├── go.mod
├── cmd/
│   └── localctl/
├── internal/
│   ├── runtime/
│   ├── evidence/
│   ├── fixture/
│   └── qualification/
├── docs/
│   └── PSEUDOCODE.md
├── fixtures/
└── testdata/
```

M0 may need almost none of that.

The rule is: earn the directory when the responsibility exists.

## Development checkpoint format

For each meaningful milestone, record enough to answer:

```text
OBJECTIVE

PROPERTY WE NEEDED

WHAT WE BUILT

WHAT THE OPERATOR SHOULD NOW UNDERSTAND

VALIDATION

OBSERVED EVIDENCE

KNOWN FAILURE MODES

WHAT IS PROVEN

WHAT IS NOT PROVEN

NEXT DECISION
```

Small milestones should stay small. This is a reasoning aid, not report-writing theater.

## Scope-drift check

Before adding significant code, ask:

1. What property are we trying to establish?
2. What evidence would establish it?
3. Is this the smallest implementation that can produce that evidence?
4. Are we consuming a decision that has not actually been made yet?
5. Are we introducing an abstraction for a future runtime/client/scheduler that does not exist?
6. Are we confusing model output with verified completion?
7. Are we preserving the raw evidence needed to revisit this conclusion later?

If the answer exposes scope drift, reduce the change.

## Current status

The repository begins in the learning/bootstrap phase.

The intended next move is **M0**, not runtime orchestration:

1. establish the Go module and smallest executable;
2. understand `package main`, `func main()`, build/run/test, arguments, and process exit;
3. validate the executable locally;
4. then manually inspect `llama-server` before writing Go that manages it.

No later milestone should be considered implemented merely because it is documented here.

## Definition of project success

The project is successful if it eventually lets us answer questions such as:

- What local configurations on this machine have actually demonstrated a capability?
- What evidence supports that statement?
- How current is that evidence?
- What does the configuration cost in latency, memory, and switching overhead?
- Can the current warm model perform the request adequately?
- When should local execution be rejected or escalated?
- Can an external coding tool request a capability without caring which model currently provides it?

A larger end-to-end experiment should eventually compare a frontier baseline with local-first execution over a bounded real coding task set and measure verified outcomes, local completions, escalations, bad routing decisions, latency, model switches, and failures.

The optimization target is not "use local at all costs."

It is:

> Use the least expensive capable intelligence while refusing to treat unproven local capability as fact.
