import test from "node:test";
import assert from "node:assert/strict";
import { capabilityLabel, escapeHTML, formatPercent, stateClass, terminalJob } from "../dist/viewmodel.js";

test("escapeHTML prevents evidence strings becoming markup", () => {
  assert.equal(escapeHTML('<img src=x onerror="boom">'), "&lt;img src=x onerror=&quot;boom&quot;&gt;");
});

test("unknown and stale remain visually distinct", () => {
  assert.equal(capabilityLabel("UNKNOWN", "UNKNOWN"), "UNKNOWN");
  assert.equal(capabilityLabel("SUPPORTED", "STALE"), "STALE");
  assert.notEqual(stateClass("UNKNOWN"), stateClass("STALE"));
});

test("job terminal classification matches durable lifecycle", () => {
  for (const state of ["COMPLETED", "FAILED", "CANCELLED", "INTERRUPTED"]) assert.equal(terminalJob(state), true);
  for (const state of ["PLANNED", "QUEUED", "STARTING_RUNTIME", "RUNNING", "EVALUATING", "PERSISTING"]) assert.equal(terminalJob(state), false);
});

test("percent formatting does not fabricate precision", () => {
  assert.equal(formatPercent(0.9544), "95%");
  assert.equal(formatPercent(undefined), "—");
});
