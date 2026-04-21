"""
Phase 1 acceptance tests — single-machine replay bit-exact.

Each test currently fails with ``NotImplementedError`` raised by the stub
classes. Phase 1 implementation lands when all tests in this file pass.

PASS criteria are hard-coded: ``max_abs_err == 0.0`` across all comparisons.
Any non-zero drift at single-machine level indicates a determinism leak in
the implementation, not a V6 architectural claim failure. Fix before
proceeding to Phase 2.
"""

from __future__ import annotations

import numpy as np
import pytest

from .replay_engine import ReplayEngine
from .replay_types import BatchLog
from .worker_simulator import WorkerSimulator

# Match C0 report's baseline (see docs/testing/reports/2026-04-20-1329-c0-fail).
MODEL = "Qwen/Qwen2.5-3B-Instruct"
DEVICE = "cuda"
PROMPTS = {
    "task-p1-001": "Write a short sentence about the night sky:",
    "task-p1-002": "List the first three primary colors:",
    "task-p1-003": "How many sides does a hexagon have?",
    "task-p1-004": "What is the capital of France?",
}
SAMPLING = dict(max_new_tokens=10, temperature=0.7, top_p=0.9, seed=42)


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
