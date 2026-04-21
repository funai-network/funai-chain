"""
Phase 2 acceptance tests — cross-hardware replay bit-exact.

These tests require artifacts produced by a Worker run on a DIFFERENT GPU
than the one running pytest. Invocation:

    # on machine A (Worker, e.g. T4):
    python -m scripts.v6_replay.worker_simulator \
        --model Qwen/Qwen2.5-3B-Instruct \
        --emit-log /tmp/phase2-run.pkl
    scp /tmp/phase2-run.pkl machineB:/tmp/
    # on machine B (Replayer, e.g. RTX 5090):
    pytest -v scripts/v6_replay/test_phase2.py \
        --worker-artifact=/tmp/phase2-run.pkl

A hook for the ``--worker-artifact`` flag lives in Phase 1's conftest (to be
added alongside the CLI entry point).

PASS criteria: ``max_abs_err == 0.0`` across all comparisons, both
directions (A→B and B→A). Any non-zero drift → KILL V6 (A2 claim refuted),
fall back to Option B.
"""

from __future__ import annotations

import os
import pickle

import numpy as np
import pytest

from .replay_engine import ReplayEngine
from .replay_types import BatchLog, TaskLogits

MODEL = "Qwen/Qwen2.5-3B-Instruct"
DEVICE = "cuda"


@pytest.fixture(scope="module")
def worker_artifact():
    """
    Load a Worker-side batch log + logits pickled on a different machine.

    Skips Phase 2 cleanly if no artifact was provided — Phase 2 is only
    meaningful when cross-hardware data is available, and CI cannot run it.
    """
    path = os.environ.get("V6_PHASE2_ARTIFACT")
    if not path:
        pytest.skip(
            "V6_PHASE2_ARTIFACT env var not set — provide a pickled "
            "(prompts, log, worker_logits) triple produced on a different GPU"
        )
    with open(path, "rb") as f:
        prompts, log, worker_logits = pickle.load(f)
    assert isinstance(log, BatchLog)
    return prompts, log, worker_logits


def test_replay_is_bit_exact_cross_gpu(worker_artifact, target):
    # ``target`` is parametrized at collection time by ``pytest_generate_tests``
    # below, reading the artifact's prompt set from V6_PHASE2_ARTIFACT.
    """
    Load-bearing Phase 2 assertion.

    Worker ran on machine A. We are running on machine B (different GPU).
    For every ``target`` present in the Worker's prompt set, the replay
    engine's logits must match the Worker's logits bit-exactly.
    """
    prompts, log, worker_logits = worker_artifact

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
            f"Phase 2 KILL: target={target} step={i} max_abs_err={diff:g} "
            f"— V6 A2 (cross-hardware bit-exact) claim refuted"
        )


def pytest_generate_tests(metafunc):
    """Collect targets from the artifact, once, at collection time."""
    if "target" not in metafunc.fixturenames:
        return
    path = os.environ.get("V6_PHASE2_ARTIFACT")
    if not path or not os.path.isfile(path):
        metafunc.parametrize("target", ["__noartifact__"])
        return
    with open(path, "rb") as f:
        prompts, _log, _wl = pickle.load(f)
    metafunc.parametrize("target", list(prompts))
