"""
Worker-side continuous-batching simulator + batch log emitter.

Phase 1 fills in the ``run_batch`` method. Contract and determinism
expectations are documented inline so any future implementer can match the
acceptance criteria in ``README.md`` §Phase 1 without re-deriving them.
"""

from __future__ import annotations

from .replay_types import BatchLog, TaskLogits


class WorkerSimulator:
    """
    Runs deterministic continuous-batched inference and emits a replayable
    schedule alongside the generated outputs.

    **Determinism contract.** The implementer must enforce all of:

    - ``torch.use_deterministic_algorithms(True)`` at init
    - ``torch.manual_seed(seed)`` and ``torch.cuda.manual_seed_all(seed)``
      at the start of each ``run_batch`` call
    - ``model.eval()`` — no dropout, no layer-norm training variance
    - Fixed dtype end-to-end (recommend ``torch.float16`` to mirror C0)
    - Deterministic SDPA backend — avoid ``flash_attention`` unless its
      version is known deterministic; ``scaled_dot_product_attention`` with
      ``enable_mem_efficient=False`` is a safe default for the PoC
    - Stable batch-membership transitions — when a task's output hits EOS,
      its slot is freed before the next step; when a new task joins, it
      always takes the lowest free slot. Replay reconstructs exactly this.

    If any of the above is skipped, Phase 1 WILL fail the ``max_abs_err ==
    0.0`` bit-exact assertion and should be reported as an implementation
    defect, not a V6 claim failure.
    """

    def __init__(self, model_id: str, device: str) -> None:
        self.model_id = model_id
        self.device = device

    def run_batch(
        self,
        task_prompts: dict[str, str],
        *,
        max_new_tokens: int,
        temperature: float,
        top_p: float,
        seed: int,
    ) -> tuple[dict[str, str], BatchLog, dict[str, TaskLogits]]:
        """
        Run a batched generation and record the schedule.

        Args:
            task_prompts: ``task_id -> prompt``. The simulator decides the
                intra-batch ordering; callers must not assume it matches
                insertion order.
            max_new_tokens: maximum output tokens per task.
            temperature, top_p: sampling params, passed through.
            seed: Worker seed. Replay uses the same seed.

        Returns:
            outputs: ``task_id -> generated text``.
            batch_log: ``BatchLog`` with per-step active-task roster; feed
                this into ``ReplayEngine.replay``.
            task_logits: ``task_id -> TaskLogits``; the Worker-side ground
                truth used to validate replay bit-exactness in Phase 1/2.
        """
        raise NotImplementedError(
            "Phase 1: implement continuous-batching on HuggingFace transformers "
            "with the determinism contract documented in WorkerSimulator's "
            "docstring. See scripts/v6-replay/README.md §Phase 1."
        )
