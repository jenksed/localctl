# LocalCTL Baseline Lab

LocalCTL now has two surfaces:

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

The lab surface makes local-model experimentation easy. The runtime surface remains available so the systems behavior is never hidden.

## First five commands

```bash
./localctl check
./localctl models
./localctl exercises
./localctl baseline granite
./localctl runs
```

`granite` is an example model reference. LocalCTL accepts a unique substring of a discovered GGUF artifact name, so references such as `granite`, `ministral`, or `ornith` work when they are unambiguous on the machine.

## What a baseline run does

`localctl baseline <model>`:

1. discovers the requested GGUF artifact;
2. starts a managed `llama-server` if one is not already running for that artifact;
3. waits for runtime readiness;
4. runs the core baseline exercise set;
5. captures runtime measurements exposed by the completion response;
6. evaluates deterministic exercises;
7. marks subjective exercises as pending instead of inventing a score;
8. persists every observation;
9. stops the runtime if the baseline command started it.

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

`show` reconstructs the useful context: configuration, prompt, response, deterministic evaluation, and any later human judgment.

## Compare models from saved evidence

After running the same baseline against multiple models:

```bash
./localctl baseline granite
./localctl baseline ministral
./localctl compare granite ministral
```

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
5. predict whether your chosen model will pass
6. localctl exercise run go-slice-alias-v1 granite
7. inspect the saved run
8. localctl baseline granite
9. repeat with a second model
10. localctl compare granite ministral
11. inspect failures instead of only reading the score
12. judge a subjective coding/writing exercise
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

The current catalog is a starting measurement surface. It helps expose differences and generates comparable evidence. Over time, stronger workloads should be added where success can be checked by compilers, tests, schemas, or other external properties.

LocalCTL remains responsible for discovering and recording runtime reality. Qualification, routing, authorization, and higher-level intelligence remain outside this Go layer.
