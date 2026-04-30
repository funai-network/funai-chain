package keeper_test

// Tests for KT 30-case Issues 5 + 8 (settlement keeper layer).
// (Issue 10 is exercised in p2p/proposer; tests live with that package.)
//
// Issue 5  FraudProof must bind ActualContent ↔ ContentHash.
//          Pre-fix the keeper verified Worker's signature on ContentHash but
//          never checked sha256(ActualContent) == ContentHash. An attacker
//          who captured any (ContentHash, WorkerContentSig) pair from past
//          inference traffic could submit it with arbitrary ActualContent
//          and slash the Worker for content the Worker never produced.
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
// Issue 5 — FraudProof binding.
// ============================================================

// Pre-existing TestProcessFraudProof_Success in keeper_test.go uses
// signFraudContent("content") which sets ContentHash = sha256("content")
// AND ActualContent = "content" (matching). That test continues to pass
// because the binding check is satisfied. The cases below pin the
// rejection path.

func TestKT_Issue5_FraudProof_RejectsContentMismatch(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	worker := makeAddr("kt-i5-worker")
	taskId := []byte("kt-i5-mismatch-task1")

	// Worker legitimately signed sha256("real-content").
	contentHash, contentSig := signFraudContent(t, []byte("real-content"))

	// Attacker submits with the legitimate (hash, sig) but FAKE actual content.
	msg := &types.MsgFraudProof{
		Reporter:         makeAddr("kt-i5-attacker").String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    []byte("fabricated-content-the-worker-never-produced"),
	}

	err := k.ProcessFraudProof(ctx, msg)
	if err == nil {
		t.Fatal("Issue 5: keeper MUST reject FraudProof when sha256(ActualContent) != ContentHash")
	}
	// Worker MUST NOT be slashed by an unbound proof.
	if k.HasFraudMark(ctx, taskId) {
		t.Fatal("Issue 5: fraud mark must NOT be set when content/hash binding fails")
	}
}

func TestKT_Issue5_FraudProof_RejectsMissingActualContent(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	worker := makeAddr("kt-i5-worker-2")
	taskId := []byte("kt-i5-missing-actual1")

	contentHash, contentSig := signFraudContent(t, []byte("anything"))

	// Empty ActualContent + non-empty ContentHash → unbinding case.
	msg := &types.MsgFraudProof{
		Reporter:         makeAddr("kt-i5-rep-2").String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    nil,
	}
	if err := k.ProcessFraudProof(ctx, msg); err == nil {
		t.Fatal("Issue 5: keeper MUST reject when ContentHash is set but ActualContent is empty")
	}
}

func TestKT_Issue5_FraudProof_AcceptsMatchingContent(t *testing.T) {
	// Sanity check that the post-fix path still accepts a legitimate proof.
	// Mirrors TestProcessFraudProof_Success but with explicit Issue-5 framing.
	// PR #50 (Issue 5.2): a chain footprint is now required, so seed a
	// SettledTask record before invoking ProcessFraudProof.
	k, ctx, _, _ := setupKeeper(t)

	worker := makeAddr("kt-i5-worker-3")
	taskId := []byte("kt-i5-bound-proof-001")
	k.SetSettledTask(ctx, types.SettledTaskID{
		TaskId:        taskId,
		Status:        types.TaskSettled,
		WorkerAddress: worker.String(),
		Fee:           sdk.NewCoin("ufai", math.NewInt(100)),
	})

	content := []byte("the actual content the worker produced")
	contentHash, contentSig := signFraudContent(t, content)

	msg := &types.MsgFraudProof{
		Reporter:         makeAddr("kt-i5-rep-3").String(),
		TaskId:           taskId,
		WorkerAddress:    worker.String(),
		ContentHash:      contentHash,
		WorkerContentSig: contentSig,
		ActualContent:    content,
	}
	if err := k.ProcessFraudProof(ctx, msg); err != nil {
		t.Fatalf("Issue 5: bound proof must still succeed, got %v", err)
	}
	if !k.HasFraudMark(ctx, taskId) {
		t.Fatal("Issue 5: fraud mark must be set on accepted bound proof")
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
