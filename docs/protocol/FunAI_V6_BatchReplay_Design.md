# V6 Batch Log-Replay Verification

## P0 — Blocks mainnet

### 1. Replay Scheduler

Add a replay mode to the Verifier's inference engine. Following the batch log the
Worker sent, insert/remove the designated request into/out of the batch at the
designated step.

**Why.** FunAI's verification scenario is: Worker runs batched inference,
Verifier verifies after the fact. The C0 test proved that logits from a Worker
at `batch=4` differ from a Verifier at `batch=1` by 2.27 %, so the verification
breaks down. An engineer has separately verified that same-batch, same-content
execution across T4 and 5090 is bit-exact (0.000000). Therefore, if the
Verifier can precisely replay the Worker's batching process, verification holds.
The replay scheduler is what lets the Verifier do that.

### 2. Worker Batch Logging

Add a hook to the Worker's scheduler that records, at every decode step, the
list and order of requests in the batch. After inference completes, the log is
P2P-broadcast to the Verifier alongside the `BatchReceipt`. Add a field on
`BatchReceipt` marking which inference batch each task belongs to.

**Why.** Under continuous batching, batch composition changes every decode step
(new requests join, finished requests leave). Without a per-step batch roster,
the Verifier cannot know how the Worker actually ran the inference, and cannot
replay precisely.

### 3. Batch-mode Dispatch

- Worker declares `batch_capacity` at registration time. No upper bound.
- Chain maintains an `in_flight` counter. Leader keeps dispatching while
  `in_flight < capacity`.
- Remove the busy/idle two-state.
- **Bundle timing lives on the Leader, not the Worker.** See item #12
  (Leader-side request bundling). The Worker runs whatever bundle the
  Leader delivers as one batch — no Worker-side `batch_wait_timeout`.

**Why.** The original one-task-at-a-time + busy state forced the Worker to
`batch=1` forever, producing GPU utilisation of only 10-20 %. Under batch mode
the Worker processes multiple tasks concurrently; throughput rises 5-10×, so
operator revenue rises materially — operators will actually show up.

No upper bound on capacity because the technology keeps advancing; a hard cap
would be outdated quickly. A Worker over-stating capacity naturally triggers
timeouts and therefore jail — the market punishes misrepresentation
automatically.

Moving the wait-window to the Leader (item #12) matters because Worker-side
waiting is only useful when tasks happen to arrive at the Worker during the
window. At low-to-medium traffic, single-dispatch-per-request means each
request arrives solo, Worker's window closes empty, and `batch=1` wins.
Leader-side bundling closes that gap.

### 4. Settlement Adjustment

- First-tier (3 verifiers) PASS → settle immediately.
- Picked by VRF for second-/third-tier verification → fee is locked until
  verification completes; the full three-round chain must finish within 24
  hours.
- `BatchReceipt` carries an inference-batch-ID field. The Proposer's
  `MsgBatchSettlement` logic itself is unchanged.

**Why.** Workers can't be made to wait for all tasks to walk through all three
rounds before getting paid — the vast majority of tasks clear after the first
verification and should be paid out immediately. Only tasks selected for
subsequent verification have their fee locked. That way Worker cash flow is
healthy and money isn't held up waiting for verification. 24 hours is a hard
cap; exceeding it means the verification chain itself is broken and needs
manual intervention.

### 12. Leader-side Request Bundling

The Leader accumulates eligible requests for up to
`leader_batch_window_ms` (default 500 ms, chain-adjustable), then
dispatches the whole bundle to one chosen Worker as a single P2P
message. A bundle closes early when it reaches the target Worker's
`batch_capacity - in_flight`.

- New Leader-side state: `pending_bundle[worker_id] = [req, req, ...]`
  plus a `bundle_start_time[worker_id]` timestamp. Both exist only in
  Leader memory; no chain-level artefact.
- `AssignTask` becomes a list — one P2P message, N tasks. Worker's ACK
  covers the entire bundle.
- Worker selection per bundle uses VRF dispatch ranking (α=1.0,
  unchanged). The Leader commits to a target Worker when the first
  eligible request lands; subsequent requests in the window join that
  Worker's bundle as long as its remaining capacity allows, otherwise
  start a new bundle for a different Worker.
- Tight-latency bypass: requests with `MaxLatencyMs <
  2 × leader_batch_window_ms` (default 1 s) skip the window entirely
  and are dispatched solo to the fastest available Worker, so SLA
  budgets aren't consumed in the bundle wait.

**Why.** Without Leader-side bundling, the Leader dispatches one request
per message and the Worker sees tasks arrive singly. Even with
Worker-side batching configured, the only way a Worker's local batch
fills is if multiple requests happen to hit its P2P queue within a
short window. Under low-to-medium traffic that almost never happens —
each request dispatches solo, the Worker runs `batch=1`, GPU
utilisation stays near the V5.2 baseline of 10-20 %, and item #3's
5-10× throughput promise doesn't materialise.

With Leader-side bundling, the batch exists as a protocol object
before it reaches the Worker. The Worker simply runs what it receives.
Under the same traffic profile that would produce `batch=1` in the
old design, the Leader now delivers e.g. `batch=4` once per 500 ms
window — filling the GPU during the 500 ms that would otherwise be
idle waiting.

**Secondary benefit — reduces review finding C2.** Worker has no
scheduling freedom over batch composition. Adversarial-partner
attacks (C2 in the ingest-time review) therefore cannot be mounted by
a malicious Worker — they would require a malicious Leader, and the
Leader is a rotating VRF-elected role with broadcast-observable
decisions, making sustained manipulation materially harder. C2 is not
eliminated (Leader collusion remains possible) but the attack surface
shrinks significantly.

**Tradeoffs.**

- Adds up to `leader_batch_window_ms` of pre-dispatch latency to every
  request that isn't tight-SLA-exempt. At the default 500 ms this is
  25 % of a 2 s TTFT budget — acceptable for most workloads and
  covered by the bypass rule for the rest.
- Leader memory cost: `O(pending_requests × serialized_size)`. At 500
  TPS and 500 ms window, the pending bundle holds ~250 requests at
  peak, negligible.
- Partitioned Leaders per `model_id × sub_topic` continue to apply
  (from V5.2 §6 leader election); bundling is per-partition, so a
  topic split doesn't break bundling.

---

## P1 — Penalty mechanism fixes

### 5. `jail_count` decays per 1000 tasks

Decay one level per 1000 tasks, rather than resetting to zero at 50 tasks.

**Why.** The original "reset at 50" rule invites cadenced cheating — cheat
once → do 50 honest tasks → `jail_count` resets → cheat again. The Worker
stays permanently at jail-count = 1 (10 min cooldown), so cheating cost is
constant. Changing to "one level decay per 1000 tasks" compresses cheating
frequency 20× and causes the penalty to escalate on repeat offence within the
window.

### 6. 3 consecutive misses trigger jail

3 consecutive timeouts / no-result deliveries → triggers progressive jail. The
progressive-jail ladder is unchanged (10 min → 1 h → permanent + 5 % slash).

**Why.** The original design deducted only reputation (-0.10) on a miss, with
no jail. A Worker that over-stated capacity could then time out many requests
in a row — reputation would slowly decline while users accumulated wait-time
and re-dispatch cost. Triggering jail after 3 consecutive misses is far
stricter than reputation alone, and escalates to permanent ban in a handful of
rounds.

### 7. No batch log = the whole batch FAILs

If the Worker fails to deliver the batch log, the Verifier cannot replay, so
every task in that batch is judged FAIL and the Worker walks the
progressive-jail ladder.

**Why.** The batch log is the Verifier's only replay input. A Worker who just
realised it was cheating (e.g. ran a smaller model) might withhold the log to
escape verification. Equating "no log" with "caught cheating" closes this free
escape hatch. The whole batch must FAIL (not just one task) because without a
log the Verifier cannot even tell which task is correct.

### 8. Verifier collective punishment

First-tier PASS but second-tier FAIL → each of the 3 first-tier verifiers
loses 2 % stake and 0.20 reputation.

**Why.** Without a penalty, the first-tier Verifier's dominant strategy is to
PASS every received verification task and skip the GPU work — there is no
downside. Collective punishment flips the expected value: skipping saves a
handful of pennies of GPU, but risks being caught by second-tier verification
at a cost of 2 % stake (hundreds to thousands of FAI). 2 % and -0.20 are
suggested values; make both chain-adjustable.

### 9. Capacity over-statement penalty

No dedicated mechanism needed. Over-statement → sustained timeouts → 3
consecutive misses → progressive jail (per #6).

**Why.** Item #6 already covers this scenario. A Worker declaring capacity
= 100 but able to run only 10 will have 90 of every 100 tasks time out, hit 3
consecutive misses almost instantly, and walk up the jail ladder to permanent
ban in a few rounds. No additional logic required.

---

## P2 — Verification logic tweaks

### 10. Logits-comparison epsilon

Keep the existing design: the model proposer measures and sets epsilon at model
registration time, stored in the model registry.

**Why.** Different models have different logit ranges — a single global
epsilon is wrong. The proposer measures across 100 prompts × 2+ GPU families ×
3 runs at model-registration time, picks P99.9, and records it. This design
was already correct.

Under batch log-replay, the expected diff is 0.000000, but the protocol
should not hard-code epsilon = 0 — this is an open-source foundational system,
and the full combinatorial space of hardware / CUDA / driver versions in the
wild is unpredictable. Letting the model proposer decide from measured data is
the most resilient approach.

### 11. ChaCha20 sampling verification restored to 100 % coverage

Batch log-replay guarantees that logits match precisely → the sampling path is
fully replayable → every `temperature > 0` task is 100 % verifiable.

**Why.** Under V5.2, only 7 % of sampled tasks could undergo ChaCha20 sampling
verification (the remaining 93 % were skipped because batch drift made logits
disagree, so sampling verification was unreliable). With batch log-replay
guaranteeing exact logits, ChaCha20 can replay the sampling path correctly on
every task. Sampling-manipulation attacks (injected advertisements,
conversational steering, content censorship) move from "undetectable on 93 %
of tasks" to "detected with precision on 100 %". This is V6's largest
security gain relative to V5.2.

---

## Deprecated

- :x: Verification proxy — superseded by batch replay; no need to force the
  Verifier path to `batch=1`.
- :x: 7 % VRF-sampled logits resubmission — with 100 % precise verification,
  no sampling fallback is needed.
- :x: Top-K rank check — precise comparison removes the need for lenient
  matching.
- :x: 48 h `TaskCache` — no Worker after-the-fact data re-submission required.
- :x: Auto-adjusted `sampling_rate` — no sampling rate to tune; every task is
  verified.
- :x: SGLang determinism mode — no dependency on a specific engine.
- :x: LLM-42 — same reason.
- :x: Hardware-partitioned subnets — batch replay is bit-exact across T4 and
  5090 already, so no partitioning needed.
- :x: Clawback — fee is locked until verification completes, then released;
  there is no "pay first, claw back later".

---

## Unchanged

- Three-tier verification architecture (first / second / third) — layered
  defence, each tier addresses different attacks.
- Progressive-jail ladder (10 min → 1 h → permanent + 5 % slash) — existing
  design is sound.
- FraudProof user reports — last-line-of-defence at the user layer.
- VRF Verifier selection — existing design is sound.
- Reputation system (miss -0.10, success +0.01, hourly decay toward 1.0) —
  existing design is sound.
- Per-model-registration epsilon set by the proposer — existing design is
  sound.
- `MsgBatchSettlement` logic — Proposer's on-chain-batch packaging remains
  unchanged.

---

**Suggestion on #6:** use a sliding window instead of strict "consecutive":
jail on `≥ 3 misses in the last 10 tasks`. This avoids tripping on occasional
1–2 misses while still catching a Worker that misses frequently. Make both
10 and 3 chain-adjustable parameters.
