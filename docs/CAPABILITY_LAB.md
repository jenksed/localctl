# LocalCTL Capability Lab

This document describes the broader real-world capability surface built on top of LocalCTL's fast 18-exercise baseline.

The objective is not to produce a single benchmark score. It is to discover the **edge of usefulness** for a local model on this machine:

```text
What can I delegate locally with high confidence?
What can I use locally if I review the answer?
What tasks waste more time than they save?
Which model/configuration is strongest for each workload class?
How stable are those conclusions over repeated runs and runtime changes?
```

## Fast baseline versus focused suites

Keep the core baseline fast:

```bash
./localctl baseline granite
```

Use focused suites when you want deeper evidence:

```bash
./localctl suite coding granite
./localctl suite developer granite
./localctl suite summarization granite
./localctl suite linux granite
./localctl suite docker granite
./localctl suite kubernetes granite
```

The infrastructure investigation pack contains exactly 100 scenarios:

```text
Linux        34
Docker       33
Kubernetes   33
----------------
Total       100
```

These mix deterministic questions with harder human-judged investigation plans.

## Real-world scenario areas

The full catalog now includes work such as:

```text
coding semantics and debugging
Go concurrency and process boundaries
Python / JavaScript / TypeScript / Elixir
structured JSON and API-style contracts
reasoning and resource constraints
source grounding and uncertainty
operations and failure boundaries
security judgment
data extraction and transformation
context retrieval under distractors
PR review
commit-message generation
PR descriptions
diff summaries
bug triage
regression test planning
technical summarization
incident summaries
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

List one category:

```bash
./localctl exercises linux --all
./localctl exercises developer --all
```

Inspect before running:

```bash
./localctl exercise show linux-performance-triage-v1
./localctl exercise show dev-pr-review-v1
```

## Why strict-output failures matter

A model can know the answer and still fail the task contract.

Example:

```text
Expected:
NO

Model:
NO

Explanation: ...
```

For an exercise that explicitly requires exactly `NO`, that is a failure.

New evidence distinguishes failure modes including:

```text
incorrect_answer
contract_extra_output
empty_output
invalid_json
structured_mismatch
missing_required_concepts
inference_error
```

This lets historical analysis separate:

```text
model did not know the answer
!=
model knew the answer but ignored the output contract
!=
model produced no visible answer
!=
runtime/inference itself failed
```

Older schema-v1 runs remain valid historical observations. Their detailed failure subtype is reported as legacy/unclassified because LocalCTL will not rewrite old evidence to pretend it observed facts that were not recorded then.

## Historical evidence audit

Every learner exercise, baseline scenario, freeform `try`, and direct inference against a LocalCTL-managed runtime is intended to leave evidence.

Check the evidence corpus:

```bash
./localctl evidence audit
```

The audit checks:

```text
observation files
index rows
observations missing from the index
duplicate index rows
prompt artifacts
response artifacts
human judgments
schema-version history
runs by model
```

The per-run directory remains authoritative:

```text
~/.localctl/runs/YYYY/MM/DD/<run-id>/
    observation.json
    prompt.txt
    response.txt
    judgment.json    # optional
```

`index.ndjson` is a derivative lookup/aggregation structure. If it is damaged or missing, rebuild it from the observation files:

```bash
./localctl evidence rebuild-index
```

That does **not** rewrite historical observation files.

See the evidence location:

```bash
./localctl evidence path
```

## Longitudinal intelligence

View historical capability for one installed model:

```bash
./localctl insights granite
./localctl insights ornith
```

Or all saved evidence:

```bash
./localctl insights
```

Insights currently summarize:

```text
total runs
successful inference
inference errors
auto pass / fail
manual pending
empty visible output
median elapsed time
median generation throughput
auto-scored pass rate
human judgments
pass rate by category
failure modes
evidence schema mix
```

This is where repeated daily/weekly use starts becoming knowledge instead of terminal history.

## Model discovery

Installed artifacts:

```bash
./localctl models
```

Curated candidates for the M1 Pro / 16 GB machine class:

```bash
./localctl explore
```

The curated radar intentionally distinguishes:

```text
priority       strong next candidates in the current footprint class
speed-control  smaller candidate used to find the lower useful boundary
stretch        larger candidate that may fit but needs careful memory/context testing
```

Live discovery:

```bash
./localctl explore --live
```

The live view searches recently updated Hugging Face GGUF repositories whose names suggest roughly 3B–14B scale.

Important:

```text
recent != good
repository name != exact parameter truth
model-file size != total memory use
GGUF exists != installed llama.cpp supports it
loads once != useful
useful on one workload != universally strong
```

The live radar exists to create an investigation queue, not to manufacture recommendations.

## Recommended comparison strategy

For a new model:

```text
1. confirm it loads
2. run the 18-test baseline
3. inspect failures
4. run developer suite
5. run Linux/Docker/Kubernetes suites if those matter to your work
6. run summarization/writing scenarios
7. judge the manual runs
8. repeat important suites
9. compare historical insights
```

Example:

```bash
./localctl baseline qwen
./localctl suite developer qwen
./localctl suite linux qwen
./localctl suite docker qwen
./localctl suite kubernetes qwen
./localctl insights qwen
```

Then compare against another installed model:

```bash
./localctl compare granite qwen
./localctl insights granite
./localctl insights qwen
```

## What to look for

A useful local coding/operations model is not simply the one with the largest pass rate.

Look for a workload-specific operating envelope:

```text
Correctness
  Does it identify the right problem?

Contract reliability
  Does it follow exact/JSON/output restrictions?

Grounding
  Does it invent facts not present in evidence?

Investigation quality
  Does it choose useful next observations instead of guessing causes?

Compression
  Can it summarize diffs/logs/incidents without dropping key facts?

Developer utility
  Are PR reviews, commit messages, test plans, and explanations worth using?

Latency
  Does the answer arrive quickly enough to be useful in the workflow?

Throughput
  How expensive is longer generation?

Stability
  Does the same workload succeed repeatedly?

Resource fit
  Does the model remain practical at the context size you actually need?
```

The target conclusion should be bounded:

```text
On this machine, with this artifact/runtime/configuration,
Model X has demonstrated strong evidence for workload Y
and weak evidence for workload Z.
```

Not:

```text
Model X is the best model.
```

## Infrastructure investigation suites

### Linux

The Linux pack covers process state, signals, exit codes, permissions, filesystem capacity, inodes, mounts, deleted-open files, file descriptor limits, memory pressure, OOM kills, swap, systemd/journal investigation, sockets, loopback binding, DNS, timeouts, PATH/environment behavior, and multi-subsystem latency triage.

```bash
./localctl suite linux <model>
```

### Docker

The Docker pack covers image/container boundaries, port publication, EXPOSE, bind mounts, named volumes, layer caching, ENTRYPOINT/CMD, PID 1, graceful stop, health checks, restart policies, logs/inspect, container DNS, localhost boundaries, secrets in image layers, memory/CPU limits, writable-layer persistence, `.dockerignore`, Compose readiness assumptions, multi-stage builds, networking triage, startup failure, and build optimization.

```bash
./localctl suite docker <model>
```

### Kubernetes

The Kubernetes pack covers Pod states, CrashLoopBackOff, image pulls, logs/events, Services/selectors/endpoints, ClusterIP/NodePort, readiness/liveness/startup probes, Deployments/ReplicaSets/rollouts, ConfigMaps/Secrets, resource requests/limits, OOMKilled, scheduling failures, taints/tolerations, affinity, PVCs, StatefulSets, Jobs/CronJobs, RBAC, NetworkPolicy, namespaces, Service routing, Pod readiness, and stuck rollout triage.

```bash
./localctl suite kubernetes <model>
```

## Manual tasks are not second-class evidence

PR reviews, investigation plans, summaries, support replies, and architecture tradeoffs often cannot be scored honestly with one exact string.

LocalCTL records those as `pending` until judged:

```bash
./localctl judge <run-id> good "accurate, actionable, no invented facts"
./localctl judge <run-id> partial "useful but missed the resource constraint"
./localctl judge <run-id> bad "confidently diagnosed a cause not supported by evidence"
```

Over time, those judgments are part of the model's usefulness record.

## The desired outcome

The catalog is deliberately broad because the project is trying to find a practical local-model portfolio, not crown one benchmark champion.

A plausible future result might look like:

```text
Qwen-small
  excellent exact output
  fast extraction/classification
  adequate Linux command triage
  weak PR review

Model B
  slower
  strong PR review and debugging
  reliable Kubernetes investigation

Model C
  best summarization/writing
  weak strict JSON behavior
```

That is actionable knowledge. It can later feed a higher-level Elixir system that coordinates experiments, tracks freshness, and reasons about qualified capability without turning LocalCTL itself into an orchestration framework.
