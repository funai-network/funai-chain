package keeper_test

// Tests for KT 30-case Issues 5 + 8 (settlement keeper layer).
// (Issue 10 is exercised in p2p/proposer; tests live with that package.)
//
// Issue 5  FraudProof bind check.
//          Pre-Phase-2: kept ActualContent on-chain and required
//          sha256(ActualContent) == ContentHash + Worker sig over ContentHash.
//          That semantic only proved Worker had signed *something*, not that
//          Worker had committed to two contradicting hashes — any valid
//          (content, sig) tuple from honest delivery passed it. FraudProof
//          Phase 2 replaces the schema with a two-signature contradiction
//          proof (ReceiptResultHash + WorkerReceiptSig vs ReceivedOutputHash
//          + WorkerContentSig, both task-bound, must differ). Tests below
//          pin the Phase 2 rejection paths and the success path; the prior
//          Issue-5 sub-items 5.2 / 5.3 / 5.4 / 5.5 (footprint, worker
//          binding, terminality, idempotency) live in
//          kt_fraud_binding_residual_test.go and remain in force.
//
// Issue 8  distributeSuccessFee / distributeFailFee must dedup verifiers.
//          Pre-fix the verifier pool was split by len(verifiers) and paid
//          per-row. A duplicate-row malformed batch (same address in two
//          slots) double-paid that address.

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/settlement/types"
)

// ============================================================
// Issue 5 (Phase 2) — receipt-vs-content contradiction proof.
// ============================================================

// Phase 2 rejects when both halves agree (no contradiction). Both signatures
// validate, but the proof asserts no fraud occurred.
func TestKT_Issue5_FraudProofPhase2_RejectsAgreement(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("kt-i5p2-user")
	worker := makeAddr("kt-i5p2-worker")
	taskId := []byte("kt-i5p2-agree-task01")
	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		SettledAt:     ctx.BlockHeight(),
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	// Two halves identical — Worker honestly delivered what it committed to.
	sameHash := []byte("0123456789abcdef0123456789abcdef") // 32 bytes, content hash + result hash both = this
	receiptSig, contentSig := signFraudPair(t, taskId, sameHash, sameHash)

	msg := &types.MsgFraudProof{
		Reporter:           user.String(),
		TaskId:             taskId,
		WorkerAddress:      worker.String(),
		ReceiptResultHash:  sameHash,
		WorkerReceiptSig:   receiptSig,
		ReceivedOutputHash: sameHash,
		WorkerContentSig:   contentSig,
	}
	if err := k.ProcessFraudProof(ctx, msg); err == nil {
		t.Fatal("Phase 2: keeper MUST reject FraudProof when receipt hash == received hash (no contradiction)")
	}
	if k.HasFraudMark(ctx, taskId) {
		t.Fatal("Phase 2: fraud mark must NOT be set when both halves agree")
	}
}

// Phase 2 rejects when WorkerReceiptSig is invalid (e.g. forged or signed
// over the wrong payload). Without a valid receipt-side sig the chain has
// no proof Worker ever committed to the receipt result hash.
func TestKT_Issue5_FraudProofPhase2_RejectsInvalidReceiptSig(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("kt-i5p2-bad-receipt-user")
	worker := makeAddr("kt-i5p2-bad-receipt-worker")
	taskId := []byte("kt-i5p2-bad-receipt-01")
	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		SettledAt:     ctx.BlockHeight(),
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	receiptHash := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	receivedHash := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	_, contentSig := signFraudPair(t, taskId, receiptHash, receivedHash)

	msg := &types.MsgFraudProof{
		Reporter:           user.String(),
		TaskId:             taskId,
		WorkerAddress:      worker.String(),
		ReceiptResultHash:  receiptHash,
		WorkerReceiptSig:   []byte("forged-sig-not-from-worker"),
		ReceivedOutputHash: receivedHash,
		WorkerContentSig:   contentSig,
	}
	if err := k.ProcessFraudProof(ctx, msg); err == nil {
		t.Fatal("Phase 2: keeper MUST reject when WorkerReceiptSig fails verification")
	}
	if k.HasFraudMark(ctx, taskId) {
		t.Fatal("Phase 2: fraud mark must NOT be set when receipt sig is invalid")
	}
}

// Phase 2 rejects when WorkerContentSig is invalid. Symmetric to the
// receipt-sig case: chain refuses to slash without proof Worker signed the
// delivery hash.
func TestKT_Issue5_FraudProofPhase2_RejectsInvalidContentSig(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("kt-i5p2-bad-content-user")
	worker := makeAddr("kt-i5p2-bad-content-worker")
	taskId := []byte("kt-i5p2-bad-content-01")
	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		SettledAt:     ctx.BlockHeight(),
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	receiptHash := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	receivedHash := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	receiptSig, _ := signFraudPair(t, taskId, receiptHash, receivedHash)

	msg := &types.MsgFraudProof{
		Reporter:           user.String(),
		TaskId:             taskId,
		WorkerAddress:      worker.String(),
		ReceiptResultHash:  receiptHash,
		WorkerReceiptSig:   receiptSig,
		ReceivedOutputHash: receivedHash,
		WorkerContentSig:   []byte("forged-content-sig"),
	}
	if err := k.ProcessFraudProof(ctx, msg); err == nil {
		t.Fatal("Phase 2: keeper MUST reject when WorkerContentSig fails verification")
	}
	if k.HasFraudMark(ctx, taskId) {
		t.Fatal("Phase 2: fraud mark must NOT be set when content sig is invalid")
	}
}

// Phase 2 rejects a cross-task replay attempt: a sig captured from task A
// (which signed sha256(taskA || hash)) cannot be re-presented as a sig over
// sha256(taskB || hash) — the task_id binding makes the replayed sig fail
// verification.
func TestKT_Issue5_FraudProofPhase2_RejectsCrossTaskReplay(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("kt-i5p2-replay-user")
	worker := makeAddr("kt-i5p2-replay-worker")
	taskA := []byte("kt-i5p2-replay-task-A1")
	taskB := []byte("kt-i5p2-replay-task-B2")
	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskB,
		Status:        types.TaskSettled,
		SettledAt:     ctx.BlockHeight(),
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	receiptHash := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	receivedHash := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	// Captured for task A — these sigs are only valid against taskA's payload.
	receiptSigA, contentSigA := signFraudPair(t, taskA, receiptHash, receivedHash)

	// Reporter tries to weaponise the captured sigs against task B.
	msg := &types.MsgFraudProof{
		Reporter:           user.String(),
		TaskId:             taskB,
		WorkerAddress:      worker.String(),
		ReceiptResultHash:  receiptHash,
		WorkerReceiptSig:   receiptSigA,
		ReceivedOutputHash: receivedHash,
		WorkerContentSig:   contentSigA,
	}
	if err := k.ProcessFraudProof(ctx, msg); err == nil {
		t.Fatal("Phase 2: cross-task sig replay must be rejected by task_id binding in canonical payload")
	}
	if k.HasFraudMark(ctx, taskB) {
		t.Fatal("Phase 2: fraud mark must NOT be set on cross-task replay")
	}
}

// Phase 2 success path: two distinct hashes, both validly signed by Worker
// over the task-bound canonical payload, yields a fraud mark + slash.
func TestKT_Issue5_FraudProofPhase2_AcceptsValidContradiction(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	user := makeAddr("kt-i5p2-success-user")
	worker := makeAddr("kt-i5p2-success-worker")
	taskId := []byte("kt-i5p2-success-task01")
	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		SettledAt:     ctx.BlockHeight(),
		WorkerAddress: worker.String(),
		UserAddress:   user.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	if err := k.ProcessFraudProof(ctx, makePhase2FraudMsg(t, user.String(), worker.String(), taskId)); err != nil {
		t.Fatalf("Phase 2: valid contradiction proof must succeed, got %v", err)
	}
	if !k.HasFraudMark(ctx, taskId) {
		t.Fatal("Phase 2: fraud mark must be set on accepted contradiction proof")
	}
}

// ============================================================
// Issue 8 — verifier dedup in fee distribution.
// ============================================================

func TestKT_Issue8_DistributeSuccessFee_DedupesDuplicateVerifierAddresses(t *testing.T) {
	k, ctx, bk, _ := setupTrackingKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)

	user := makeAddr("kt-i8-user")
	worker := makeAddr("kt-i8-worker")
	v1 := makeAddr("kt-i8-v1")
	v2 := makeAddr("kt-i8-v2")

	// Pre-fix this 3-row batch with v1 duplicated would pay v1 = 2/3 of pool,
	// v2 = 1/3 — skewed split. Post-fix it should pay each unique address
	// half of the pool (v1 = 1/2, v2 = 1/2).
	fee := sdk.NewCoin("ufai", math.NewInt(120))
	_ = k.ProcessDeposit(ctx, user, fee)

	verifiers := []types.VerifierResult{
		{Address: v1.String(), Pass: true},
		{Address: v1.String(), Pass: true}, // duplicate
		{Address: v2.String(), Pass: true},
	}

	entries := []types.SettlementEntry{
		{
			TaskId: []byte("kt-i8-task-success-1"), UserAddress: user.String(),
			WorkerAddress: worker.String(), Fee: fee, ExpireBlock: 10000,
			Status: types.SettlementSuccess, VerifierResults: verifiers,
		},
	}
	msg := makeBatchMsg(t, makeAddr("kt-i8-prop").String(), entries)
	if _, err := k.ProcessBatchSettlement(ctx, msg); err != nil {
		t.Fatalf("settle: %v", err)
	}

	// Verifier pool = 120 * 120/1000 = 14 ufai. Two unique → ~7 each
	// (last gets remainder so it might be 7+7 = 14 or 7+7=14).
	gotV1 := bk.receivedBy(v1)
	gotV2 := bk.receivedBy(v2)
	totalVerifier := gotV1.Add(gotV2)

	if !totalVerifier.Equal(math.NewInt(14)) {
		t.Fatalf("Issue 8: total verifier distribution must equal pool (14), got %s (v1=%s v2=%s)",
			totalVerifier, gotV1, gotV2)
	}
	// v1 must NOT be paid double (was the bug). Per-unique split = 14/2 = 7;
	// v1 should receive 7 (not 14 - 14/3 = 9 or so).
	if !gotV1.Equal(math.NewInt(7)) {
		t.Fatalf("Issue 8: duplicated v1 row must not double-pay; expected 7 (= pool/2), got %s", gotV1)
	}
	if !gotV2.Equal(math.NewInt(7)) {
		t.Fatalf("Issue 8: v2 must get half the pool; expected 7, got %s", gotV2)
	}
}

func TestKT_Issue8_DistributeFailFee_DedupesDuplicateVerifierAddresses(t *testing.T) {
	k, ctx, bk, _ := setupTrackingKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)

	user := makeAddr("kt-i8-fail-user")
	worker := makeAddr("kt-i8-fail-worker")
	v1 := makeAddr("kt-i8-f-v1")
	v2 := makeAddr("kt-i8-f-v2")

	// failFee = 1000 * 150/1000 = 150 → verifier portion = 150 * 12/15 = 120.
	// Per-unique split = 60 each.
	fee := sdk.NewCoin("ufai", math.NewInt(1000))
	_ = k.ProcessDeposit(ctx, user, fee)

	failVerifiers := []types.VerifierResult{
		{Address: v1.String(), Pass: false},
		{Address: v1.String(), Pass: false}, // duplicate
		{Address: v2.String(), Pass: false},
	}

	entries := []types.SettlementEntry{
		{
			TaskId: []byte("kt-i8-task-fail-001x"), UserAddress: user.String(),
			WorkerAddress: worker.String(), Fee: fee, ExpireBlock: 10000,
			Status: types.SettlementFail, VerifierResults: failVerifiers,
		},
	}
	msg := makeBatchMsg(t, makeAddr("kt-i8-prop-2").String(), entries)
	if _, err := k.ProcessBatchSettlement(ctx, msg); err != nil {
		t.Fatalf("settle: %v", err)
	}

	gotV1 := bk.receivedBy(v1)
	gotV2 := bk.receivedBy(v2)

	// Each unique verifier should get exactly 60.
	if !gotV1.Equal(math.NewInt(60)) {
		t.Fatalf("Issue 8 (FAIL): duplicated v1 must not double-pay; expected 60 (= 120/2), got %s", gotV1)
	}
	if !gotV2.Equal(math.NewInt(60)) {
		t.Fatalf("Issue 8 (FAIL): v2 must get half the FAIL verifier pool; expected 60, got %s", gotV2)
	}
}

func TestKT_Issue8_DistributeSuccessFee_AllUniqueUnchanged(t *testing.T) {
	// Sanity: when all verifiers are unique, behavior is unchanged from
	// pre-fix. 3 unique verifiers split a 12-unit pool into 4 each.
	k, ctx, bk, _ := setupTrackingKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)

	user := makeAddr("kt-i8-uniq-user")
	worker := makeAddr("kt-i8-uniq-worker")
	v1 := makeAddr("kt-i8-u-v1")
	v2 := makeAddr("kt-i8-u-v2")
	v3 := makeAddr("kt-i8-u-v3")

	fee := sdk.NewCoin("ufai", math.NewInt(100))
	_ = k.ProcessDeposit(ctx, user, fee)

	entries := []types.SettlementEntry{
		{
			TaskId: []byte("kt-i8-task-uniq-001x"), UserAddress: user.String(),
			WorkerAddress: worker.String(), Fee: fee, ExpireBlock: 10000,
			Status: types.SettlementSuccess,
			VerifierResults: []types.VerifierResult{
				{Address: v1.String(), Pass: true},
				{Address: v2.String(), Pass: true},
				{Address: v3.String(), Pass: true},
			},
		},
	}
	msg := makeBatchMsg(t, makeAddr("kt-i8-prop-3").String(), entries)
	if _, err := k.ProcessBatchSettlement(ctx, msg); err != nil {
		t.Fatalf("settle: %v", err)
	}

	// 3 unique × 4 = 12 (pool). Last-gets-remainder may be 4 or higher
	// depending on dust; total must equal 12.
	total := bk.receivedBy(v1).Add(bk.receivedBy(v2)).Add(bk.receivedBy(v3))
	if !total.Equal(math.NewInt(12)) {
		t.Fatalf("Issue 8 sanity: 3-unique total must equal pool (12), got %s", total)
	}
}
