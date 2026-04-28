"""
Phase 1 MoE bit-exactness tests.

Run on a Mixture-of-Experts model (Mixtral 8x7B, Qwen-MoE, DeepSeek-MoE,
Phi-3.5-MoE, …). The question this test answers:

    Under V6's batch-replay protocol, does an MoE model produce
    bit-exact logits + bit-exact expert routing on Worker and Verifier
    when (a) the batch composition truly changes step to step (V6's
    distinctive claim) and (b) sampling is non-greedy (Companion's
    real production setting)?

A non-zero ``max_abs_err`` here means MoE introduces non-determinism that
batch logging alone does not cover. Two diagnoses:

    Path 1 — gating non-determinism: the router's softmax output picks
             different top-k experts on the replay side. Mitigation:
             record top-k expert IDs in the batch log and force the
             Replayer to follow them (not implemented yet; this test
             surfaces whether the mitigation is needed).

    Path 2 — expert internal compute drift: same as the dense-model
             case, addressed by the existing batch-replay path.

The test produces both signals so the team can tell the two apart from
a single run instead of needing a second ablation.

Coverage in this file
---------------------
- **Dynamic schedule** — `DYNAMIC_SCHEDULE` has 4 tasks with different
  `(start, end)` windows, so the active roster genuinely changes step
  to step (joins at step 2 and 4, leaves at step 7 and 9). The
  2026-04-27 RunPod report's overstated "dynamic batch" claim came
  from a schedule of `{tid: (0, 10) for tid}`, which produced static
  rosters every step. This file fixes that by exercising real V6
  Phase 1c semantics.
- **Two sampling configs** — greedy (`temperature=0`) and ChaCha20
  (`temperature=0.7`). The latter is the Companion business default
  and was never validated on MoE prior to this file.

Hardware notes
--------------
- Mixtral-8x7B-Instruct-v0.1 in bfloat16 needs ≈ 94 GB VRAM. Single
  4090 (24 GB) is OOM. Either (a) use AWQ quantization (the autoawq
  load path is now supported by `_common.load_model_and_tokenizer`),
  (b) use multi-GPU with ``device_map="auto"``, (c) downscale.
- Smaller MoE alternatives that fit a single 4090 24 GB in bfloat16 are
  rare; ``Qwen/Qwen1.5-MoE-A2.7B`` (~14 B total / 2.7 B active) is
  ≈ 28 GB bf16 and still tight. AWQ variants drop ≈ 25 GB → 4090 OK.
- Recommended runtimes:
    - Single A100/H100/H200 80GB+:  Mixtral 8x7B bf16
    - 1 × RTX PRO 6000 96GB:  Phi-3.5-MoE 42 B bf16 (top-k=2 sensitivity case)
    - 1 × L20 / RTX A6000 48GB:  Qwen1.5-MoE-A2.7B bf16
    - 1 × 4090 24GB:  Mixtral-AWQ or Qwen1.5-MoE-AWQ (with autoawq installed)

Usage
-----
::

    V6_MODEL=mistralai/Mixtral-8x7B-Instruct-v0.1 \\
    V6_DEVICE=cuda \\
    pytest scripts/v6_replay/test_phase1_moe.py -v

If the chosen ``V6_MODEL`` is not actually MoE (no router_logits surfaced
by the model), this test is skipped — keeping the dense-model Phase 1
suite unaffected.
"""

from __future__ import annotations

import gc
import os

import numpy as np
import pytest
import torch

from ._common import is_moe_model
from .replay_engine import ReplayEngine
from .worker_simulator import WorkerSimulator

MODEL = os.environ.get("V6_MODEL", "")
DEVICE = os.environ.get("V6_DEVICE", "cuda")
PROMPTS = {
    "task-moe-001": "Write a short sentence about the night sky:",
    "task-moe-002": "List the first three primary colors:",
    "task-moe-003": "How many sides does a hexagon have?",
    "task-moe-004": "What is the capital of France?",
}

# True dynamic-batch schedule: 4 tasks with distinct (start, end) windows
# so the active roster changes step to step. Concretely:
#   step 0-1  → {001, 002}                        (2 active)
#   step 2-3  → {001, 002, 003}                   (003 joins)
#   step 4-6  → {001, 002, 003, 004}              (004 joins)
#   step 7-8  → {001,      003, 004}              (002 leaves)
#   step 9    → {001,      003     }              (004 leaves)
# This exercises Phase 1c's load-bearing property (composition changes
# mid-batch and replay must reproduce per-step rosters bit-exact). The
# 2026-04-27 RunPod report's `{tid: (0, 10) for tid}` schedule was
# functionally static and missed this entirely.
DYNAMIC_SCHEDULE = {
    "task-moe-001": (0, 10),
    "task-moe-002": (0, 7),
    "task-moe-003": (2, 10),
    "task-moe-004": (4, 9),
}

SAMPLING_GREEDY = dict(temperature=0.0, top_p=1.0, seed=42)
SAMPLING_CHACHA = dict(temperature=0.7, top_p=0.9, seed=42)


def _resolve_skip_reason() -> str | None:
    if not MODEL:
        return (
            "V6_MODEL not set; specify a Mixture-of-Experts model id "
            "(e.g. V6_MODEL=Qwen/Qwen1.5-MoE-A2.7B)"
        )
    return None


# Memory layout strategy
# ----------------------
# `worker_runs` loads a Worker, executes BOTH sampling configs against it,
# then explicitly releases the Worker's GPU memory before returning. The
# `replay_engine` fixture is module-scoped and depends on `worker_runs`, so
# pytest is forced to evaluate `worker_runs` (which frees Worker) before
# instantiating Replayer. Net: only one model copy is resident on the GPU
# at a time. This is what makes Phi-3.5-MoE (~84 GB bf16) testable on a
# single RTX PRO 6000 96GB — Worker + Replayer concurrent would peak near
# 168 GB and OOM.
@pytest.fixture(scope="module")
def worker_runs():
    reason = _resolve_skip_reason()
    if reason:
        pytest.skip(reason)
    w = WorkerSimulator(MODEL, DEVICE)
    if not is_moe_model(w.model):
        pytest.skip(
            f"V6_MODEL={MODEL!r} loaded but is not Mixture-of-Experts; "
            f"this test is MoE-specific. For dense models use test_phase1.py."
        )
    greedy = w.run_batch_dynamic(PROMPTS, DYNAMIC_SCHEDULE, **SAMPLING_GREEDY)
    chacha = w.run_batch_dynamic(PROMPTS, DYNAMIC_SCHEDULE, **SAMPLING_CHACHA)

    # Drop Worker before Replayer loads. The data we returned is already
    # on the CPU (numpy arrays + Python ints), so the model's weights and
    # any cached activations can go.
    del w
    gc.collect()
    if torch.cuda.is_available():
        torch.cuda.empty_cache()

    return greedy, chacha


@pytest.fixture(scope="module")
def moe_run_greedy(worker_runs):
    """Worker output for the greedy (temperature=0) configuration."""
    return worker_runs[0]


@pytest.fixture(scope="module")
def moe_run_chacha(worker_runs):
    """Worker output for the ChaCha20-sampled (temperature=0.7) configuration.

    This path covers V5.2 §9.3 ChaCha20-seeded sampling on MoE — the
    actual Companion-default sampling on the actual MoE production
    architecture. Was not validated by the 2026-04-27 RunPod report.
    """
    return worker_runs[1]


@pytest.fixture(scope="module")
def replay_engine(worker_runs):
    """Replayer instance, loaded *after* `worker_runs` has freed the
    Worker's GPU memory. Module-scoped so all parametrized replay tests
    share one ReplayEngine instead of reloading a multi-GB model per test.
    """
    # The dependency on `worker_runs` is what enforces lifecycle ordering —
    # pytest must produce `worker_runs` first, and that fixture body
    # releases Worker before returning. Keep the parameter name even
    # though the value isn't used directly.
    _ = worker_runs
    return ReplayEngine(MODEL, DEVICE)


def _is_truly_dynamic(batch_log) -> bool:
    """Return True if the captured BatchLog's per-step active roster
    actually varies across steps (i.e. the test is exercising Phase 1c
    semantics, not degenerating to static composition)."""
    rosters = {tuple(s.active_task_ids) for s in batch_log.steps}
    return len(rosters) > 1


# ── Schedule sanity ──────────────────────────────────────────────────────────


def test_schedule_is_truly_dynamic(moe_run_greedy):
    """Lock in that DYNAMIC_SCHEDULE actually varies the active roster.

    If a future refactor accidentally collapses the schedule back to
    `{tid: (0, N) for tid in PROMPTS}` (every task active every step),
    this test fails loudly — preventing a silent regression to the
    static-composition mistake the 2026-04-27 RunPod report made.
    """
    _, log, _ = moe_run_greedy
    assert _is_truly_dynamic(log), (
        "BatchLog rosters are identical across steps — DYNAMIC_SCHEDULE "
        "has been collapsed to static composition. Phase 1c is no longer "
        "tested. Restore distinct (start, end) windows per task."
    )


# ── Greedy (temperature = 0) — Phase 1c on MoE ──────────────────────────────


def test_worker_emits_expert_routing_greedy(moe_run_greedy):
    _, _, task_logits = moe_run_greedy
    for tid, tl in task_logits.items():
        # Some tasks may be inactive at step 0; the greedy fixture's
        # logits/routing are sized to active-step count. Empty list means
        # the model is MoE but transformers did not surface router data.
        assert tl.expert_routing, (
            f"{tid}: expert_routing is empty — model loaded as MoE but "
            f"the worker did not capture top-k expert IDs. Either "
            f"transformers version mismatch or a model family whose "
            f"router_logits surface differently (e.g. DeepSeek-V2)."
        )
        assert len(tl.expert_routing) == len(tl.logits), (
            f"{tid}: expert_routing length ({len(tl.expert_routing)}) "
            f"must equal logits length ({len(tl.logits)})"
        )


@pytest.mark.parametrize("target", list(PROMPTS))
def test_replay_logits_bit_exact_moe_greedy(moe_run_greedy, replay_engine, target):
    """Phase 1c logits assertion under greedy decoding. PASS = max_abs_err 0.0."""
    _, log, worker_logits = moe_run_greedy
    replayed = replay_engine.replay_dynamic(log, target_task_id=target)

    w = worker_logits[target].logits
    rp = replayed.logits
    assert len(w) == len(rp), (
        f"{target}: step count differs — worker={len(w)}, replay={len(rp)}"
    )
    for i, (w_step, r_step) in enumerate(zip(w, rp)):
        diff = float(np.max(np.abs(np.asarray(w_step) - np.asarray(r_step))))
        assert diff == 0.0, (
            f"Phase 1c MoE greedy KILL: target={target} step={i} "
            f"max_abs_err={diff:g}. Inspect expert-routing diff "
            f"(test_replay_expert_routing_bit_exact_moe_greedy) to "
            f"distinguish Path 1 from Path 2."
        )


@pytest.mark.parametrize("target", list(PROMPTS))
def test_replay_expert_routing_bit_exact_moe_greedy(moe_run_greedy, replay_engine, target):
    """Phase 1c expert-routing assertion under greedy decoding."""
    _, log, worker_logits = moe_run_greedy
    replayed = replay_engine.replay_dynamic(log, target_task_id=target)

    w_routing = worker_logits[target].expert_routing
    r_routing = replayed.expert_routing
    assert len(w_routing) == len(r_routing), (
        f"{target}: routing step count differs — "
        f"worker={len(w_routing)}, replay={len(r_routing)}"
    )

    mismatches: list[tuple[int, int, list[int], list[int]]] = []
    for step_i, (w_step, r_step) in enumerate(zip(w_routing, r_routing)):
        assert len(w_step) == len(r_step), (
            f"{target} step={step_i}: layer count differs — "
            f"worker={len(w_step)}, replay={len(r_step)}"
        )
        for layer_i, (w_top, r_top) in enumerate(zip(w_step, r_step)):
            if sorted(w_top) != sorted(r_top):
                mismatches.append((step_i, layer_i, w_top, r_top))

    assert not mismatches, (
        f"Phase 1c MoE Path 1 hit (greedy): target={target} — gating "
        f"selected different experts on replay at {len(mismatches)} "
        f"(step, layer) positions. First 5: {mismatches[:5]}. "
        f"Path 1 mitigation (force-routing in Replayer) is now needed."
    )


# ── ChaCha20 (temperature = 0.7) — Phase 1b ⊗ 1c on MoE ─────────────────────


def test_chacha20_actually_diverges_from_argmax(moe_run_greedy, moe_run_chacha):
    """Sanity: ChaCha20 sampling must produce different sampled tokens
    than greedy on at least one task / step. Otherwise the temperature>0
    fixture is silently degenerating to argmax (e.g. due to a bug that
    overrides sampling) and the rest of this section is not actually
    testing the sampling path."""
    _, _, greedy_logits = moe_run_greedy
    _, _, chacha_logits = moe_run_chacha
    diverged = False
    for tid in PROMPTS:
        g = greedy_logits[tid].sampled_tokens
        c = chacha_logits[tid].sampled_tokens
        if g != c:
            diverged = True
            break
    assert diverged, (
        "ChaCha20 sampling produced the same tokens as argmax across all "
        "4 tasks — sampling path is not actually exercised. Confirm "
        "temperature=0.7 reaches the sampler and ChaCha20 stream is seeded."
    )


@pytest.mark.parametrize("target", list(PROMPTS))
def test_replay_logits_bit_exact_moe_chacha(moe_run_chacha, replay_engine, target):
    """Phase 1b ⊗ 1c logits assertion under ChaCha20 sampling."""
    _, log, worker_logits = moe_run_chacha
    replayed = replay_engine.replay_dynamic(log, target_task_id=target)

    w = worker_logits[target].logits
    rp = replayed.logits
    assert len(w) == len(rp), (
        f"{target}: step count differs — worker={len(w)}, replay={len(rp)}"
    )
    for i, (w_step, r_step) in enumerate(zip(w, rp)):
        diff = float(np.max(np.abs(np.asarray(w_step) - np.asarray(r_step))))
        assert diff == 0.0, (
            f"Phase 1b ⊗ 1c MoE KILL (ChaCha20): target={target} step={i} "
            f"max_abs_err={diff:g}. Sampling path on MoE drifts under replay."
        )


@pytest.mark.parametrize("target", list(PROMPTS))
def test_replay_sampled_tokens_bit_exact_moe_chacha(moe_run_chacha, replay_engine, target):
    """Phase 1b ⊗ 1c sampled-token assertion. The ChaCha20 stream + same
    logits should produce the same sampled token; if not, the sampler
    itself is non-deterministic on Replayer."""
    _, log, worker_logits = moe_run_chacha
    replayed = replay_engine.replay_dynamic(log, target_task_id=target)

    w_tok = worker_logits[target].sampled_tokens
    r_tok = replayed.sampled_tokens
    assert w_tok == r_tok, (
        f"Phase 1b ⊗ 1c sampler KILL: target={target} — "
        f"worker={w_tok} vs replay={r_tok}"
    )


@pytest.mark.parametrize("target", list(PROMPTS))
def test_replay_expert_routing_bit_exact_moe_chacha(moe_run_chacha, replay_engine, target):
    """Phase 1b ⊗ 1c expert-routing assertion under ChaCha20 sampling.

    Routing is determined by the model's gating network on the prompt +
    sampled-token prefix. Under deterministic sampling (which ChaCha20
    is by construction), routing should match Worker and Replayer
    bit-exact. A failure here with logits also failing means MoE +
    sampling exposes a routing non-determinism not seen in greedy mode.
    """
    _, log, worker_logits = moe_run_chacha
    replayed = replay_engine.replay_dynamic(log, target_task_id=target)

    w_routing = worker_logits[target].expert_routing
    r_routing = replayed.expert_routing
    assert len(w_routing) == len(r_routing), (
        f"{target}: routing step count differs — "
        f"worker={len(w_routing)}, replay={len(r_routing)}"
    )
    mismatches: list[tuple[int, int, list[int], list[int]]] = []
    for step_i, (w_step, r_step) in enumerate(zip(w_routing, r_routing)):
        for layer_i, (w_top, r_top) in enumerate(zip(w_step, r_step)):
            if sorted(w_top) != sorted(r_top):
                mismatches.append((step_i, layer_i, w_top, r_top))
    assert not mismatches, (
        f"Phase 1b ⊗ 1c Path 1 hit: target={target} ChaCha20 — gating "
        f"diverged at {len(mismatches)} (step, layer) positions. First 5: "
        f"{mismatches[:5]}."
    )
