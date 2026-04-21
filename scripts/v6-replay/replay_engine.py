"""
Verifier-side replay engine.

Given a ``BatchLog`` produced by ``WorkerSimulator``, re-execute the forward
pass on the same (or different) hardware, driven by the log's per-step batch
membership. Returns logits for a designated target task that Phase 1/2 diffs
against the Worker's original logits.
"""

from __future__ import annotations

from .replay_types import BatchLog, TaskLogits


class ReplayEngine:
    """
    Replay the exact continuous-batch schedule captured in a ``BatchLog``.

    **Determinism contract.** Same as ``WorkerSimulator`` — deterministic
    algorithms, fixed dtype, deterministic SDPA backend, identical seed. The
    engine must NOT consult any wall-clock or RNG state beyond what's
    recorded in the log; any such leak will break cross-hardware
    bit-exactness.

    **Scheduling.** Unlike ``WorkerSimulator``, the replay engine has no
    scheduler of its own — it executes the exact per-step active-task
    roster dictated by ``batch_log.steps``. A task's KV cache is built via
    prefill at the step where it first enters the active set, and freed at
    the step immediately after it exits.

    **Partial replay (future).** For verifier-cost reasons (review finding
    B1), Phase 3 may add a mode that stops replay at the step where the
    target task exits, instead of running the whole batch to completion.
    Out of scope for the Phase 1/2 PoC.
    """

    def __init__(self, model_id: str, device: str) -> None:
        self.model_id = model_id
        self.device = device

    def replay(self, batch_log: BatchLog, *, target_task_id: str) -> TaskLogits:
        """
        Replay the schedule and return logits for ``target_task_id``.

        The returned ``TaskLogits.logits`` must have the same length as
        ``batch_log.active_step_indices(target_task_id)``, and each entry
        must be the vocabulary logit vector produced at that step under the
        scheduled batch layout.

        Raises:
            ValueError: if ``target_task_id`` is not present in the log.
        """
        raise NotImplementedError(
            "Phase 1: implement log-driven replay on HuggingFace transformers "
            "with the determinism contract documented in ReplayEngine's "
            "docstring. See scripts/v6-replay/README.md §Phase 1."
        )
