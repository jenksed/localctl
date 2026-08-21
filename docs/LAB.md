# LocalCTL Baseline Lab

LocalCTL has two surfaces:

```text
learner lab                 glass-box runtime
-----------                 -----------------
localctl check              localctl runtime start
localctl models             localctl runtime stop
localctl exercises          localctl runtime status
localctl exercise ...       localctl runtime inspect
localctl try ...            localctl runtime infer ...
localctl baseline ...
localctl runs
localctl show ...
localctl judge ...
localctl compare ...
```

The lab surface makes repeated local-model experimentation easy. The runtime surface remains available when the learner wants to inspect process/runtime mechanics directly.

## Runtime behavior: keep the current model warm

Lab commands use a sticky managed runtime.

```text
baseline granite
    -> start Granite if needed
    -> run exercises
    -> KEEP Granite running

exercise run ... granite
    -> reuse the same Granite runtime

baseline granite
    -> reuse the same Granite runtime

baseline ornith
    -> stop Granite
    -> start Ornith
    -> run exercises
    -> KEEP Ornith running
```

You do not need to manually stop a model before requesting another model from `exercise`, `try`, or `baseline`.

A model switch is explicit in the output:

```text
runtime: switching granite-...gguf -> ornith-...gguf
runtime stopped
pid: ...
runtime started
pid: ...
```

If LocalCTL finds recorded runtime state but the runtime is no longer ready, the next lab command attempts to reconcile that state automatically before starting the requested model.

Manual lifecycle control is still available when you deliberately want it:

```bash
./localctl runtime status
./localctl runtime inspect
./localctl runtime stop
```

## First five commands

```bash
./localctl check
./localctl models
./localctl exercises
./localctl baseline granite
./localctl runs
```

`granite` is an example model reference. LocalCTL accepts a unique substring of a discovered GGUF artifact name, so references such as `granite`, `ministral`, or `ornith` work when they are unambiguous on the machine.

`localctl models` marks the active managed model with `*` when one is ready.

## What a baseline run does

`localctl baseline <model>`:

1. discovers the requested GGUF artifact;
2. reuses the current managed runtime when it already has that artifact loaded;
3. automatically switches runtimes when a different model is requested;
4. reconciles stale managed state when possible;
5. waits for runtime readiness when a runtime must be started;
6. runs the selected exercise set;
7. captures runtime measurements exposed by the completion response;
8. evaluates deterministic exercises;
9. marks subjective exercises as pending instead of inventing a score;
10. persists every observation;
11. leaves the selected runtime ready for the next lab command.

The extended catalog is available with:

```bash
./localctl exercises --all
./localctl baseline granite --all
```

A category can be isolated with:

```bash
./localctl exercises coding
./localctl baseline granite --category=coding
```

## Reading baseline output

PASS rows stay compact:

```text
[ 1/18] exact-output-v1                 PASS     375ms
```

FAIL rows explain the failure and preserve the exact run for deeper inspection:

```text
[ 4/18] json-extraction-v1              FAIL     2.2s
        why: response was not valid JSON: ...
        got: "..."
        saved: run_...
```

At the end, LocalCTL reports the overall pass rate, auto-scored pass rates by category, failed exercise IDs, and how to inspect the saved evidence.

The compact display is for navigation. The evidence store remains authoritative.

## Exercise catalog

The built-in catalog intentionally mixes different kinds of work.

### Coding

Examples include:

- Go defer ordering
- Go slice aliasing
- map comma-ok behavior
- compiler-error diagnosis
- channel deadlock reasoning
- JSON tag repair
- Unix pipeline reasoning
- SQL generation
- process-identity code review
- small boundary/refactoring judgment

### Structured output

Examples include:

- exact JSON objects
- fact extraction into JSON
- sorted JSON arrays
- strict output contracts

### Reasoning

Examples include:

- arithmetic
- hard-constraint elimination
- ordering logic
- sequence completion
- evidence-versus-judgment distinctions

### Extraction and analysis

Examples include:

- multi-fact extraction
- bounded classification
- contradiction detection
- source-grounding discipline
- failure-boundary classification
- uncertainty discipline

### Writing and planning

Examples include:

- faithful summarization
- concise rewriting
- technical explanation
- bounded planning tradeoffs

These are marked for human judgment when no honest deterministic validator exists.

## Evaluation modes

LocalCTL currently uses four explicit evaluation modes:

```text
exact
contains_all
json_exact
manual
```

`exact`, `contains_all`, and `json_exact` produce deterministic pass/fail evidence.

`manual` produces:

```text
pending
```

until a person records a judgment.

That separation is deliberate:

```text
model produced text
!=
text was correct
```

## Freeform use also becomes evidence

```bash
./localctl try granite "Explain why process started is not the same as server ready."
```

A freeform run is saved exactly like a baseline run, but its evaluation remains pending until judged.

Because the runtime stays warm, several freeform prompts against the same model avoid repeated model startup cost.

## Judge a run

```bash
./localctl judge <run-id> good "accurate and concise"
./localctl judge <run-id> partial "correct result but missed the race"
./localctl judge <run-id> bad "plausible diagnosis contradicted the source"
```

Human judgment is stored separately from the machine observation. Revising a judgment never rewrites what actually happened.

## Evidence storage

Runs are stored under:

```text
~/.localctl/runs/
```

A run directory contains:

```text
observation.json
prompt.txt
response.txt
judgment.json      # only after human judgment
```

An append-only index is maintained at:

```text
~/.localctl/runs/index.ndjson
```

`observation.json` is schema-versioned and records facts such as:

- run identity and timestamps
- exercise identity/category/difficulty
- model artifact identity/path/size
- runtime kind and URL
- requested context/temperature/max tokens
- inference status
- finish reason
- token usage when reported
- latency
- prompt and generation throughput when reported
- deterministic evaluation result

Unknown runtime metrics remain absent/zero rather than being fabricated.

## Browse history

```bash
./localctl runs
./localctl show <run-id>
```

`show` reconstructs configuration, prompt, response, deterministic evaluation detail, and any later human judgment.

## Compare models from saved evidence

After running comparable baselines:

```bash
./localctl baseline granite
./localctl baseline ministral
./localctl compare granite ministral
```

The second baseline automatically switches the managed runtime.

The comparison reports observed run counts, auto-scored pass rates, pending manual judgments, median latency, median generation throughput when available, and per-category auto-scored pass rates.

The command deliberately ends with:

```text
These are observed results from saved runs, not a universal model ranking.
```

That is a contract, not decoration.

## Learning path

A useful first session is:

```text
1. localctl check
2. localctl models
3. localctl exercises
4. localctl exercise show go-slice-alias-v1
5. predict whether the chosen model will pass
6. localctl exercise run go-slice-alias-v1 granite
7. run another exercise immediately; Granite is still warm
8. localctl baseline granite
9. localctl baseline ministral (automatic switch)
10. localctl compare granite ministral
11. inspect failures instead of only reading the score
12. judge a subjective coding/writing exercise
13. localctl runtime stop when the lab session is actually finished
```

The intended learning loop remains:

```text
prediction
    -> experiment
    -> evidence
    -> comparison
    -> updated mental model
```

## What the baseline does not prove

A high pass rate does not prove that a model is generally good at coding or reasoning.

A low pass rate also needs investigation. Failure can come from wrong reasoning, instruction-following failure, output-format mismatch, truncation, or a weakness in the exercise itself. Inspect the saved response before turning a FAIL into a broad model claim.

The current catalog is a starting measurement surface. It helps expose differences and generates comparable evidence. Over time, stronger workloads should be added where success can be checked by compilers, tests, schemas, or other external properties.

LocalCTL remains responsible for discovering and recording runtime reality. Qualification, routing, authorization, and higher-level intelligence remain outside this Go layer.
