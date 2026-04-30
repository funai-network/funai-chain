package keeper_test

// Tests for KT 30-case Issue 5 residual sub-items (5.2 + 5.3 + 5.4).
// PR #48 closed Issue 5.1 (sha256(ActualContent) == ContentHash binding).
// This file pins the remaining three:
//
//   5.2  FraudProof requires the task have a chain footprint (SettledTask,
//        SecondVerificationPending, or FrozenBalanceKey). Pre-fix, any random
//        taskId paired with captured (ContentHash, sig) + a registered worker
//        address could slash that worker for a task the chain knows nothing
//        about.
//
//   5.3  When a SettledTask exists, msg.WorkerAddress MUST match
//        settledTask.WorkerAddress. Pre-fix, an attacker could quote
//        (TaskId, ContentHash, sig) from worker A's settled task but submit
//        with msg.WorkerAddress = worker B → slash B for A's task.
//
//   5.4  Fraud mark is terminal. After SetFraudMark, settleAuditedTask /
//        ProcessFraudProof / ProcessBatchSettlement must skip the task.

import (
	"crypto/sha256"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/settlement/types"
)

// ============================================================
// Issue 5.2 — chain footprint required.
// ============================================================

func TestKT_Issue5_2_FraudProof_RejectsTaskWithNoChainFootprint(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	worker := makeAddr("kt-i52-worker")
	taskId := []byte("kt-i52-no-footprint01")

	// Build a properly-signed fraud proof — but the keeper has NO record of
	// this taskId (no SettledTask, no pending audit, no FrozenBalanceKey).
	contentHash, contentSig := signFraudContent(t, []byte("any-content"))
	msg := &types.MsgFraudProof{
		Reporter:         makeAddr("kt-i52-rep").String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("any-content"),
	}

	err := k.ProcessFraudProof(ctx, msg)
	if err == nil {
		t.Fatal("Issue 5.2: must reject FraudProof for taskId with no chain footprint")
	}
	if k.HasFraudMark(ctx, taskId) {
		t.Fatal("Issue 5.2: fraud mark must NOT be set when chain has no footprint")
	}
}

func TestKT_Issue5_2_FraudProof_AcceptsWhenSettledTaskExists(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	worker := makeAddr("kt-i52-worker-2")
	taskId := []byte("kt-i52-with-settled1")

	// Pre-populate a SettledTask record matching this worker — provides the
	// chain footprint, and SettledTask.WorkerAddress matches msg.WorkerAddress
	// (5.3 satisfied).
	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		WorkerAddress: worker.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	contentHash, contentSig := signFraudContent(t, []byte("c"))
	msg := &types.MsgFraudProof{
		Reporter:         makeAddr("kt-i52-rep-2").String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("c"),
	}
	if err := k.ProcessFraudProof(ctx, msg); err != nil {
		t.Fatalf("Issue 5.2: with SettledTask footprint must succeed: %v", err)
	}
	if !k.HasFraudMark(ctx, taskId) {
		t.Fatal("Issue 5.2: fraud mark must be set on accepted proof")
	}
}

func TestKT_Issue5_2_FraudProof_AcceptsWhenAuditPending(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	worker := makeAddr("kt-i52-worker-3")
	taskId := []byte("kt-i52-pending-audit1")

	// Pre-populate a SecondVerificationPending — pre-settle fraud window.
	k.SetSecondVerificationPending(ctx, types.SecondVerificationPendingTask{
		TaskId:        taskId,
		WorkerAddress: worker.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	contentHash, contentSig := signFraudContent(t, []byte("d"))
	msg := &types.MsgFraudProof{
		Reporter:         makeAddr("kt-i52-rep-3").String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("d"),
	}
	if err := k.ProcessFraudProof(ctx, msg); err != nil {
		t.Fatalf("Issue 5.2: with pending audit must succeed: %v", err)
	}
}

// ============================================================
// Issue 5.3 — worker address must match settled task.
// ============================================================

func TestKT_Issue5_3_FraudProof_RejectsWrongWorkerAddress(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	realWorker := makeAddr("kt-i53-real-worker")
	otherWorker := makeAddr("kt-i53-other-worker")
	taskId := []byte("kt-i53-cross-worker01")

	// SettledTask records realWorker as the executor.
	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		WorkerAddress: realWorker.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	// Attacker submits with otherWorker's address — must be rejected.
	contentHash, contentSig := signFraudContent(t, []byte("any"))
	msg := &types.MsgFraudProof{
		Reporter:         makeAddr("kt-i53-attacker").String(),
		TaskId:           taskId,
		WorkerAddress:    otherWorker.String(), // wrong worker
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("any"),
	}
	err := k.ProcessFraudProof(ctx, msg)
	if err == nil {
		t.Fatal("Issue 5.3: must reject FraudProof when msg.WorkerAddress != settledTask.WorkerAddress")
	}
	if k.HasFraudMark(ctx, taskId) {
		t.Fatal("Issue 5.3: fraud mark must NOT be set when worker binding fails")
	}
}

// ============================================================
// Issue 5.4 — fraud mark is terminal.
// ============================================================

// TestKT_Issue5_4_FraudMark_BlocksLaterAuditedSettle simulates the audit-vs-fraud
// race: a task's SecondVerificationPending exists when ProcessFraudProof
// arrives. After the fraud mark is set, late audit completion (3 results
// arriving) MUST NOT rewrite the SettledTask state back from TaskFraud to
// TaskSettled / TaskFailSettled. This is observable through the public
// ProcessSecondVerificationResult API — settleAuditedTask is package-private.
func TestKT_Issue5_4_FraudMark_BlocksLaterAuditedSettle(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)
	k.SetCurrentThirdVerificationRate(ctx, 0)

	user := makeAddr("kt-i54-user")
	worker := makeAddr("kt-i54-worker")
	taskId := []byte("kt-i54-fraud-then-aud")
	_ = k.ProcessDeposit(ctx, user, sdk.NewCoin("ufai", math.NewInt(10_000_000)))

	// Step 1: SettledTask record exists (TaskSettled) AND a SecondVerificationPending
	// is in flight — the race window where audit results may arrive after a
	// FraudProof tx lands.
	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(1_000_000)),
	})
	k.SetSecondVerificationPending(ctx, types.SecondVerificationPendingTask{
		TaskId:            taskId,
		OriginalStatus:    types.SettlementSuccess,
		SubmittedAt:       ctx.BlockHeight(),
		UserAddress:       user.String(),
		WorkerAddress:     worker.String(),
		VerifierAddresses: []string{makeAddr("kt-i54-orig-v1").String()},
		Fee:               sdk.NewCoin("ufai", math.NewInt(1_000_000)),
		ExpireBlock:       10000,
	})

	// Step 2: SDK reports fraud — fraud mark set, status flipped to TaskFraud.
	contentHash, contentSig := signFraudContent(t, []byte("c"))
	if err := k.ProcessFraudProof(ctx, &types.MsgFraudProof{
		Reporter:         makeAddr("kt-i54-rep").String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("c"),
	}); err != nil {
		t.Fatalf("FraudProof must succeed for matched binding: %v", err)
	}
	post1, _ := k.GetSettledTask(ctx, taskId)
	if post1.Status != types.TaskFraud {
		t.Fatalf("after FraudProof status must be TaskFraud, got %s", post1.Status)
	}

	// Step 3: 3 late-arriving audit responses → triggers processAuditJudgment
	// → settleAuditedTask. With the fraud mark set, settleAuditedTask must
	// short-circuit and the SettledTask MUST stay at TaskFraud (not flip
	// back to TaskSettled or TaskFailed).
	for i := 0; i < 3; i++ {
		_ = k.ProcessSecondVerificationResult(ctx, &types.MsgSecondVerificationResult{
			SecondVerifier: makeAddr("kt-i54-aud-vX").String(),
			TaskId:         taskId,
			Epoch:          1,
			Pass:           true,
			LogitsHash:     []byte("h"),
		})
	}

	post2, found := k.GetSettledTask(ctx, taskId)
	if !found {
		t.Fatal("SettledTask must still exist after audit completion")
	}
	if post2.Status != types.TaskFraud {
		t.Fatalf("Issue 5.4: fraud-marked task must stay TaskFraud after late audit, got %s", post2.Status)
	}
	// And the pending record should be deleted (settleAuditedTask returned
	// true via the fraud short-circuit, so processAuditJudgment cleared it).
	if _, found := k.GetSecondVerificationPending(ctx, taskId); found {
		t.Fatal("Issue 5.4: pending must be deleted after fraud-shortcut audit completion")
	}
}

// ============================================================
// Issue 5.5 — idempotent on duplicate fraud proof.
// ============================================================

func TestKT_Issue5_5_FraudProof_DuplicateIsIdempotent(t *testing.T) {
	k, ctx, _, wk := setupKeeper(t)

	worker := makeAddr("kt-i55-worker")
	taskId := []byte("kt-i55-double-fraud01")

	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		WorkerAddress: worker.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	contentHash, contentSig := signFraudContent(t, []byte("c"))
	msg := &types.MsgFraudProof{
		Reporter:         makeAddr("kt-i55-rep").String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("c"),
	}
	if err := k.ProcessFraudProof(ctx, msg); err != nil {
		t.Fatalf("first proof must succeed: %v", err)
	}
	slashCountAfterFirst := len(wk.slashCalls)

	// Second submission must short-circuit on HasFraudMark.
	if err := k.ProcessFraudProof(ctx, msg); err == nil {
		t.Fatal("Issue 5.5: duplicate FraudProof must error (idempotent reject)")
	}
	if got := len(wk.slashCalls); got != slashCountAfterFirst {
		t.Fatalf("Issue 5.5: duplicate must NOT double-slash; slash count went %d → %d",
			slashCountAfterFirst, got)
	}
}

// Compile-time guard: ensure sha256 stays used (defensive in case future
// editors strip the import while refactoring).
var _ = sha256.Sum256
