# V6 Batch Log-Replay PoC

Research-stage proof-of-concept for the V6 Batch Log-Replay verification scheme
described in [`docs/protocol/FunAI_V6_BatchReplay_Design.md`](../../docs/protocol/FunAI_V6_BatchReplay_Design.md).

This directory is **not** production code. It validates the load-bearing
assumptions of V6 *before* committing engineering effort to the protocol
rewrite. Ships on a `research/v6-replay-poc` branch; only promotes to
`mainnet-readiness/*` if Phase 1 and Phase 2 both PASS.

## Purpose

V6 rests on two claims that current FunAI evidence does not yet support:

- **A1 (engineering feasibility).** Some inference runtime can be driven to
  *replay* a pre-recorded per-step batch schedule, producing the same logits
  as the original Worker that generated that schedule.
- **A2 (cross-hardware determinism).** When the same schedule is replayed on
  different GPU hardware (e.g. T4 vs RTX 5090), the logits at every decode
  step are bit-exact.

If either is false, V6's verification architecture does not work and
development should fall back to Option B (single-request teacher forcing
attached to the receipt). This PoC answers both in ~3 weeks of focused work,
before any protocol-layer code is written.

## Engine choice

HuggingFace transformers, driven manually at the token level — not TGI, not
vLLM.

Rationale:

- TGI and vLLM both hide continuous-batch scheduling inside Rust / C++ and do
  not expose a public "replay this schedule" API. Forking either adds 2–3
  months before the first cross-hardware run.
- `transformers` is pure Python and exposes `forward(input_ids,
  past_key_values=...)` directly, so implementing a deterministic manual
  scheduler is a few hundred lines.
- Throughput is 10–20× lower than TGI at runtime, which disqualifies this
  engine for production Workers — but Phase 1 and Phase 2 are not about
  throughput, they are about determinism. Production engine choice is a
  Phase 3+ decision, contingent on Phase 1/2 results.

A positive PoC does **not** prove V6 is implementable on TGI or vLLM, only
that the scheme is internally consistent. That's deliberate. If the
transformers-based PoC fails, we stop. If it passes, Phase 3 evaluates
production-engine ports.

## Phase structure

| Phase | Duration | Deliverable | Gate question |
|---|---|---|---|
| 0 | 0.5–1 d | This README + stub modules + failing tests | Is the contract definable? |
| 1 | 2–4 w | `WorkerSimulator` + `ReplayEngine` on transformers | Does same-GPU replay produce bit-exact logits? |
| 2 | 1 w | Cross-hardware run on 2 GPU instances | Does cross-hardware replay produce bit-exact logits? |
| 3+ | TBD | Protocol integration only if Phases 1–2 PASS | — |

## Acceptance criteria

Each phase has a **hard PASS condition** and a **kill condition**. A kill
means V6 research stops — fall back to Option B and either close
`research/v6-replay-poc` or reframe it as a negative-result writeup.

### Phase 0 — Scaffold (this PR)

**PASS when:**
- `scripts/v6_replay/` directory exists with all files listed below
- `pytest scripts/v6_replay/` runs and fails with clear `NotImplementedError`
  messages referencing the methods that still need to be written
- Python syntax is clean (`python -m py_compile` on every `.py`)

**Kill:** N/A. Phase 0 is mechanical.

### Phase 1 — Single-machine replay bit-exact

**Method.**
- Pick model `Qwen/Qwen2.5-3B-Instruct` FP16 (same baseline as C0 — avoids
  introducing a new hardware-availability gate).
- Run `WorkerSimulator.run_batch(prompts=[P1..P4], temperature=0.7,
  max_new_tokens=10)` once. Capture `(outputs, per-prompt logits at every
  position, batch_log)`.
- Call `ReplayEngine.replay(batch_log, target_task_id=P1.task_id)`. Capture
  the replayed logits for P1 at every position.
- Diff the Worker's per-position logits for P1 against the replayed logits.

**PASS condition.** `max_abs_err == 0.0` exactly, across all output positions
of all 4 target prompts (re-run targeting each of P1..P4), across 3 repeated
runs of the whole procedure. 12 Worker-vs-Replay comparisons, zero drift.

**INVESTIGATE (not PASS, not KILL).** `max_abs_err ∈ (0, 1e-6]`. Suggests
that implementation has a minor non-determinism leak — fix before
proceeding to Phase 2. Likely causes: `torch.use_deterministic_algorithms`
not set, dropout active in eval mode, PyTorch random state not seeded.

**KILL.** `max_abs_err > 1e-6` and not fixable by deterministic-flag tuning,
OR replay output text differs from Worker output text for the same seed.
Means the manual-scheduler abstraction does not capture enough state to
reproduce the batched execution. V6 dies; report findings; fall back to
Option B.

### Phase 2 — Cross-hardware replay bit-exact

**Method.**
- Two GPU machines provisioned via `scripts/tgi-bootstrap-aliyun.sh` (or
  equivalent) — one T4-class, one 5090/4090-class, different SM architectures
  deliberately. Record driver / CUDA / torch versions for both.
- On machine A: run `WorkerSimulator.run_batch` → record `batch_log` and
  `reference_logits`.
- Transfer `batch_log` + prompts to machine B.
- On machine B: run `ReplayEngine.replay(batch_log)` → capture
  `replayed_logits`.
- Diff `reference_logits` vs `replayed_logits`.

**PASS condition.** `max_abs_err == 0.0` exactly, across all output positions
of all 4 targets, across both (A→B) and (B→A) directions, across 3 repeats
each. 24 cross-machine comparisons, zero drift.

**INVESTIGATE.** `max_abs_err ∈ (0, 1e-6]`. Document; this may still be
acceptable if a chain-level ε_floor of 1e-6 is tolerable (matches Option B's
ε_floor). Not a PASS in the strict sense but not a KILL either.

**KILL.** `max_abs_err > 1e-6`. Means cross-hardware determinism claim A2
is false; V6 assumption breaks. Stop V6; Option B remains the answer.

### Phase 3 gate (non-goal for this PoC)

Do not start Phase 3 until both Phase 1 and Phase 2 are PASS (or mutually
acceptable INVESTIGATE). The gate is binary: pass or fall back.

## Output format

Each phase run emits a `verdict.json` colocated with its artifacts, following
the C0 report convention ([`docs/testing/reports/2026-04-20-1329-c0-fail/verdict.json`](../../docs/testing/reports/2026-04-20-1329-c0-fail/verdict.json)):

```json
{
  "result": "PASS" | "INVESTIGATE" | "KILL",
  "phase": 1 | 2,
  "stats": {
    "targets": 4,
    "positions_per_target": 10,
    "repeats": 3,
    "comparisons": 12,
    "max_abs_err": 0.0,
    "max_rel_err": 0.0,
    "mismatching_positions": 0
  },
  "thresholds": { "pass": 0.0, "investigate": 1e-06 },
  "config": { ... },
  "artifacts_dir": "scripts/v6_replay/results/phaseN-<timestamp>/"
}
```

## Directory layout

```
scripts/v6_replay/
├── README.md              # this file
├── requirements.txt       # torch, transformers, numpy, pytest
├── types.py               # BatchStep, BatchLog dataclasses
├── worker_simulator.py    # WorkerSimulator.run_batch
├── replay_engine.py       # ReplayEngine.replay
├── test_phase1.py         # Phase 1 acceptance test
├── test_phase2.py         # Phase 2 acceptance test
└── results/               # phase run outputs (gitignored, created at run time)
```

## What this PoC does not cover

- **Item 3 (batch-mode dispatch), Item 4 (settlement adjustments), Items
  5–9 (penalty mechanics), Item 11 (ChaCha20 100 %)** — all protocol-layer.
  Out of scope until Phase 3 gates.
- **BatchReceipt wire format.** The PoC uses Python dataclasses; a real
  chain-level `BatchReceipt` gets defined in Phase 3 once we know the log
  shape / size is actually tractable.
- **Log-forgery defence (review finding C1).** The PoC trusts `batch_log`
  inputs unconditionally — fine at research stage. Phase 3 must reject
  `batch_log` entries whose `task_id` does not resolve to a real on-chain
  `InferRequest`.
- **Adversarial-partner injection defence (review finding C2).** Same — out
  of scope for the PoC.
- **Verifier compute amplification (review finding B1).** The PoC records
  replay cost but does not explore fee-distribution changes.

See [`docs/protocol/FunAI_V6_BatchReplay_Design.md`](../../docs/protocol/FunAI_V6_BatchReplay_Design.md)
for the full design; this README only covers what Phase 0–2 verify.

## Running the PoC

Phase 0 only exists to gate the contract. There is nothing to run yet except
the failing test skeletons:

```bash
cd scripts/v6_replay
pip install -r requirements.txt
pytest -v test_phase1.py  # all fail with NotImplementedError — expected
```

Phase 1 implementation fills in `WorkerSimulator` and `ReplayEngine`. Phase 1
acceptance run:

```bash
pytest -v test_phase1.py  # all pass when Phase 1 complete
```

Phase 2 acceptance run (requires 2 GPU machines, see acceptance criteria §
Phase 2 above):

```bash
# on machine A
python -m scripts.v6_replay.worker_simulator --emit-log phase2-run1.json
# transfer phase2-run1.json to machine B
# on machine B
pytest -v test_phase2.py --log=phase2-run1.json
```

## Related

- V6 design note: [`docs/protocol/FunAI_V6_BatchReplay_Design.md`](../../docs/protocol/FunAI_V6_BatchReplay_Design.md)
- C0 report (this PoC's motivation): [`docs/testing/reports/2026-04-20-1329-c0-fail/report.md`](../../docs/testing/reports/2026-04-20-1329-c0-fail/report.md)
- C0 test script (reference style for acceptance scripts): [`scripts/c0-logits-consistency.py`](../c0-logits-consistency.py)
