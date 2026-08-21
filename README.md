# localctl

`localctl` is a small Go CLI for learning and exploring the systems boundary around useful local LLMs.

It is being built incrementally by hand so the implementation remains understandable from the operating-system boundary upward:

```text
Go
HTTP
OS processes
llama-server
llama.cpp
GGUF
Metal / Apple Silicon
runtime state
failure behavior
resource behavior
local inference
```

The current implementation is intentionally small.

The objective is not merely to create a convenient wrapper around `llama.cpp`.

The broader objective is to understand what a genuinely useful machine-facing operating layer for local LLMs needs to observe, control, measure, and expose — especially for software-development and other practical local-model workloads.

---

## Current architecture

```text
shell
  |
  v
localctl
Go OS process
  |
  | HTTP
  v
llama-server
separate OS process
  |
  +-- llama.cpp runtime machinery
  |
  +-- loaded model/runtime state
          |
          v
      GGUF-backed model
          |
          v
      Metal / Apple Silicon
```

Important process boundary:

```text
localctl process
!=
llama-server process
```

`localctl` currently communicates with an already-running `llama-server` over HTTP.

It does not yet start, stop, supervise, or own that process.

`llama.cpp` is not another OS process beneath `llama-server`.

`llama-server` is an executable built from the llama.cpp project, and the inference machinery runs inside that server process.

---

## Project direction

The project began with a deliberately narrow question:

> Can a small Go program correctly observe and communicate with a separately running local inference runtime?

That remains the immediate learning path.

The larger question is:

> What does a useful machine-facing operating layer for local LLMs need to know, control, measure, and expose?

Potential long-term concerns include:

```text
runtime discovery
runtime lifecycle
model artifact discovery
artifact identity
runtime build identity
runtime configuration
observed runtime state
resource behavior
latency
time to first token
throughput
bounded inference
failure classification
capability observations
task-specific evaluation
configuration profiles
evidence capture
selection inputs
```

These are directions, not promises that every concern belongs inside LocalCTL.

One purpose of the project is to discover the correct boundaries before building large abstractions.

---

## What LocalCTL should not become

The project is not currently trying to build:

```text
another chat UI
another coding agent
another agent framework
another Ollama
another LM Studio
another generic LLM proxy
another model marketplace
another opaque "best model" router
```

The more interesting questions are:

```text
What actually exists?

What is actually running?

What configuration is actually active?

What did this model/runtime combination actually demonstrate?

What constraints does this machine impose?

What facts should a higher-level system be allowed to rely on?
```

---

## Model name is not capability identity

A useful local-model configuration is more than a model name.

A meaningful identity is closer to:

```text
model
+
model artifact
+
artifact digest
+
quantization
+
runtime
+
runtime version/build
+
machine
+
context configuration
+
offload configuration
+
sampling configuration
+
prompt/system recipe
+
tool/output contract
+
task/workload class
```

There is a major difference between:

```text
"this model is good at coding"
```

and:

```text
this particular model artifact
running under this particular runtime configuration
on this particular machine
demonstrated this bounded capability
within this observed operating envelope
```

LocalCTL is currently learning how to observe the lower layers required to eventually make the second kind of statement.

---

# Relationship to Invariant

`localctl` is currently an independent personal learning project.

It is not currently an Invariant component.

However, the work is directly relevant to questions that Invariant will eventually need answered.

A possible future conceptual relationship is:

```text
       machine / local runtime layer

            LocalCTL-like work
                  |
       +----------+----------+
       |          |          |
    observe    measure    control
       |          |          |
       +----------+----------+
                  |
                  v
           evidence / facts
                  |
          +-------+-------+
          |               |
          v               v
        Bench          Manifold
   qualification       selection
          |               |
          +-------+-------+
                  |
                  v
                Kiln
        execution / authority
```

This is a conceptual separation of responsibilities.

LocalCTL does not currently implement Bench, Manifold, or Kiln.

---

# What examining Manifold taught this project

The existing Invariant Manifold implementation was examined to determine whether LocalCTL should reuse or directly extend it for local-model selection.

The current conclusion is:

> Not yet.

That decision itself produced useful architectural lessons.

---

## Manifold already has a legitimate narrow responsibility

Manifold's responsibility is selection.

Conceptually:

```text
requirement
+
candidate profiles
+
qualification evidence
        |
        v
     Manifold
        |
        v
selection / assignment
```

Manifold deliberately does not own:

```text
provider execution
network access
process control
runtime discovery
runtime supervision
repository mutation
qualification
execution authority
generic orchestration
```

That boundary reinforces an important lesson:

```text
runtime observation
!=
qualification
!=
selection
!=
execution
!=
authority
```

These should not casually collapse into one large AI-management component.

---

## Current Manifold is intentionally simpler than the local-model problem

The current bounded Manifold implementation behaves approximately like:

```text
Intelligence Requirement
        +
Profiles
        +
Eligibility Snapshots
        |
        v
validate contracts
        |
        v
filter by role
        |
        v
require current QUALIFIED evidence
        |
        v
deterministic tie-break
        |
        v
Intelligence Assignment
```

It does not yet perform general-purpose comparison based on:

```text
tokens/sec
memory requirements
context size
task-specific capability
local vs remote
latency
hardware fit
runtime availability
cost
```

Its current deterministic selection rule is deliberately much narrower.

That is appropriate for the Invariant milestone it was designed to prove.

It also means extending it for this project would immediately turn LocalCTL work into real Manifold contract and architecture development.

That would distract from the current learning objective.

---

## Current Manifold contracts are also intentionally specific

The current Invariant profile contracts were created for the bounded system being proven inside Invariant.

They are not yet a universal vocabulary for arbitrary combinations such as:

```text
Granite GGUF
Qwen GGUF
different quantizations
llama.cpp
Ollama
LM Studio
different context sizes
different Apple Silicon machines
different memory envelopes
different local performance characteristics
```

Trying to force those concepts into Manifold now would require genuine evolution of:

```text
canonical schemas
profile identity
requirements
qualification artifacts
selection semantics
fixtures
tests
downstream consumers
```

That may eventually be worthwhile.

It is not required to continue learning about local inference.

---

# Manifold is therefore a reference, not a dependency

For this project, Manifold is currently useful as an architectural north star.

LocalCTL can independently discover what facts a future selector might actually need.

For example, a future observed local profile might conceptually contain facts such as:

```text
candidate:
  granite-local

runtime:
  llama.cpp

artifact:
  granite-4.1-8b-Q4_K_S.gguf

context:
  8192

memory:
  observed value

throughput:
  observed value

capabilities:
  observed code explanation
  observed structured output
  observed small edit behavior
```

Another local configuration could produce another evidence set.

Only when multiple plausible candidates exist does a genuine selection problem appear.

That suggests the progression:

```text
one runtime
    |
    v
understand runtime truth

multiple configurations
    |
    v
understand comparable evidence

multiple viable candidates
    |
    v
discover what selection inputs matter

small selection experiment
    |
    v
learn which concepts generalize

generalizable lessons
    |
    v
potential future Manifold evolution
```

This allows implementation experience to inform architecture instead of forcing architecture onto experiments prematurely.

---

# Selection is not automatically an AI problem

Another useful lesson from Manifold is that candidate selection should begin deterministically.

For example:

```text
Candidate A
context = 4096
memory = 5 GB

Candidate B
context = 16384
memory = 9 GB

Task requirement
context >= 8192
memory budget <= 8 GB
```

The deterministic result is:

```text
A rejected:
context requirement not satisfied

B rejected:
memory constraint not satisfied

result:
NO ELIGIBLE CANDIDATE
```

No model is required to make that decision.

A useful future selection hierarchy might look like:

```text
hard constraints
    |
    v
deterministic eligibility
    |
    v
qualification evidence
    |
    v
measured comparison
    |
    v
explicit preferences
    |
    v
bounded intelligence only if ambiguity remains
```

A useful principle is:

> Use intelligence only where deterministic evidence and policy are insufficient.

---

# How LocalCTL can inform Manifold later

Local-model experiments can help answer architectural questions empirically.

Examples include:

```text
What should a candidate profile actually contain?

Which properties are hard constraints?

Which measurements are stable enough to select on?

What constitutes model/runtime identity?

How should machine-specific capability be represented?

How stale can performance evidence become?

Is context capacity runtime state, qualification evidence, or both?

How should locality be represented?

How should memory pressure affect eligibility?

How should task classes be represented?

When is deterministic selection insufficient?

What evidence should accompany a selection explanation?
```

Rather than:

```text
design Manifold first
        |
        v
force experiments into the design
```

the preferred learning path is:

```text
real experiments
      |
      v
observations
      |
      v
real selection problems emerge
      |
      v
small deterministic experiments
      |
      v
generalizable lessons
      |
      v
future Manifold evolution
```

This keeps LocalCTL useful on its own while allowing it to improve the mental model for a future Manifold.

---

# Possible future runtime-layer relationship to Invariant

It is possible that some machine-facing runtime work developed here eventually becomes an Invariant component or sibling open-source project.

A bounded responsibility could eventually include:

```text
discover local runtimes
discover model artifacts
start and stop runtime processes
observe actual runtime state
apply runtime profiles
measure resource behavior
perform bounded inference probes
capture raw evidence
expose normalized runtime capabilities
```

That component should still not absorb responsibilities belonging to:

```text
Bench
Manifold
Kiln
```

No decision has been made that LocalCTL itself must become that component.

The current project is allowed to remain a learning implementation even if concepts later graduate elsewhere.

---

# Current Go structure

The implementation is intentionally organized so source structure mirrors conceptual boundaries.

```text
main.go
  process entry and exit

cli.go
  CLI command dispatch

runtime.go
  llama-server HTTP behavior and protocol structures

cli_test.go
  CLI behavior tests

runtime_test.go
  runtime HTTP and failure-behavior tests
```

Only `main()` owns actual process termination:

```go
func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}
```

Application behavior returns exit codes instead of calling `os.Exit()` throughout the command implementation.

This allows:

```text
production
  os.Stdout
  os.Stderr

tests
  bytes.Buffer
```

and permits deferred cleanup to execute normally.

---

# Requirements

Current development environment:

- macOS / Apple Silicon
- Go 1.26.5
- `llama.cpp` / `llama-server`
- compatible GGUF model

No third-party Go dependencies are currently used.

---

# Build

```bash
go build
```

This creates:

```text
./localctl
```

The generated executable is ignored by Git.

---

# Commands

## Version

```bash
./localctl version
```

Current output:

```text
localctl dev
```

---

## Runtime status

```bash
./localctl runtime status
```

LocalCTL sends:

```text
GET http://127.0.0.1:8080/health
```

A healthy running `llama-server` produces:

```text
runtime: 200 OK
```

Runtime status has an explicit one-second HTTP timeout.

A controlled server that accepts the request but does not answer within that deadline causes LocalCTL to return non-zero rather than waiting indefinitely.

Therefore:

```text
reachable != responsive
```

A successful health request currently proves:

> Something reachable at the configured address returned a successful `/health` response within the bounded status timeout.

It does not prove:

```text
runtime identity
expected llama.cpp build
expected model identity
complete runtime configuration
qualification for a workload
```

---

## Runtime inspection

```bash
./localctl runtime inspect
```

LocalCTL currently queries:

```text
GET http://127.0.0.1:8080/v1/models
```

It decodes the response into Go structures and reports observed model/runtime information.

An observed development response included:

```text
runtime: llamacpp
model: granite-4.1-8b-Q4_K_S.gguf
context: 2048
training context: 131072
parameters: 8791592960
size: 5087248384
```

This provides stronger evidence than health alone.

It still does not establish complete runtime identity or workload qualification.

---

## Inference

```bash
./localctl runtime infer "What is 2 + 2? Answer briefly."
```

LocalCTL currently:

1. accepts the prompt from the CLI
2. constructs a Go request structure
3. encodes it as JSON
4. creates an HTTP POST
5. sends it to `/v1/chat/completions`
6. checks transport success
7. checks HTTP status
8. decodes the JSON response
9. verifies at least one completion choice exists
10. prints the assistant content

Example observed output:

```text
2 + 2 = 4.
```

---

# Running llama-server

The current development experiment uses:

```bash
MODEL="$HOME/.lmstudio/models/ibm-granite/granite-4.1-8b-GGUF/granite-4.1-8b-Q4_K_S.gguf"

llama-server \
  --model "$MODEL" \
  --host 127.0.0.1 \
  --port 8080 \
  --ctx-size 2048
```

The model path is environment-specific.

The corresponding model ID is currently hard-coded:

```text
granite-4.1-8b-Q4_K_S.gguf
```

This is temporary development state rather than a finished configuration design.

---

# What has been demonstrated

The implementation has been exercised against both a real `llama-server` and controlled HTTP test servers.

Demonstrated behavior includes:

```text
Go module creation
native arm64 Mach-O executable
CLI command dispatch
CLI subcommand dispatch
stdout/stderr separation
exit-code behavior
HTTP communication with a separate OS process
runtime health observation
runtime model inspection
connection-refused handling
HTTP non-2xx handling
JSON request encoding
JSON response decoding
malformed JSON rejection
empty inference-result rejection
real GGUF-backed inference
explicit status timeout behavior
HTTP request cancellation
automated CLI tests
automated runtime HTTP tests
```

A real end-to-end contract request produced:

```text
LOCALCTL_INFERENCE_OK
```

through:

```text
shell
  |
  v
localctl
  |
  | HTTP POST
  v
llama-server
  |
  v
llama.cpp
  |
  v
Granite GGUF model
  |
  v
generated tokens
  |
  | HTTP JSON response
  v
localctl
  |
  v
stdout
```

A normal inference request also produced:

```text
2 + 2 = 4.
```

---

# Failure boundaries exercised

## Transport failure

When no process is listening on port `8080`:

```text
connection refused
```

LocalCTL exits non-zero.

No HTTP response existed.

---

## HTTP failure

A controlled server returning:

```text
503 Service Unavailable
```

is reachable over HTTP, but LocalCTL still exits non-zero.

```text
transport success != HTTP success
```

---

## Protocol failure

A controlled server returning HTTP `200` with malformed JSON is rejected.

```text
HTTP success != valid protocol response
```

---

## Missing inference result

A controlled server returning valid JSON with:

```json
{
  "choices": []
}
```

is rejected.

```text
valid JSON != valid inference result
```

---

## Slow runtime

A controlled server was configured to accept the request and wait for two seconds.

Before an explicit timeout policy:

```text
server accepts request
        |
        v
waits approximately 2 seconds
        |
        v
200 OK
```

LocalCTL simply waited.

After introducing the status timeout:

```text
server accepts request
        |
        v
server stalls
        |
        v
1-second client timeout
        |
        v
request cancelled
        |
        v
LocalCTL returns exit 1
```

The controlled HTTP server observes cancellation through:

```go
r.Context().Done()
```

This establishes:

```text
unreachable
!=
reachable but stalled
```

The current error wording still reports both cases beneath the broad `runtime unreachable` failure class.

That classification may deserve refinement later.

---

# Automated tests

The project now has an automated Go test suite using only the standard library.

Current coverage includes:

```text
version behavior
missing command
unknown command
missing runtime subcommand
unknown runtime subcommand
missing inference prompt

healthy runtime status
unhealthy HTTP response
unreachable runtime
slow/stalled runtime

successful inference
inference request contract
inference HTTP failure
malformed inference JSON
missing inference choices

successful runtime inspection
malformed inspection JSON
empty model list
```

Run:

```bash
go test -v ./...
```

The tests use Go standard-library facilities including:

```text
testing
net/http/httptest
bytes.Buffer
encoding/json
strings
time
```

`httptest.NewServer()` creates a real temporary HTTP listener.

It does not create another OS process.

The controlled server normally executes inside the Go test process.

Therefore:

```text
real HTTP boundary
!=
separate OS process boundary
```

---

# Important distinctions

The project deliberately preserves distinctions such as:

```text
installed != running

running != ready

ready != reachable

reachable != responsive

responsive != inference succeeded

inference succeeded != requested contract followed

contract followed once != reliable

artifact exists != model loaded

configuration requested != runtime state observed

HTTP success != protocol success

protocol success != semantic result

task success != operationally acceptable

evidence != qualification

qualification != selection

selection != execution

selection != authority

capability != authority

passing tests != every intended system property proven
```

These distinctions determine what claims the system is actually justified in making.

---

# Evidence discipline

Useful evidence categories for this project are:

```text
OBSERVED
INFERRED
ASSUMED
EXPECTED
UNKNOWN
```

For example:

```text
OBSERVED:
GET /health returned 200 OK.

INFERRED:
the server considers itself healthy.

UNKNOWN:
whether it is the exact llama.cpp build we intended.
```

Or:

```text
OBSERVED:
the expected inference text was returned once.

SUPPORTED:
this execution path can produce the requested result.

NOT PROVEN:
the configuration is reliable or qualified for a coding workload.
```

The project prefers collecting the evidence required for the actual property rather than promoting a proxy into a stronger claim.

---

# Current limitations

The implementation remains intentionally primitive.

Current limitations include:

- server URL is hard-coded
- model identity is hard-coded
- prompts are expected as a single quoted CLI argument
- complete runtime identity is not verified
- expected llama.cpp build identity is not verified
- model artifacts are not discovered automatically
- runtime configuration is only partially observed
- status has a timeout policy, but inference timeout/cancellation policy is not yet deliberately designed
- runtime inspection timeout behavior is not yet deliberately designed
- runtime lifecycle management does not exist
- persistent configuration does not exist
- resource measurement does not exist
- capability evaluation does not exist
- model/configuration qualification does not exist
- candidate selection does not exist

These are boundaries of the current implementation.

They are not automatically the next features to build.

---

# Current learning checkpoint

The demonstrated inference path is:

```text
CLI arguments
  |
  v
Go command dispatch
  |
  v
Go structs
  |
  v
JSON encoding
  |
  v
HTTP request
  |
  v
HTTP transport
  |
  v
llama-server
  |
  v
llama.cpp inference
  |
  v
HTTP response
  |
  v
JSON decoding
  |
  v
Go response structs
  |
  v
assistant content
  |
  v
stdout
```

The status path additionally demonstrates bounded waiting:

```text
HTTP request
  |
  v
server stalls
  |
  v
1-second client timeout
  |
  v
request cancelled
  |
  v
LocalCTL returns control
```

---

# Next engineering direction

The next useful systems boundary is runtime lifecycle.

Today:

```text
shell
  |
  +-- manually starts llama-server
  |
  +-- runs localctl
```

That manual setup becomes friction for every later experiment.

A future bounded experiment can investigate:

```text
localctl
  |
  v
start llama-server child process
  |
  v
observe PID
  |
  v
determine when runtime is actually ready
  |
  v
use runtime
  |
  v
stop the exact process that was started
```

This introduces useful systems concepts including:

```text
os/exec
parent and child processes
PID ownership
stdout/stderr ownership
startup failure
runtime readiness
process termination
cleanup
```

Important distinctions include:

```text
process started
!=
server listening

server listening
!=
model loaded

model loaded
!=
runtime ready
```

This lifecycle capability should simplify later experiments by removing repeated manual runtime setup while making process ownership explicit.

---

# Development principle

The project continues to use this progression:

```text
mental model
    |
    v
identify one property
    |
    v
predict behavior
    |
    v
small experiment
    |
    v
observe evidence
    |
    v
compare prediction with reality
    |
    v
update mental model
    |
    v
add the minimum implementation needed
```

The important unit of progress is not merely:

```text
feature completed
```

It is:

```text
property understood
+
property demonstrated
+
evidence inspected
```

Complexity should be introduced only when the property being protected requires it.
