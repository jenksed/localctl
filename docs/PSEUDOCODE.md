# localctl — End-to-End Pseudocode and Mental Model

This document is a **directional implementation map**, not a frozen architecture.

Its job is to keep the learning/build process pointed at the original system thesis while allowing the actual design to change when experiments teach us something.

The pseudocode is deliberately written closer to plain English than production Go. When a milestone is implemented, the real code should be smaller than the future-looking pseudocode until the extra behavior is actually required.

The central rule is:

```text
DO NOT IMPLEMENT THE WHOLE DOCUMENT.

Read the current milestone.
Understand the property.
Build the smallest thing that proves it.
Observe reality.
Update the design only when the evidence requires it.
```

---

## 1. The system in one picture

The first useful system is intentionally simple:

```text
USER / SHELL
     │
     │ argv + env + stdin/stdout/stderr
     ▼
LOCALCTL (Go process)
     │
     │ HTTP request with timeout/cancellation
     ▼
LLAMA-SERVER (separate process)
     │
     │ runtime calls
     ▼
LLAMA.CPP
     │
     │ model execution
     ▼
METAL / CPU / HARDWARE
     │
     ▼
GGUF MODEL DATA
```

Later, after we have evidence for what a control plane actually needs:

```text
USER / CODING TOOL
        │
        ▼
     LOCALCTL
        │
        │ language-neutral API
        ▼
   CONTROL PLANE
        │
        ├── capability state
        ├── qualification state
        ├── evidence
        ├── runtime lifecycle
        ├── routing
        ├── queues
        ├── resource policy
        └── escalation
        │
        ▼
   RUNTIME ADAPTER
        │
        ▼
    LLAMA-SERVER
        │
        ▼
     LLAMA.CPP
```

The second picture is **not** permission to build the control plane now.

---

# 2. Responsibility boundaries

Before writing code, keep ownership explicit.

## Shell owns

```text
resolve executable
construct argv
environment variables
start localctl process
connect standard streams
wait for process
receive exit code
```

Conceptually:

```text
shell.run(command):
    executable = find(command.name)
    process = OS_START_PROCESS(
        executable,
        argv = command.arguments,
        env = current_environment,
        stdin = shell_stdin,
        stdout = shell_stdout,
        stderr = shell_stderr,
    )

    exit_status = WAIT(process)
    return exit_status
```

The shell does not know whether a model is good at diff review.

## localctl owns initially

```text
interpret CLI request
resolve local configuration
construct bounded HTTP requests
apply deadlines/cancellation
parse transport responses
validate explicit output properties
record evidence
present useful operator output
return meaningful exit status
```

It does not initially own:

```text
token generation
model loading internals
GPU kernels
KV cache internals
arbitrary shell authority
repository mutation
qualification by vibes
```

## llama-server owns initially

```text
server process
HTTP inference interface
loaded runtime/model state
server-side runtime configuration
request scheduling behavior inside llama.cpp server
readiness/loading state
```

## llama.cpp owns

```text
model loading
tokenization
inference execution
sampling
runtime caches
backend execution
Metal/CPU interaction
```

## GGUF owns nothing

A model file is passive data.

```text
GGUF_PRESENT == true
```

proves only:

```text
there is a file at a path
```

It does not prove:

```text
valid
loadable
compatible
responsive
capable
qualified
```

---

# 3. The proof ladder

Every future feature should respect this progression:

```text
DISCOVERED
    ↓ evidence required
LOADABLE
    ↓ evidence required
READY
    ↓ evidence required
RESPONSIVE
    ↓ evidence required
CONTRACT_COMPLIANT
    ↓ evidence required
TASK_CAPABLE
    ↓ explicit criteria
QUALIFIED
```

Never skip levels because an API returned a model name.

Conceptual functions:

```text
function discover_model(path):
    if FILE_EXISTS(path):
        return Observation("model artifact present", path)
    else:
        return Failure("artifact missing")

function prove_loadable(runtime, model):
    result = runtime.start(model)

    if result.failed:
        return Evidence(loadable = false, failure = result.error)

    return Evidence(loadable = true)

function prove_ready(runtime):
    health = runtime.health()

    if health.status == READY:
        return Evidence(ready = true)

    return Evidence(ready = false, observed_status = health.status)

function prove_responsive(runtime, probe):
    response = runtime.infer(probe)

    if response.transport_failed:
        return Evidence(responsive = false)

    return Evidence(
        responsive = true,
        raw_response = response.raw,
    )

function prove_contract(response, contract):
    validation = contract.validate(response)

    return Evidence(
        contract_satisfied = validation.ok,
        validation_details = validation,
    )

function qualification_decision(evidence_set, criteria):
    // This is a decision over evidence.
    // It must not rewrite the evidence.

    return APPLY_EXPLICIT_CRITERIA(evidence_set, criteria)
```

---

# 4. M0 — smallest real Go executable

## Property

We can build, run, test, and reason about a real Go executable before it touches inference.

## Minimum conceptual repository

Do not create mature architecture yet.

Something as small as:

```text
localctl/
├── go.mod
├── main.go
└── README.md
```

may be enough.

## Conceptual Go program

```text
package main

function main():
    args = OS_ARGUMENTS()

    if args == [program, "version"]:
        PRINT(version)
        EXIT(0)

    PRINT_ERROR("usage: localctl version")
    EXIT(nonzero)
```

Do not implement command routers, service containers, dependency injection, or package hierarchies to support one command.

## M0 observations we should be able to explain

```text
What does `go mod init` create?
What is a module path?
Why does executable code use `package main`?
Why is `main()` special?
What is compiled by `go build`?
What does `go run` do differently from running the built binary?
Where do command-line arguments come from?
Who receives the exit code?
What does `go test ./...` actually discover?
```

## M0 acceptance

Conceptually:

```text
RUN: go test ./...
EXPECT: exit 0

RUN: go build ...
EXPECT: binary produced

RUN: localctl version
EXPECT:
    correct output
    exit 0

RUN: localctl nonsense
EXPECT:
    useful stderr
    nonzero exit
```

M0 proves nothing about llama.cpp.

---

# 5. M1 — manually understand llama-server

## Property

The operator understands the runtime process before Go is allowed to manage it.

## Manual flow

```text
model_path = CHOOSE_REAL_GGUF()

command = llama-server(
    model = model_path,
    port = known_port,
    explicit_runtime_options = minimal_set,
)

START_MANUALLY(command)

OBSERVE:
    process exists
    stdout/stderr
    model loading behavior
    port/listening behavior
    readiness behavior
    memory behavior if visible

STOP_MANUALLY()

OBSERVE:
    process exits
    port disappears
    model state disappears with process unless external state says otherwise
```

## Important distinction

```text
PROCESS_EXISTS
```

is not the same as:

```text
SERVER_READY
```

and neither proves:

```text
MODEL_USEFUL_FOR_TASK
```

## M1 questions

```text
Who started llama-server?
Who owns its lifetime right now?
Where did its model path come from?
Where did its context/runtime configuration come from?
What happens if the model path is invalid?
What happens while the model is still loading?
What endpoint proves readiness on the installed version?
What signal stops it?
```

No Go process management yet.

---

# 6. M2 — runtime status probe

## Property

`localctl` can distinguish useful runtime reachability/readiness states without pretending that readiness is qualification.

Current upstream llama-server documents `/health` and `/v1/health`; installed runtime behavior remains authoritative.

## Possible status vocabulary

Do not prematurely freeze this enum. Conceptually:

```text
UNREACHABLE
LOADING
READY
UNEXPECTED
```

Transport errors should preserve their actual cause.

## Conceptual CLI flow

```text
function command_runtime_status(config):
    endpoint = config.runtime_endpoint

    deadline = NOW + config.status_timeout
    context = NEW_CONTEXT(deadline)

    result = probe_health(context, endpoint)

    if result.connection_refused:
        PRINT("runtime unreachable")
        EXIT(nonzero)

    if result.timeout:
        PRINT("runtime status timed out")
        EXIT(nonzero)

    if result.loading:
        PRINT("runtime loading")
        EXIT(status_for_not_ready)

    if result.ready:
        PRINT("runtime ready")
        EXIT(0)

    PRINT("runtime returned unexpected state")
    PRESERVE(result.raw_response)
    EXIT(nonzero)
```

## HTTP pseudocode

```text
function probe_health(context, endpoint):
    request = HTTP_GET(endpoint + "/health")
    request.attach_context(context)

    response, error = HTTP_CLIENT.DO(request)

    if error:
        return TransportFailure(error)

    raw_body = READ_RESPONSE_BODY_BOUNDED(response)

    if response.status == 200:
        parsed = TRY_PARSE_HEALTH(raw_body)

        if parsed says ready:
            return Ready(raw = raw_body)

        return Unexpected(status = 200, raw = raw_body)

    if response.status == 503:
        return Loading(raw = raw_body)

    return Unexpected(
        status = response.status,
        raw = raw_body,
    )
```

Notice what is retained:

```text
HTTP status
raw body
transport error
elapsed time if useful
```

Do not throw away the unexpected body simply because parsing failed.

## Failure experiments

```text
server stopped
wrong port
wrong host
deadline too short
server loading
unexpected HTTP response
malformed health body
```

---

# 7. M3 — runtime/model inspection

## Property

We can inspect what the running runtime actually exposes without immediately pretending those fields are our permanent domain schema.

## Rule

```text
FIRST observe runtime JSON.
THEN define the transport struct needed to decode it.
THEN decide whether a smaller internal concept is useful.
```

Not:

```text
invent UniversalModel with 48 fields
then force llama-server into it
```

## Conceptual flow

```text
function command_models(config):
    response = HTTP_GET(runtime + "/v1/models")

    raw = response.body

    if response failed:
        return failure + raw evidence

    transport_models = DECODE_RUNTIME_RESPONSE(raw)

    for each transport_model:
        PRINT_RUNTIME_FACTS(transport_model)

    if evidence recording enabled:
        SAVE_RAW_RUNTIME_RESPONSE(raw)
```

## Possible transport/domain split

```text
LlamaServerModelsResponse
    // shaped like actual server JSON

ObservedModel
    // only introduced if we can identify stable concepts
```

Possible observed concepts:

```text
runtime-reported ID
runtime-reported object/type
other fields actually returned
raw source response
```

Do not infer quantization from a pretty name if it is not actually proven.

---

# 8. M4 — first bounded inference

## Property

The runtime can accept a bounded inference request and we can distinguish transport success from task/property success.

## Probe task

```text
TASK_ID = "exact-pong-v1"
PROMPT = "Return exactly the word PONG and nothing else."

EXPECTED_PROPERTY:
    normalize only permitted transport whitespace
    output == "PONG"
```

## Execution flow

```text
function execute_probe(runtime, model, prompt, config):
    run_id = NEW_RUN_ID()
    start = MONOTONIC_TIME()

    request_record = {
        run_id,
        runtime,
        model,
        prompt,
        config,
    }

    response, transport_error = runtime.chat_completion(
        context = bounded_context,
        model = model,
        prompt = prompt,
        generation_options = explicit_options,
    )

    finish = MONOTONIC_TIME()

    evidence = {
        run_id,
        request = request_record,
        elapsed = finish - start,
        transport_error,
        raw_response = response.raw_if_any,
    }

    if transport_error:
        evidence.transport_success = false
        return evidence

    evidence.transport_success = true

    extracted_text, parse_error = EXTRACT_ASSISTANT_TEXT(response)
    evidence.parsed_text = extracted_text
    evidence.parse_error = parse_error

    if parse_error:
        evidence.property_satisfied = false
        return evidence

    evidence.property_satisfied = EXACT_PONG_VALIDATOR(extracted_text)

    return evidence
```

## Important result examples

### Case A

```text
HTTP 200
body parse succeeds
model output = PONG

transport_success = true
property_satisfied = true
```

### Case B

```text
HTTP 200
model output = "Sure! PONG"

transport_success = true
property_satisfied = false
```

### Case C

```text
connection timeout

transport_success = false
property_satisfied = not evaluated
```

Do not flatten these into one `success bool` too early.

---

# 9. M5 — Go begins owning llama-server process lifecycle

## Property

We have explicit semantics for child-process ownership instead of accidentally inheriting whatever behavior `os/exec` gives us.

## First design question

Choose one initial lifecycle model deliberately.

Possible model A:

```text
localctl runtime serve ...

localctl remains parent/foreground supervisor
llama-server is child
Ctrl-C reaches localctl
localctl requests child shutdown
localctl waits
localctl exits
```

Possible model B:

```text
localctl runtime start ...

start child detached
persist enough state to find it later
return shell immediately
```

Model B creates more problems:

```text
PID persistence
stale PID detection
ownership across CLI invocations
log location
shutdown command
crash discovery
orphan behavior
```

Therefore start with the simpler lifecycle unless the use case requires detachment.

## Foreground child pseudocode

```text
function runtime_serve(config):
    VERIFY_EXECUTABLE_EXISTS(config.llama_server_path)
    VERIFY_MODEL_PATH_EXISTS(config.model_path)

    args = BUILD_EXPLICIT_LLAMA_ARGS(config)

    child = NEW_CHILD_PROCESS(
        executable = config.llama_server_path,
        args = args,
    )

    child.stdout = current_stdout_or_log_pipe
    child.stderr = current_stderr_or_log_pipe

    signal_context = CONTEXT_CANCELLED_BY(SIGINT, SIGTERM)

    start_result = child.START()

    if start_result failed:
        return StartupFailure(start_result.error)

    readiness_result = WAIT_FOR_READINESS(
        child = child,
        endpoint = config.endpoint,
        timeout = config.startup_timeout,
    )

    if readiness_result failed:
        REQUEST_CHILD_STOP(child)
        WAIT_BOUNDED_FOR_EXIT(child)
        return StartupFailure(readiness_result)

    PRINT("runtime ready")

    winner = WAIT_FOR_FIRST(
        signal_context.cancelled,
        child.exited,
    )

    if winner == child.exited:
        return RuntimeExited(child.exit_status)

    if winner == signal:
        REQUEST_GRACEFUL_SHUTDOWN(child)

        if child exits within grace_period:
            return CleanShutdown

        REQUEST_FORCEFUL_SHUTDOWN(child)
        WAIT(child)
        return ForcedShutdown
```

## Readiness algorithm

```text
function wait_for_readiness(child, endpoint, timeout):
    deadline = NOW + timeout

    loop:
        if child has exited:
            return Failure(
                "child exited before ready",
                exit_status,
            )

        health = probe_health(short_context, endpoint)

        if health == READY:
            return Success

        if NOW >= deadline:
            return Failure("startup timeout")

        SLEEP(bounded_backoff)
```

Important:

Do not call process creation successful merely because `Start()` returned nil.

The required property is closer to:

```text
child process started
AND
expected runtime became ready
```

---

# 10. M6 — structured output contract

## Property

We can ask for a machine-readable response and independently validate whether the response satisfies the contract.

## Example contract

```text
ClassificationResult:
    classification ∈ {bug, configuration, environment, unknown}
    confidence ∈ [0.0, 1.0]
```

## Contract flow

```text
function run_structured_task(task):
    response = infer(task.prompt, task.runtime_options)

    evidence.raw_response = response.raw

    if transport failed:
        evidence.transport_success = false
        return evidence

    candidate_text = EXTRACT_TEXT(response)

    json_value, json_error = PARSE_JSON(candidate_text)

    if json_error:
        evidence.contract_success = false
        evidence.contract_failure = {
            kind: "invalid_json",
            error: json_error,
            candidate_text: candidate_text,
        }
        return evidence

    schema_result = VALIDATE_SCHEMA(json_value)

    if schema_result failed:
        evidence.contract_success = false
        evidence.contract_failure = schema_result
        return evidence

    evidence.contract_success = true
    evidence.parsed_output = json_value

    return evidence
```

## Cases to intentionally test

```text
{"classification":"bug","confidence":0.9}
```

valid.

```text
```json
{"classification":"bug","confidence":0.9}
```
```

Maybe invalid under a strict exact-JSON contract. Do not silently strip fences unless the contract explicitly permits that normalization.

```text
{"classification":"banana","confidence":4}
```

JSON transport shape may parse, but contract fails.

This separation is important.

---

# 11. M7 — evidence persistence

## Property

Facts about runs survive the process and can later support comparison, diagnosis, and qualification.

## Rule

Preserve facts first.

Derive summaries later.

## Conceptual evidence object

This is not a frozen Go struct.

```text
RunEvidence:
    schema_version

    identity:
        run_id
        started_at_wall_clock
        localctl_version

    machine:
        os
        architecture
        relevant_hardware_facts

    runtime:
        name = "llama.cpp"
        version_if_observed
        server_binary_identity_if_observed
        endpoint
        launch_config_if_owned

    model:
        runtime_model_id
        model_path_or_artifact_identity
        artifact_hash_if_worth_cost
        quantization_if_proven
        profile_id

    task:
        task_id
        fixture_version
        prompt/input
        expected_contract

    timing:
        started_monotonic
        elapsed
        optional load/warm metadata

    transport:
        request_shape
        response_status
        raw_response
        transport_error

    interpretation:
        parsed_output
        parsing_error
        contract_result
        validator_result

    failure:
        kind
        details
```

## Storage candidate A — one JSON file per run

```text
.localctl/
└── runs/
    ├── run-001.json
    ├── run-002.json
    └── run-003.json
```

Write pattern:

```text
function persist_run(run):
    bytes = ENCODE_JSON(run)

    temp_path = target + ".tmp"

    WRITE_FULL_FILE(temp_path, bytes)
    FSYNC_IF_REQUIRED_BY_DURABILITY_GOAL()
    RENAME(temp_path, target)
```

The exact durability guarantees should be explicit before we pretend an atomic rename solves every problem.

## Storage candidate B — JSONL

Good for append-oriented inspection; less convenient for independently addressable runs and partial-write recovery.

## Storage candidate C — SQLite

Do not choose this simply because the system will eventually query evidence.

Introduce it when we can identify real requirements such as:

```text
indexed queries
atomic multi-record changes
concurrent writers
qualification references
migration/versioning needs
```

---

# 12. M8 — fixture tasks

## Property

We can repeat a bounded useful task with an explicit expected property.

## Conceptual fixture

```text
Fixture:
    id
    version
    capability
    input
    prompt_template
    contract
    validator
    timeout
    metadata
```

Example:

```text
Fixture:
    id = "ts-test-diagnosis-001"
    capability = "test_failure_diagnosis"

    input:
        failing_test
        stack_trace
        relevant_function

    contract:
        JSON object with `failing_symbol` and `reason`

    validator:
        failing_symbol must equal "parseConfig"
```

## Fixture execution

```text
function execute_fixture(fixture, profile):
    prepared_input = BUILD_INPUT_EXACTLY_AS_FIXTURE_DEFINES(fixture)

    response_evidence = infer(
        prompt = RENDER(fixture.prompt_template, prepared_input),
        profile = profile,
    )

    contract_evidence = VALIDATE_CONTRACT(response_evidence)

    if contract_evidence.failed:
        return FixtureResult(
            passed = false,
            evidence = all_evidence,
        )

    property_result = fixture.validator(contract_evidence.parsed_output)

    return FixtureResult(
        passed = property_result.ok,
        evidence = all_evidence,
        validator_details = property_result,
    )
```

## Bad fixture

```text
"Review this diff and give a strong answer."
```

There is no stable observable property.

## Better fixture

```text
Given this diff and public contract:

response must identify that error handling was removed from function X
AND
must not claim function Y changed
AND
must satisfy schema Z
```

Still imperfect, but testable.

---

# 13. M9 — repeated measurement

## Property

We have distributions and failure patterns rather than one successful anecdote.

## Repetition flow

```text
function measure_fixture(profile, fixture, repetitions):
    results = []

    for i in 1..repetitions:
        result = execute_fixture(fixture, profile)
        PERSIST(result.evidence)
        results.append(result)

    return summarize(results)
```

## Summary pseudocode

```text
function summarize(results):
    return MeasurementSummary(
        attempts = COUNT(results),
        property_passes = COUNT(result.passed),
        contract_passes = COUNT(result.contract_success),
        transport_failures = COUNT(result.transport_failed),
        median_latency = MEDIAN(latencies),
        p95_latency = P95(latencies) if sample supports it,
        observed_failure_kinds = GROUP_FAILURES(results),
        evidence_refs = RUN_IDS(results),
    )
```

Do not overstate statistics from tiny sample sizes.

---

# 14. M10 — profiles

## Property

A reusable execution configuration has an identity, and changing it does not rewrite history.

## Conceptual profile

```text
Profile:
    id = "qwen-fast-v1"
    runtime = "llama.cpp"
    model_artifact = known model
    runtime_parameters:
        context_size = explicit value
        other observed/controlled flags
    inference_parameters:
        temperature = explicit value if used
        max_tokens = explicit value
        other settings only if supported
```

## Versioning rule

Do not mutate:

```text
qwen-fast-v1
```

and then pretend old evidence used the new values.

Instead:

```text
qwen-fast-v1
qwen-fast-v2
```

or use an immutable digest of profile content.

Conceptually:

```text
profile_identity = HASH(CANONICAL_PROFILE_CONTENT)
```

Only add hashing if it solves a real identity problem; do not add cryptographic theater.

---

# 15. M11 — qualification

## Property

The system can make an explicit bounded decision about demonstrated capability without confusing that decision with the underlying measurements.

## Conceptual qualification criteria

```text
QualificationCriteria:
    capability = "test_failure_diagnosis"
    fixture_pack = "diagnosis-v3"
    minimum_attempts = 20
    minimum_property_success_rate = 0.90
    minimum_contract_success_rate = 0.98
    maximum_median_latency = 3 seconds
    disqualifying_failure_kinds = [...]
```

## Qualification flow

```text
function qualify(profile, criteria, evidence_repository):
    measurements = FIND_MEASUREMENTS(
        profile = profile,
        capability = criteria.capability,
        fixture_pack = criteria.fixture_pack,
        comparable_environment = explicit_rules,
    )

    if measurements.attempts < criteria.minimum_attempts:
        return QualificationDecision(
            status = INSUFFICIENT_EVIDENCE,
            evidence_refs = measurements.refs,
        )

    violations = []

    if measurements.property_success_rate < criteria.minimum_property_success_rate:
        violations.add("property success below threshold")

    if measurements.contract_success_rate < criteria.minimum_contract_success_rate:
        violations.add("contract compliance below threshold")

    if measurements.median_latency > criteria.maximum_median_latency:
        violations.add("latency above threshold")

    if measurements contains disqualifying failures:
        violations.add("disqualifying failure observed")

    if violations not empty:
        return QualificationDecision(
            status = NOT_QUALIFIED,
            reasons = violations,
            evidence_refs = measurements.refs,
        )

    return QualificationDecision(
        status = QUALIFIED,
        criteria_version = criteria.version,
        evidence_refs = measurements.refs,
    )
```

The decision references evidence.

It does not replace evidence.

---

# 16. Evidence currentness

A qualification can become stale even if nothing edits its record.

Potential invalidating changes:

```text
model artifact changed
runtime version changed materially
profile changed
machine changed
fixture pack changed
criteria changed
runtime backend changed
important prompt preparation changed
```

Do not invent a universal stale/equivalent rule early.

Conceptually:

```text
function currentness(decision, current_environment):
    differences = COMPARE(
        decision.execution_identity,
        current_environment.execution_identity,
    )

    return APPLY_CURRENTNESS_POLICY(differences)
```

The currentness policy itself should be inspectable.

---

# 17. M12 — resource observation

## Property

We understand enough about local resource behavior to know whether control/scheduling is warranted.

## Observations

```text
cold model load duration
warm request latency
resident memory if measurable reliably
context-size impact
model-switch cost
quantization impact
parallel-request behavior
```

## Experiment flow

```text
function compare_cold_and_warm(profile, fixture):
    ensure_runtime_stopped()

    cold_start = START_TIMER()
    runtime = start_profile(profile)
    wait_until_ready(runtime)
    cold_ready_duration = STOP_TIMER(cold_start)

    first = execute_fixture(fixture, profile)
    second = execute_fixture(fixture, profile)
    third = execute_fixture(fixture, profile)

    record:
        cold_ready_duration
        first latency
        second latency
        third latency
        resource observations
```

Do not infer model-switch strategy from one run.

---

# 18. M13 — concurrency experiments

## Property

We understand what happens when multiple callers compete for the real runtime.

Only now should Go concurrency become a first-class learning target.

## Unbounded bad version

```text
for each request:
    go execute(request)
```

This says nothing about the capacity of the runtime.

## Bounded experiment

```text
function execute_batch(requests, max_in_flight):
    semaphore = NEW_SEMAPHORE(max_in_flight)
    results_channel = NEW_CHANNEL()

    for request in requests:
        launch goroutine:
            acquire semaphore OR stop if request context cancelled

            try:
                result = execute(request)
                send result unless receiver/context gone
            finally:
                release semaphore

    collect results until all launched work completes/cancels
```

## Questions to answer with experiments

```text
Does llama-server itself queue?
What does configured parallelism mean on our build/profile?
How does latency change at 1, 2, 4, N callers?
What happens to memory?
What happens when one request cancels?
Does cancellation actually stop runtime work or only stop our caller waiting?
Can a slow request starve shorter ones?
What happens if the runtime exits with requests in flight?
```

Run the Go race detector where the code has shared state:

```text
go test -race ./...
```

A clean race detector is evidence about data races exercised by the tests, not proof of correct scheduling semantics.

---

# 19. M14 — deterministic capability selection

## Property

Given multiple **qualified** configurations, we can choose one using explicit rules and explain the decision.

## Request concept

```text
CapabilityRequest:
    capability = "diff_review"
    structured_output_required = true
    latency_ceiling = 5 seconds
    memory_ceiling = optional
    allow_unqualified = false
```

## Candidate filtering

```text
function select(request, qualification_store, runtime_state):
    candidates = qualification_store.current_qualified_for(request.capability)

    candidates = FILTER(candidates, candidate ->
        satisfies_contract_requirement(candidate, request)
        AND satisfies_latency_requirement(candidate, request)
        AND satisfies_memory_requirement_if_known(candidate, request)
    )

    if candidates empty:
        return NoQualifiedLocalCandidate(
            reason = EXPLAIN_FILTER_FAILURES()
        )

    ranked = SORT_BY_EXPLICIT_POLICY(candidates, [
        currently_loaded_and_good_enough,
        lower_switching_cost,
        lower_observed_latency,
        stronger_success_margin,
        deterministic_tiebreaker,
    ])

    selected = ranked.first

    return Selection(
        profile = selected,
        explanation = BUILD_EXPLANATION(selected, candidates, request),
    )
```

Do not allow:

```text
LLM: "I think Model A feels like the right choice."
```

as the initial router.

## Example explanation

```text
Selected qwen-fast-v2 because:
- currently qualified for diff_review under criteria v3
- structured contract compliance = 100% in referenced evidence
- median latency = 1.8s, below 5s ceiling
- already loaded
- alternative B is 4% more successful but requires 9s model switch and no request requires that margin
```

Whether that exact policy is correct is an empirical/design question. The key is that it is inspectable.

---

# 20. Context preparation

One of the long-term hypotheses is that preparing a smaller, relevant task package can improve local-model performance more than simply asking a bigger model to ingest more repository context.

Do not build a universal context compiler early.

Start with task-specific deterministic recipes.

## Test diagnosis recipe

```text
function prepare_test_diagnosis(repo, failing_test):
    return ContextPackage(
        task = failing_test.name,
        failure_output = RUN_OR_CAPTURE_FAILURE(failing_test),
        stack_trace = relevant_trace,
        implementation = SOURCE_AROUND_REFERENCED_SYMBOLS(),
        nearby_tests = RELEVANT_TESTS(),
        recent_diff = GIT_DIFF_IF_RELEVANT(),
    )
```

## Diff-review recipe

```text
function prepare_diff_review(repo, diff):
    changed_symbols = PARSE_CHANGED_SYMBOLS(diff)

    return ContextPackage(
        diff = diff,
        public_interfaces = FIND_RELEVANT_PUBLIC_INTERFACES(changed_symbols),
        nearby_tests = FIND_RELEVANT_TESTS(changed_symbols),
        relevant_callers = BOUNDED_REFERENCE_SEARCH(changed_symbols),
    )
```

## Comparison experiment

```text
same model
same profile
same fixture

A = raw broad repository context
B = bounded prepared context

measure:
    property success
    contract compliance
    latency
    input size
    failure pattern
```

This is how we test the hypothesis instead of assuming it.

---

# 21. Local-first execution and escalation

Eventually the system should not pretend local inference can do work it has not demonstrated.

Conceptually:

```text
function execute_capability_request(request):
    selection = deterministic_selector(request)

    if selection.has_qualified_local_candidate:
        result = execute_local(selection.profile, request)

        verification = VERIFY_RESULT(request, result)

        if verification.passed:
            return CompletedLocally(
                result,
                evidence,
                verification,
            )

        if policy permits bounded retry and retry_is_safe:
            retry_result = retry_according_to_explicit_policy()

            if retry_result verified:
                return CompletedLocally(...)

        return EscalationRequired(
            reason = "qualified local attempt did not produce verified completion",
            local_evidence = evidence,
        )

    return EscalationRequired(
        reason = selection.no_candidate_reason,
    )
```

The actual frontier/provider system belongs later.

The important early property is that `NO_QUALIFIED_LOCAL_CANDIDATE` is a valid, useful answer.

---

# 22. Why model output cannot own completion

Future coding flow:

```text
MODEL:
    proposes patch

SYSTEM:
    stores proposal
    applies only through explicitly authorized mechanism
    runs required verification
    captures output

REVIEW / POLICY / HUMAN:
    determines acceptance according to actual system architecture
```

Conceptual distinction:

```text
Proposal:
    "change line X to Y"

Evidence:
    patch bytes
    test output
    lint/typecheck output
    runtime behavior

Decision:
    accept / reject / escalate
```

Never:

```text
if model says "fixed":
    mark complete
```

---

# 23. M15 — language-neutral service contract

Only after the Go/direct-runtime phase reveals real requirements should we define a control-plane boundary.

The control plane should look boring from Go.

Possible API concepts:

```text
GET  /status
GET  /runtimes
GET  /models
GET  /capabilities
GET  /qualifications/{capability}
GET  /evidence/{run_id}
POST /execute
```

These are examples, not committed endpoints.

## Execute request concept

```text
POST /execute

{
    "capability": "diff_review",
    "constraints": {
        "max_latency_ms": 5000,
        "structured_output": true
    },
    "input": {...}
}
```

Response concept:

```text
{
    "request_id": "...",
    "selection": {
        "profile": "...",
        "reason": "..."
    },
    "result": {...},
    "evidence_refs": ["..."],
    "verification": {...}
}
```

Do not leak:

```text
GenServer PID
Erlang term encoding
Go interface identity
internal actor names
```

across the public boundary.

---

# 24. M16 — Elixir/OTP handoff

This is the future point where Elixir may become the natural long-lived control plane.

Conceptual ownership:

```text
ApplicationSupervisor
│
├── RuntimeSupervisor
│   └── runtime lifecycle processes
│
├── RequestSupervisor
│   └── bounded request processes
│
├── ResourceManager
│   └── owns mutable resource state
│
├── CapabilityRegistry
│   └── qualification/currentness projection
│
└── Evidence subsystem
```

The important point is **not** the names or exact supervision tree.

The important point is that by this stage we should have empirical answers to:

```text
What state needs a long-lived owner?
What runtime lifecycle transitions exist?
What can fail independently?
What needs backpressure?
What must survive process restart?
What is reconstructable from evidence?
What should remain a Go/local-host responsibility?
```

Elixir should consume those discovered requirements rather than invent them.

---

# 25. Possible future Go host agent

Do not build this unless the architecture demonstrates a need.

Potential future split:

```text
Elixir control plane
        │
        │ HTTP/gRPC
        ▼
Go machine agent
        │
        ├── start/stop runtime
        ├── inspect local processes
        ├── inspect machine resources
        └── expose bounded host operations
        │
        ▼
llama.cpp / MLX / other runtime
```

Reasons this might eventually be justified:

```text
multiple machines
portable host installation
OS-local lifecycle management
privilege separation
runtime process isolation
machine-specific resource observation
```

Reasons that are **not** enough:

```text
"Go is good for agents"
"microservices are scalable"
"we want more Go in the repo"
```

---

# 26. Runtime abstraction — when earned

Do not begin with:

```text
interface UniversalRuntime {
    DiscoverEverything()
    RunAnything()
    ManageAnyModel()
}
```

Start with llama.cpp-specific reality.

Later, after adding another runtime, inspect the overlap.

Maybe we eventually discover an interface like:

```text
Runtime:
    Health(context) -> RuntimeHealth
    Models(context) -> []ObservedModel
    Infer(context, request) -> RawInferenceResult
```

But process management may not belong in the same interface if managed runtimes and owned child processes have different authority/lifecycle semantics.

The interface should emerge from multiple real implementations, not from imagination.

---

# 27. LM Studio later

LM Studio can later be useful as a second runtime path.

Experiment:

```text
same conceptual model family
same bounded task
similar generation/profile intent

A: direct llama.cpp / GGUF
B: LM Studio managed runtime
```

Compare what is actually comparable:

```text
latency
memory observations
contract compliance
runtime reliability
configuration exposure
model lifecycle cost
operational complexity
```

Do not assume configuration equivalence merely because both expose an OpenAI-compatible API.

---

# 28. MLX-native later

On Apple hardware, an MLX-native path may provide meaningfully different behavior/performance.

Again:

```text
measure
not assume
```

Potential question:

```text
Does Apple-native MLX execution materially improve the useful work/cost envelope enough to justify another runtime adapter?
```

---

# 29. Native/cgo/Zig/C boundary — only after evidence

Current intended boundary:

```text
localctl
   ↓ HTTP
llama-server
```

Possible future boundary:

```text
Go
 ↓ cgo
native llama.cpp
```

or:

```text
Go
 ↓ local IPC
small Zig/C runtime bridge
 ↓
llama.cpp
```

Before doing this, require an identified limitation such as:

```text
server serialization overhead materially matters
required runtime control is unavailable over HTTP
process boundary prevents needed resource ownership
important instrumentation unavailable remotely
```

Then measure before and after.

Do not introduce native complexity as a prestige feature.

---

# 30. Error taxonomy — grow from observed failures

Do not begin with 100 error types.

A conceptual hierarchy may eventually become useful:

```text
ConfigurationFailure
    missing executable
    missing model
    invalid config

TransportFailure
    DNS/connection
    timeout
    cancellation
    unexpected HTTP

RuntimeFailure
    process startup
    model load
    runtime crash
    not ready

InferenceFailure
    malformed runtime response
    no assistant content

ContractFailure
    invalid JSON
    schema mismatch
    prohibited extra output

ValidationFailure
    output structurally valid but property false

EvidenceFailure
    persistence/serialization failure

QualificationFailure
    insufficient evidence
    threshold violation
    stale evidence
```

Only promote recurring distinctions into formal types when callers need to behave differently.

---

# 31. Retry policy

Retry is not automatically safe or useful.

Before retrying, ask:

```text
Did anything mutate?
Was the operation idempotent?
Did the runtime possibly complete after our timeout?
Will a retry consume meaningful compute?
Is the failure transient?
Does repeated sampling change the experiment itself?
```

For a pure inference request:

```text
retry may be operationally safe
BUT
retry changes measurement semantics
AND
may hide reliability problems
```

Therefore evidence should record attempts separately.

Conceptual:

```text
LogicalRequest
    ├── Attempt 1 -> timeout
    └── Attempt 2 -> success
```

Do not rewrite that as:

```text
request succeeded
```

without retaining the failed attempt.

---

# 32. Cancellation

Cancellation has multiple layers.

```text
user presses Ctrl-C
    ↓
Go context cancelled
    ↓
HTTP request abandoned/cancelled
    ↓
Does llama-server stop generation?
    ↓
Does llama.cpp release work immediately?
```

Do not assume the lower layers cancelled merely because Go returned `context canceled`.

Test it.

Evidence may need to distinguish:

```text
caller stopped waiting
server acknowledged cancellation
runtime work actually ceased
```

Those are different properties.

---

# 33. Time

Use wall-clock and monotonic time for different reasons.

Conceptually:

```text
wall_clock_started_at:
    useful for provenance / human inspection

monotonic_elapsed:
    useful for duration measurement
```

Do not calculate latency from wall-clock timestamps if the language/runtime already provides monotonic duration semantics.

---

# 34. Machine identity

Do not collect telemetry for its own sake.

Record only facts needed to judge comparability.

Potential facts:

```text
OS/version if materially relevant
architecture
memory capacity
chip/GPU identity if observable and useful
llama.cpp version/build identity
localctl version
model artifact identity
profile
```

Question:

> If we look at this result six months later, what facts would determine whether it is still comparable to today's environment?

That question should drive the schema.

---

# 35. Qualification does not imply authority

Even a configuration qualified for:

```text
small_code_edit
```

should initially mean:

```text
it has demonstrated the ability to produce candidate edits under criteria X
```

not:

```text
it may directly mutate a repository
```

Future mutation path should be separately governed.

Conceptually:

```text
model output
    ↓
candidate patch
    ↓
patch parser / bounded representation
    ↓
authority gate
    ↓
apply if explicitly permitted
    ↓
verification
    ↓
evidence
    ↓
acceptance decision
```

---

# 36. Real coding task experiment

Eventually construct a bounded task set.

Example:

```text
20 real software tasks
```

For each task:

```text
known repo state
clear objective
bounded allowed changes
verification command/property
frontier baseline result
```

Local-first flow:

```text
for task in tasks:
    request = CLASSIFY_TO_KNOWN_CAPABILITY(task)

    selection = SELECT_QUALIFIED_LOCAL(request)

    if no qualified candidate:
        ESCALATE_WITH_REASON("no qualified local candidate")
        continue

    result = RUN_LOCAL(selection)
    verification = VERIFY(task, result)

    if verification passes:
        record LOCAL_COMPLETION
    else:
        record LOCAL_FAILURE
        escalate according to explicit policy
```

Metrics:

```text
verified local completions
verified final completions
local attempts
bad local routing decisions
frontier escalations
escalation reasons
latency
model switches
runtime failures
contract failures
verification failures
```

The target is not maximum local usage.

The target is useful verified work with rational compute/provider cost.

---

# 37. Scope-control algorithm

Before implementing any significant idea:

```text
function should_build(feature):
    property = ASK("What property are we trying to establish?")

    if property is vague:
        return NO

    evidence = ASK("What evidence would prove or disprove it?")

    if evidence is undefined:
        return NO

    smallest_experiment = DESIGN_SMALLEST_EXPERIMENT(property, evidence)

    if feature is substantially larger than smallest_experiment:
        return REDUCE_SCOPE

    if feature consumes unresolved future decision:
        return DEFER

    if feature creates abstraction for only hypothetical implementation:
        return DEFER

    return BUILD_SMALLEST_EXPERIMENT
```

Use this especially when tempted by:

```text
plugin systems
distributed queues
agents
multi-runtime interfaces
native bridges
databases
autonomous repo tooling
fancy TUI
frontier routing
```

---

# 38. Milestone closeout pseudocode

For each meaningful milestone:

```text
function close_milestone(milestone):
    report OBJECTIVE
    report PROPERTY_WE_NEEDED
    report WHAT_WE_BUILT
    report VALIDATION_COMMANDS
    report OBSERVED_OUTPUT
    report KNOWN_FAILURE_MODES_TESTED
    report WHAT_IS_PROVEN
    report WHAT_IS_NOT_PROVEN
    report WHAT_OPERATOR_SHOULD_NOW_UNDERSTAND
    report SMALLEST_NEXT_DECISION
```

The closeout is not complete if `OBSERVED_OUTPUT` is replaced with:

```text
"tests should pass"
```

or:

```text
"the code looks correct"
```

---

# 39. Learning-agent interaction loop

The conversational agent guiding development should behave approximately like this:

```text
while project_not_finished:
    current = READ_CURRENT_REPO_STATE()
    milestone = IDENTIFY_CURRENT_MILESTONE(current)

    explain mental model required for next tiny step

    ask operator to predict important behavior

    request one small implementation/action

    give exact validation command

    STOP

    wait for operator to return actual output

    compare:
        prediction
        expected property
        observed output

    if mismatch:
        do not immediately rewrite everything

        identify:
            boundary
            evidence
            plausible causes
            discriminating next test

        continue debugging

    if property proven:
        record checkpoint
        choose next small step
```

The agent should not run ten milestones ahead because the roadmap exists.

---

# 40. Current starting point

At repository bootstrap, the only implementation milestone authorized by this document is:

```text
M0
```

Expected first implementation state should remain tiny.

Likely sequence:

```text
1. clone/open repo
2. inspect README + pseudocode
3. confirm Go installation/version
4. create Go module
5. create smallest `package main`
6. run it
7. build it
8. inspect binary/process behavior
9. add one small version/argument behavior
10. add the smallest meaningful test if appropriate
11. close M0 with observed evidence
```

Then:

```text
M1 = manually operate llama-server
```

Only after those are understood do we begin:

```text
Go → HTTP → llama-server
```

---

# 41. Final north star

The complete future loop is approximately:

```text
REQUEST
   │
   ▼
UNDERSTAND REQUIRED CAPABILITY
   │
   ▼
PREPARE BOUNDED CONTEXT
   │
   ▼
FIND CURRENT QUALIFIED CONFIGURATIONS
   │
   ├── none ───────────────► ESCALATE / DECLINE LOCAL
   │
   ▼
APPLY RESOURCE / LATENCY CONSTRAINTS
   │
   ▼
DETERMINISTIC SELECTION
   │
   ▼
EXECUTE
   │
   ▼
CAPTURE RAW EVIDENCE
   │
   ▼
VALIDATE CONTRACT
   │
   ▼
VERIFY TASK PROPERTY
   │
   ├── failure ────────────► RETRY ONLY IF POLICY JUSTIFIES
   │                             │
   │                             └──► OTHERWISE ESCALATE
   │
   ▼
VERIFIED LOCAL RESULT
   │
   ▼
RETAIN EVIDENCE
   │
   ▼
IMPROVE FUTURE QUALIFICATION / ROUTING KNOWLEDGE
```

The core idea remains simple:

> The system should know what local compute has actually demonstrated, use it where the evidence supports doing so, and refuse to confuse availability or model confidence with verified capability.

Everything else in this document exists to make that claim operational rather than rhetorical.
