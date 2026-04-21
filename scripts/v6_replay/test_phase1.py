"""
Phase 1 acceptance tests — single-machine replay bit-exact.

Current scope: Phase 1a (temperature=0 argmax, fixed-batch composition).
Phases 1b (temperature>0 with ChaCha20 sampling) and 1c (dynamic batch
composition) will add tests here as each lands.

PASS criteria are hard-coded: ``max_abs_err == 0.0`` across all comparisons.
Any non-zero drift at single-machine level indicates a determinism leak in
the implementation, not a V6 architectural claim failure. Fix before
proceeding to Phase 2.

Environment overrides for quick iteration:
- ``V6_MODEL`` — defaults to Qwen2.5-3B-Instruct (C0 baseline). Swap to
  ``Qwen/Qwen2.5-0.5B-Instruct`` for a ~10× faster cycle when debugging
  infrastructure issues.
- ``V6_DEVICE`` — defaults to ``cuda``. Set ``cpu`` for laptop / CI smoke
  tests with a tiny model (determinism still verifiable, just slower).
"""

from __future__ import annotations

import os

import numpy as np
import pytest

from .replay_engine import ReplayEngine
from .replay_types import BatchLog
from .worker_simulator import WorkerSimulator

# Match C0 report's baseline by default; overridable for quick iteration.
MODEL = os.environ.get("V6_MODEL", "Qwen/Qwen2.5-3B-Instruct")
DEVICE = os.environ.get("V6_DEVICE", "cuda")
PROMPTS = {
    "task-p1-001": "Write a short sentence about the night sky:",
    "task-p1-002": "List the first three primary colors:",
    "task-p1-003": "How many sides does a hexagon have?",
    "task-p1-004": "What is the capital of France?",
}
# Phase 1a: argmax (temperature=0). Phase 1b will add a separate
# temperature>0 test block.
SAMPLING = dict(max_new_tokens=10, temperature=0.0, top_p=1.0, seed=42)


@pytest.fixture(scope="module")
def worker_run():
    """Run the Worker once; all Phase-1 tests reuse its outputs."""
    w = WorkerSimulator(MODEL, DEVICE)
    return w.run_batch(PROMPTS, **SAMPLING)


def test_worker_emits_log_and_logits(worker_run):
    outputs, log, logits = worker_run
    assert set(outputs) == set(PROMPTS), "every task must have an output"
    assert set(logits) == set(PROMPTS), "every task must have per-step logits"
    assert isinstance(log, BatchLog)
    assert log.steps, "log must contain at least one batch step"
    for task_id in PROMPTS:
        active_steps = log.active_step_indices(task_id)
        assert active_steps, f"{task_id} never appears in any step"
        assert len(logits[task_id].logits) == len(active_steps), (
            f"{task_id}: logits count ({len(logits[task_id].logits)}) must "
            f"match active-step count ({len(active_steps)})"
        )


@pytest.mark.parametrize("target", list(PROMPTS))
def test_replay_is_bit_exact_same_gpu(worker_run, target):
    """
    Load-bearing Phase 1 assertion.

    Worker's per-step logits for ``target`` must match ReplayEngine's
    per-step logits bit-exactly. Any non-zero drift → KILL Phase 1 until
    determinism defect is fixed.
    """
    _, log, worker_logits = worker_run

    r = ReplayEngine(MODEL, DEVICE)
    replayed = r.replay(log, target_task_id=target)

    w = worker_logits[target].logits
    rp = replayed.logits
    assert len(w) == len(rp), (
        f"{target}: step count differs — worker={len(w)}, replay={len(rp)}"
    )
    for i, (w_step, r_step) in enumerate(zip(w, rp)):
        diff = float(np.max(np.abs(np.asarray(w_step) - np.asarray(r_step))))
        assert diff == 0.0, (
            f"Phase 1 KILL: target={target} step={i} max_abs_err={diff:g} "
            f"— determinism defect or V6 A1 claim failure"
        )


def test_replay_three_repeats_stable(worker_run):
    """
    Running ``run_batch`` three times with the same inputs must produce
    identical logits every time. Flaky here → fix determinism before
    testing replay.
    """
    _, log, base_logits = worker_run
    w = WorkerSimulator(MODEL, DEVICE)
    for repeat in range(2):
        _, log2, logits2 = w.run_batch(PROMPTS, **SAMPLING)
        assert [s.active_task_ids for s in log2.steps] == [
            s.active_task_ids for s in log.steps
        ], f"batch schedule differs on repeat {repeat + 1}"
        for task_id in PROMPTS:
            for i, (a, b) in enumerate(
                zip(base_logits[task_id].logits, logits2[task_id].logits)
            ):
                diff = float(np.max(np.abs(np.asarray(a) - np.asarray(b))))
                assert diff == 0.0, (
                    f"repeat {repeat + 1} {task_id} step {i}: non-deterministic "
                    f"max_abs_err={diff:g}"
                )


# ─────────────────────────────────────────────────────────────────────────────
# Phase 1c — dynamic batch composition
# ─────────────────────────────────────────────────────────────────────────────
#
# Phase 1c is the load-bearing V6 validation step: tasks join and leave the
# batch at different decode steps, so `BatchLog.steps[k].active_task_ids`
# varies across k. A replay that honors the per-step roster must still
# produce bit-exact logits for every target at every step where it was
# active.
#
# Two schedule shapes cover the interesting transitions:
#   LEAVE_SCHEDULE: all tasks start at step 0; leave at different steps.
#       Exercises KV-cache-pruning equivalent scenarios.
#   JOIN_SCHEDULE: some tasks enter the batch mid-run (prefill while others
#       are already decoding). Exercises the "new task arrives" path.


# Leave-only schedule: all start at 0, staggered exits.
LEAVE_SCHEDULE = {
    "task-p1-001": (0, 4),
    "task-p1-002": (0, 6),
    "task-p1-003": (0, 8),
    "task-p1-004": (0, 10),
}

# Join+leave schedule: some tasks arrive late.
JOIN_SCHEDULE = {
    "task-p1-001": (0, 5),
    "task-p1-002": (0, 8),
    "task-p1-003": (2, 9),   # joins at step 2
    "task-p1-004": (4, 10),  # joins at step 4
}


def _assert_roster_varies(log, schedule_name: str) -> None:
    rosters = [s.active_task_ids for s in log.steps]
    assert any(r != rosters[0] for r in rosters), (
        f"{schedule_name}: composition never changes — degenerated to Phase 1a, "
        "test does not actually exercise Phase 1c"
    )


def _assert_log_matches_schedule(log, schedule, schedule_name: str) -> None:
    """Worker honours the schedule faithfully."""
    step_map = {s.step_index: set(s.active_task_ids) for s in log.steps}
    max_step = max(end for _, end in schedule.values())
    for k in range(max_step):
        expected = {
            tid for tid, (start, end) in schedule.items() if start <= k < end
        }
        got = step_map.get(k, set())
        assert got == expected, (
            f"{schedule_name}: step {k} active set mismatch — expected {expected}, got {got}"
        )


def _assert_bit_exact_per_target(schedule, worker_logits, replay_engine, log, schedule_name: str):
    for tid, (start, end) in schedule.items():
        expected_step_count = end - start
        replayed = replay_engine.replay_dynamic(log, target_task_id=tid)
        w = worker_logits[tid].logits
        rp = replayed.logits
        assert len(w) == expected_step_count, (
            f"{schedule_name} target={tid}: worker logits length "
            f"{len(w)} != schedule active steps {expected_step_count}"
        )
        assert len(rp) == len(w), (
            f"{schedule_name} target={tid}: replay length {len(rp)} != worker length {len(w)}"
        )
        for i, (w_step, r_step) in enumerate(zip(w, rp)):
            diff = float(np.max(np.abs(np.asarray(w_step) - np.asarray(r_step))))
            assert diff == 0.0, (
                f"{schedule_name} KILL: target={tid} step={i} max_abs_err={diff:g} "
                "— Phase 1c bit-exactness broken"
            )


@pytest.fixture(scope="module")
def leave_worker_run():
    """Worker runs the leave-only schedule once; all Phase-1c-leave tests reuse."""
    w = WorkerSimulator(MODEL, DEVICE)
    return w.run_batch_dynamic(
        PROMPTS,
        LEAVE_SCHEDULE,
        temperature=0.0,
        top_p=1.0,
        seed=42,
    )


@pytest.fixture(scope="module")
def join_worker_run():
    w = WorkerSimulator(MODEL, DEVICE)
    return w.run_batch_dynamic(
        PROMPTS,
        JOIN_SCHEDULE,
        temperature=0.0,
        top_p=1.0,
        seed=42,
    )


def test_1c_leave_roster_varies(leave_worker_run):
    _, log, _ = leave_worker_run
    _assert_roster_varies(log, "LEAVE_SCHEDULE")


def test_1c_leave_log_matches_schedule(leave_worker_run):
    _, log, _ = leave_worker_run
    _assert_log_matches_schedule(log, LEAVE_SCHEDULE, "LEAVE_SCHEDULE")


def test_1c_leave_replay_bit_exact(leave_worker_run):
    """Load-bearing Phase 1c claim for leave-only dynamic composition."""
    _, log, worker_logits = leave_worker_run
    r = ReplayEngine(MODEL, DEVICE)
    _assert_bit_exact_per_target(LEAVE_SCHEDULE, worker_logits, r, log, "LEAVE")


def test_1c_join_roster_varies(join_worker_run):
    _, log, _ = join_worker_run
    _assert_roster_varies(log, "JOIN_SCHEDULE")


def test_1c_join_log_matches_schedule(join_worker_run):
    _, log, _ = join_worker_run
    _assert_log_matches_schedule(log, JOIN_SCHEDULE, "JOIN_SCHEDULE")


def test_1c_join_replay_bit_exact(join_worker_run):
    """Load-bearing Phase 1c claim for join+leave dynamic composition."""
    _, log, worker_logits = join_worker_run
    r = ReplayEngine(MODEL, DEVICE)
    _assert_bit_exact_per_target(JOIN_SCHEDULE, worker_logits, r, log, "JOIN")


# ─────────────────────────────────────────────────────────────────────────────
# Phase 1b — temperature > 0 with ChaCha20 sampling, dynamic batch
# ─────────────────────────────────────────────────────────────────────────────
#
# Phase 1b composes on top of Phase 1c: the Worker uses the same dynamic
# batch path (join+leave schedule) but samples tokens via V5.2 §9.3's
# ChaCha20-seeded inverse CDF instead of argmax. Bit-exactness now has two
# components:
#
#   1. Logits bit-exact (inherited from Phase 1c's numerical determinism).
#   2. Sampled tokens identical between Worker and Replayer (because both
#      apply the same chacha20_sample to the same logits with the same
#      per-task final_seed + per-step nonce).
#
# The "not-argmax" guard below verifies that sampling actually diverges
# from argmax — without it, temperature=0.7 could degenerate to argmax by
# coincidence on a small test set and we'd be re-testing Phase 1c.


PHASE1B_SAMPLING = dict(
    temperature=0.7,
    top_p=0.9,
    seed=42,
)


@pytest.fixture(scope="module")
def phase1b_worker_run():
    """Worker runs JOIN_SCHEDULE under temperature=0.7 once; tests reuse."""
    w = WorkerSimulator(MODEL, DEVICE)
    return w.run_batch_dynamic(PROMPTS, JOIN_SCHEDULE, **PHASE1B_SAMPLING)


def test_1b_sampling_not_argmax(phase1b_worker_run):
    """Guard: sampling must actually diverge from argmax somewhere, else
    temperature=0.7 has coincidentally reduced to greedy and this test
    stops exercising ChaCha20."""
    _, _, worker_logits = phase1b_worker_run
    saw_divergence = False
    for tl in worker_logits.values():
        for i, (tok, logit_vec) in enumerate(zip(tl.sampled_tokens, tl.logits)):
            if tok != int(np.argmax(np.asarray(logit_vec))):
                saw_divergence = True
                break
        if saw_divergence:
            break
    assert saw_divergence, (
        "Phase 1b test degenerated: all sampled tokens matched argmax. "
        "ChaCha20 sampling path is not being exercised — increase "
        "temperature, expand prompts, or widen top_p."
    )


@pytest.mark.parametrize("target", list(PROMPTS))
def test_1b_replay_is_bit_exact_logits(phase1b_worker_run, target):
    """Logits must remain bit-exact under temperature > 0 (sampling only
    affects which token is chosen, not the logits that feed it)."""
    _, log, worker_logits = phase1b_worker_run
    r = ReplayEngine(MODEL, DEVICE)
    replayed = r.replay_dynamic(log, target_task_id=target)
    w = worker_logits[target].logits
    rp = replayed.logits
    assert len(w) == len(rp), (
        f"{target}: step count differs — worker={len(w)}, replay={len(rp)}"
    )
    for i, (w_step, r_step) in enumerate(zip(w, rp)):
        diff = float(np.max(np.abs(np.asarray(w_step) - np.asarray(r_step))))
        assert diff == 0.0, (
            f"Phase 1b KILL (logits): target={target} step={i} max_abs_err={diff:g}"
        )


@pytest.mark.parametrize("target", list(PROMPTS))
def test_1b_replay_sampled_tokens_match(phase1b_worker_run, target):
    """Load-bearing Phase 1b assertion: Worker and Replayer agree on every
    ChaCha20-sampled token, at every step the target was active."""
    _, log, worker_logits = phase1b_worker_run
    r = ReplayEngine(MODEL, DEVICE)
    replayed = r.replay_dynamic(log, target_task_id=target)
    assert worker_logits[target].sampled_tokens == replayed.sampled_tokens, (
        f"Phase 1b KILL (sampling): target={target} "
        f"worker={worker_logits[target].sampled_tokens} != replay={replayed.sampled_tokens}"
    )


# ─────────────────────────────────────────────────────────────────────────────
# Phase 1d — robustness sweep + KT v2 §2.5 engine metadata
# ─────────────────────────────────────────────────────────────────────────────
#
# Phase 1a-1c established determinism under small, narrow inputs. Phase 1d
# pushes the scale along each axis KT v2 §7 identified as a potential
# failure source and checks the bit-exactness claim still holds:
#
#   §7.1 scheduler hidden state     → larger batch (12 tasks)
#   §7.2 chunked-prefill boundaries → longer prompts (~300 tokens)
#   §7.3 join/leave boundaries      → multi-phase dynamic schedule
#   §7.5 VRAM-feasibility sanity    → long generation (30 steps)
#   §2.5 engine pinning             → mismatch-rejection guard
#
# A PASS extends the operating envelope under which V6 A1 is known to
# hold on this hardware; a FAIL pinpoints the exact scale at which
# determinism leaks, which would be a useful diagnostic before Phase 2
# cross-hardware.


def test_1d_engine_mismatch_rejected():
    """KT v2 §2.5 — Replayer must refuse a log whose engine metadata does
    not match its own runtime. Guard test: tamper with engine_version,
    expect ValueError."""
    w = WorkerSimulator(MODEL, DEVICE)
    small_prompts = dict(list(PROMPTS.items())[:2])
    small_schedule = {tid: (0, 3) for tid in small_prompts}
    _, log, _ = w.run_batch_dynamic(
        small_prompts, small_schedule, temperature=0.0, top_p=1.0, seed=42,
    )
    # Tamper. Worker's real version is correct; we simulate a Verifier
    # running an older/newer transformers that the Worker was not told
    # about.
    log.engine_version = "9.99.99-bogus"
    r = ReplayEngine(MODEL, DEVICE)
    with pytest.raises(ValueError, match="engine_version"):
        r.replay_dynamic(log, target_task_id=list(small_prompts)[0])


# Large-batch test — 12 tasks, fixed composition, 10 steps.
LARGE_PROMPTS = {
    f"task-1d-large-{i:02d}": f"Question {i}. Please give a very short answer: what is {i} plus {i}?"
    for i in range(12)
}
LARGE_SCHEDULE = {tid: (0, 10) for tid in LARGE_PROMPTS}


@pytest.fixture(scope="module")
def large_batch_run():
    w = WorkerSimulator(MODEL, DEVICE)
    return w.run_batch_dynamic(
        LARGE_PROMPTS, LARGE_SCHEDULE, temperature=0.0, top_p=1.0, seed=42,
    )


def test_1d_large_batch_bit_exact(large_batch_run):
    """12 tasks concurrent. If scheduler hidden state (KT v2 §7.1) depends
    on batch size, we'd see drift here but not in the 4-task tests."""
    _, log, worker_logits = large_batch_run
    r = ReplayEngine(MODEL, DEVICE)
    # Sample a subset of targets to keep runtime bounded — 3 is enough.
    for target in list(LARGE_PROMPTS)[:3]:
        replayed = r.replay_dynamic(log, target_task_id=target)
        w_logits = worker_logits[target].logits
        rp_logits = replayed.logits
        assert len(w_logits) == len(rp_logits)
        for i, (ws, rs) in enumerate(zip(w_logits, rp_logits)):
            diff = float(np.max(np.abs(np.asarray(ws) - np.asarray(rs))))
            assert diff == 0.0, (
                f"Phase 1d KILL (large batch): target={target} step={i} "
                f"max_abs_err={diff:g}"
            )


# Long-prompt test — 4 tasks, ~300-token prompts.
_FILLER = (
    "The quick brown fox jumps over the lazy dog and then the slow "
    "purple cat reads a long book about the history of mathematics, "
    "including the work of Euclid, Newton, Leibniz, Gauss, Riemann, and "
    "Euler. " * 6
)
LONG_PROMPTS = {
    f"task-1d-long-{i:02d}": f"{_FILLER}Question {i}: what is the sum of {i} and {i + 1}?"
    for i in range(4)
}
LONG_SCHEDULE = {tid: (0, 10) for tid in LONG_PROMPTS}


@pytest.fixture(scope="module")
def long_prompt_run():
    w = WorkerSimulator(MODEL, DEVICE)
    return w.run_batch_dynamic(
        LONG_PROMPTS, LONG_SCHEDULE, temperature=0.0, top_p=1.0, seed=42,
    )


def test_1d_long_prompts_bit_exact(long_prompt_run):
    """~300-token prompts. If chunked-prefill split points (KT v2 §7.2)
    aren't deterministic we'd see drift. transformers doesn't chunk
    prefill by default, so this is more of a sanity check that long
    contexts don't trip some other overflow path."""
    _, log, worker_logits = long_prompt_run
    r = ReplayEngine(MODEL, DEVICE)
    for target in list(LONG_PROMPTS)[:2]:
        replayed = r.replay_dynamic(log, target_task_id=target)
        w_logits = worker_logits[target].logits
        rp_logits = replayed.logits
        for i, (ws, rs) in enumerate(zip(w_logits, rp_logits)):
            diff = float(np.max(np.abs(np.asarray(ws) - np.asarray(rs))))
            assert diff == 0.0, (
                f"Phase 1d KILL (long prompts): target={target} step={i} "
                f"max_abs_err={diff:g}"
            )


# Long-generation test — 4 tasks, 30 max_new_tokens, fixed composition.
LONG_GEN_SCHEDULE = {tid: (0, 30) for tid in PROMPTS}


@pytest.fixture(scope="module")
def long_gen_run():
    w = WorkerSimulator(MODEL, DEVICE)
    return w.run_batch_dynamic(
        PROMPTS, LONG_GEN_SCHEDULE, temperature=0.0, top_p=1.0, seed=42,
    )


def test_1d_long_generation_bit_exact(long_gen_run):
    """30 decode steps. Probes whether error accumulates across many
    steps even under determinism — shouldn't, but if it does, we'd
    want to know before Phase 2."""
    _, log, worker_logits = long_gen_run
    r = ReplayEngine(MODEL, DEVICE)
    for target in list(PROMPTS)[:2]:
        replayed = r.replay_dynamic(log, target_task_id=target)
        w_logits = worker_logits[target].logits
        rp_logits = replayed.logits
        assert len(w_logits) == 30 and len(rp_logits) == 30
        for i, (ws, rs) in enumerate(zip(w_logits, rp_logits)):
            diff = float(np.max(np.abs(np.asarray(ws) - np.asarray(rs))))
            assert diff == 0.0, (
                f"Phase 1d KILL (long gen): target={target} step={i} "
                f"max_abs_err={diff:g}"
            )


# Complex-schedule test — 6 tasks, rosters change at steps 3, 5, 7, 8.
COMPLEX_PROMPTS = {
    f"task-1d-complex-{i:02d}": f"Trivia {i}: what colour is the sky on a clear day?"
    for i in range(6)
}
COMPLEX_SCHEDULE = {
    "task-1d-complex-00": (0, 10),  # runs full length
    "task-1d-complex-01": (0, 8),   # leaves at 8
    "task-1d-complex-02": (0, 5),   # leaves at 5
    "task-1d-complex-03": (3, 10),  # joins at 3
    "task-1d-complex-04": (5, 10),  # joins at 5 (same step 02 leaves)
    "task-1d-complex-05": (7, 10),  # joins at 7
}


@pytest.fixture(scope="module")
def complex_schedule_run():
    w = WorkerSimulator(MODEL, DEVICE)
    return w.run_batch_dynamic(
        COMPLEX_PROMPTS, COMPLEX_SCHEDULE, temperature=0.0, top_p=1.0, seed=42,
    )


def test_1d_complex_schedule_bit_exact(complex_schedule_run):
    """Four composition changes across 10 steps (join at 3, leave+join at 5,
    join at 7, leave at 8). Stress-tests KT v2 §7.3 join/leave boundary
    handling under denser transitions than JOIN_SCHEDULE."""
    _, log, worker_logits = complex_schedule_run
    r = ReplayEngine(MODEL, DEVICE)
    # Check every target, since the point of this test is schedule density.
    for target in COMPLEX_PROMPTS:
        replayed = r.replay_dynamic(log, target_task_id=target)
        expected_steps = (
            COMPLEX_SCHEDULE[target][1] - COMPLEX_SCHEDULE[target][0]
        )
        w_logits = worker_logits[target].logits
        rp_logits = replayed.logits
        assert len(w_logits) == expected_steps
        assert len(rp_logits) == expected_steps
        for i, (ws, rs) in enumerate(zip(w_logits, rp_logits)):
            diff = float(np.max(np.abs(np.asarray(ws) - np.asarray(rs))))
            assert diff == 0.0, (
                f"Phase 1d KILL (complex schedule): target={target} step={i} "
                f"max_abs_err={diff:g}"
            )
