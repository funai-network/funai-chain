package keeper_test

// Issue C residual (FunAI-non-state-machine-findings-2026-04-30):
// ProcessSecondVerificationResult had a P2-7 conflict-of-interest check that
// rejected results from any address that was an ORIGINAL verifier on the
// task. It did NOT prevent the SAME second_verifier from submitting the same
// task twice. A malicious second_verifier could:
//   - inflate their epoch reward count (IncrementSecondVerifierEpochCount
//     fires on every accepted submission)
//   - skew the audit majority by stuffing two PASS or two FAIL rows
//   - reach the SecondVerifierCount threshold with fewer distinct people
//
// Post-fix: an explicit dedup loop checks ar.SecondVerifierAddresses for
// the new submitter and rejects with an error if already present. Tests
// below pin both legs of the contract:
//   - first submission is accepted, second is rejected
//   - epoch count is incremented exactly once per (task, second_verifier) pair

import (
	"strings"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/settlement/types"
)

// ============================================================
// KT-IssueC-A. Same second_verifier submits twice on same task →
// second submission must be rejected with a clear error.
// ============================================================

func TestKT_IssueC_SameVerifierTwice_SecondRejected(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)
	k.SetCurrentThirdVerificationRate(ctx, 0)

	taskId := []byte("ic-A-task-001-padded")
	origV1 := makeAddr("ic-A-orig-v1")
	origV2 := makeAddr("ic-A-orig-v2")
	origV3 := makeAddr("ic-A-orig-v3")
	dupVerifier := makeAddr("ic-A-second-verif")

	k.SetSecondVerificationPending(ctx, types.SecondVerificationPendingTask{
		TaskId:            taskId,
		OriginalStatus:    types.SettlementSuccess,
		SubmittedAt:       ctx.BlockHeight(),
		UserAddress:       makeAddr("ic-A-user").String(),
		WorkerAddress:     makeAddr("ic-A-worker").String(),
		VerifierAddresses: []string{origV1.String(), origV2.String(), origV3.String()},
		Fee:               sdk.NewCoin("ufai", math.NewInt(1_000_000)),
		ExpireBlock:       100000,
	})

	// First submission must succeed.
	if err := k.ProcessSecondVerificationResult(ctx, &types.MsgSecondVerificationResult{
		SecondVerifier: dupVerifier.String(),
		TaskId:         taskId,
		Epoch:          1,
		Pass:           true,
		LogitsHash:     []byte("hash"),
	}); err != nil {
		t.Fatalf("first submission must succeed: %v", err)
	}

	// Second submission from the SAME second_verifier on the SAME task →
	// must error. Try with the OPPOSITE Pass value to make sure majority
	// stuffing is the actual attack vector being blocked, not a content match.
	err := k.ProcessSecondVerificationResult(ctx, &types.MsgSecondVerificationResult{
		SecondVerifier: dupVerifier.String(),
		TaskId:         taskId,
		Epoch:          1,
		Pass:           false,
		LogitsHash:     []byte("hash"),
	})
	if err == nil {
		t.Fatal("Issue C residual: duplicate same-verifier submission must be rejected")
	}
	if !strings.Contains(err.Error(), "already submitted") {
		t.Fatalf("expected error to mention 'already submitted', got: %v", err)
	}

	// Record must reflect exactly ONE submission from this verifier.
	ar, found := k.GetSecondVerificationRecord(ctx, taskId)
	if !found {
		t.Fatal("audit record must exist after first accept")
	}
	if len(ar.SecondVerifierAddresses) != 1 {
		t.Fatalf("expected exactly 1 second_verifier in record, got %d (%v)", len(ar.SecondVerifierAddresses), ar.SecondVerifierAddresses)
	}
	if ar.SecondVerifierAddresses[0] != dupVerifier.String() {
		t.Fatalf("recorded verifier mismatch: got %s, want %s", ar.SecondVerifierAddresses[0], dupVerifier.String())
	}
	// Pass record reflects the FIRST submission (true), not the rejected second (false).
	if len(ar.Results) != 1 || !ar.Results[0] {
		t.Fatalf("expected Results=[true] preserved from first submission, got %v", ar.Results)
	}
}

// ============================================================
// KT-IssueC-B. Epoch count increments exactly once per (task, verifier)
// — not twice when the duplicate is rejected.
// ============================================================

func TestKT_IssueC_DuplicateRejected_EpochCountIncrementedOnce(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)

	taskId := []byte("ic-B-task-001-padded")
	dupVerifier := makeAddr("ic-B-second-verif")

	k.SetSecondVerificationPending(ctx, types.SecondVerificationPendingTask{
		TaskId:            taskId,
		OriginalStatus:    types.SettlementSuccess,
		SubmittedAt:       ctx.BlockHeight(),
		UserAddress:       makeAddr("ic-B-user").String(),
		WorkerAddress:     makeAddr("ic-B-worker").String(),
		VerifierAddresses: []string{makeAddr("ic-B-orig-v1").String(), makeAddr("ic-B-orig-v2").String(), makeAddr("ic-B-orig-v3").String()},
		Fee:               sdk.NewCoin("ufai", math.NewInt(1_000_000)),
		ExpireBlock:       100000,
	})

	// First submission accepted → epoch count = 1.
	if err := k.ProcessSecondVerificationResult(ctx, &types.MsgSecondVerificationResult{
		SecondVerifier: dupVerifier.String(),
		TaskId:         taskId,
		Epoch:          1,
		Pass:           true,
		LogitsHash:     []byte("hash"),
	}); err != nil {
		t.Fatalf("first submission must succeed: %v", err)
	}

	countOf := func(addr string) uint64 {
		for _, c := range k.GetAllVerifierSecondVerifierEpochCounts(ctx) {
			if c.Address == addr {
				return c.AuditCount
			}
		}
		return 0
	}

	if got := countOf(dupVerifier.String()); got != 1 {
		t.Fatalf("epoch count after first submission: got %d, want 1", got)
	}

	// Duplicate rejected → epoch count must remain 1.
	_ = k.ProcessSecondVerificationResult(ctx, &types.MsgSecondVerificationResult{
		SecondVerifier: dupVerifier.String(),
		TaskId:         taskId,
		Epoch:          1,
		Pass:           true,
		LogitsHash:     []byte("hash"),
	})

	if got := countOf(dupVerifier.String()); got != 1 {
		t.Fatalf("Issue C residual: duplicate must NOT inflate epoch count. got %d, want 1", got)
	}
}

// ============================================================
// KT-IssueC-C. Threshold not reached early — duplicate cannot trigger
// processAuditJudgment with only 2 distinct verifiers.
// ============================================================

func TestKT_IssueC_DuplicateCannotReachThreshold(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	k.SetCurrentSecondVerificationRate(ctx, 0)
	k.SetCurrentThirdVerificationRate(ctx, 0)

	taskId := []byte("ic-C-task-001-padded")
	v1 := makeAddr("ic-C-second-v1")
	v2 := makeAddr("ic-C-second-v2")

	k.SetSecondVerificationPending(ctx, types.SecondVerificationPendingTask{
		TaskId:            taskId,
		OriginalStatus:    types.SettlementSuccess,
		SubmittedAt:       ctx.BlockHeight(),
		UserAddress:       makeAddr("ic-C-user").String(),
		WorkerAddress:     makeAddr("ic-C-worker").String(),
		VerifierAddresses: []string{makeAddr("ic-C-orig-v1").String(), makeAddr("ic-C-orig-v2").String(), makeAddr("ic-C-orig-v3").String()},
		Fee:               sdk.NewCoin("ufai", math.NewInt(1_000_000)),
		ExpireBlock:       100000,
	})

	// Default SecondVerifierCount = 3 → 2 distinct + 1 duplicate must NOT reach threshold.
	for i, sv := range []sdk.AccAddress{v1, v2, v1 /* duplicate */} {
		err := k.ProcessSecondVerificationResult(ctx, &types.MsgSecondVerificationResult{
			SecondVerifier: sv.String(),
			TaskId:         taskId,
			Epoch:          1,
			Pass:           true,
			LogitsHash:     []byte("hash"),
		})
		if i < 2 && err != nil {
			t.Fatalf("submission %d should succeed: %v", i, err)
		}
		if i == 2 && err == nil {
			t.Fatal("Issue C residual: third submission (duplicate of v1) must be rejected")
		}
	}

	// Pending must still be alive — judgment did not fire.
	if _, found := k.GetSecondVerificationPending(ctx, taskId); !found {
		t.Fatal("Issue C residual: with only 2 distinct verifiers, judgment must NOT fire and pending must remain")
	}

	ar, found := k.GetSecondVerificationRecord(ctx, taskId)
	if !found {
		t.Fatal("audit record must exist after first two submissions")
	}
	if len(ar.SecondVerifierAddresses) != 2 {
		t.Fatalf("expected exactly 2 distinct second_verifiers, got %d (%v)", len(ar.SecondVerifierAddresses), ar.SecondVerifierAddresses)
	}
}
