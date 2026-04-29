package proposer

// Tests for KT 30-case Issue 10 — readyAuditEntry.Epoch must be populated.
//
// Pre-fix moveAuditToReadyLocked appended `readyAuditEntry{TaskId, Responses}`
// without setting Epoch, so the field defaulted to 0. BuildAuditBatch then
// emitted MsgSecondVerificationResultBatch entries with `Epoch: 0`,
// silently corrupting the per-epoch dimension on every audit submission.
//
// Post-fix Proposer carries a `currentEpoch` field updated by the dispatch
// tick (`SetCurrentEpoch`); moveAuditToReadyLocked stamps the entry with
// that value at audit-completion time. SecondVerificationEntrySigBytes
// does NOT include Epoch in the canonical pre-image, so this layered
// stamping is signature-safe.

import (
	"testing"

	p2ptypes "github.com/funai-wiki/funai-chain/p2p/types"
)

// newTestProposerForEpoch builds a Proposer with the minimum scaffolding
// needed to drive the audit-completion code path (no chain client, no
// Store, no Rebroadcaster — none are touched by the Epoch logic).
func newTestProposerForEpoch() *Proposer {
	return &Proposer{
		Address:       "test-proposer",
		pendingTasks:  make(map[string]*TaskEvidence),
		pendingAudits: make(map[string]*AuditEvidence),
	}
}

// addThreeAuditResponses drives a single task to "ready" via three calls to
// AddSecondVerificationResp — the trigger for moveAuditToReadyLocked.
func addThreeAuditResponses(t *testing.T, p *Proposer, taskId []byte) {
	t.Helper()
	for i := 0; i < 3; i++ {
		_, _ = p.CollectSecondVerificationResponse(&p2ptypes.SecondVerificationResponse{
			TaskId:             taskId,
			Pass:               true,
			SecondVerifierAddr: []byte{byte(0x10 + i)}, // distinct stub pubkey per response
			LogitsHash:         []byte{0xa0, byte(i)},
		})
	}
}

// TestKT_Issue10_AuditEntry_DefaultsToZeroWhenEpochUnset documents the
// pre-fix (and current-no-dispatch-call) baseline. Without
// SetCurrentEpoch, readyAuditEntry.Epoch is 0 — the bug. The test pins
// this so future contributors see the explicit baseline before
// understanding the fix.
func TestKT_Issue10_AuditEntry_DefaultsToZeroWhenEpochUnset(t *testing.T) {
	p := newTestProposerForEpoch()

	addThreeAuditResponses(t, p, []byte("issue10-no-set-epoch"))

	if len(p.readyAudits) != 1 {
		t.Fatalf("expected 1 ready audit after 3 responses, got %d", len(p.readyAudits))
	}
	if p.readyAudits[0].Epoch != 0 {
		t.Fatalf("baseline (no SetCurrentEpoch): expected Epoch=0, got %d", p.readyAudits[0].Epoch)
	}
}

// TestKT_Issue10_SetCurrentEpoch_PropagatesToReadyAuditEntry verifies the fix.
func TestKT_Issue10_SetCurrentEpoch_PropagatesToReadyAuditEntry(t *testing.T) {
	p := newTestProposerForEpoch()

	// Simulate the dispatch tick at chain height 4200 (epoch 42).
	p.SetCurrentEpoch(42)

	addThreeAuditResponses(t, p, []byte("issue10-with-epoch-42"))

	if len(p.readyAudits) != 1 {
		t.Fatalf("expected 1 ready audit, got %d", len(p.readyAudits))
	}
	if got := p.readyAudits[0].Epoch; got != 42 {
		t.Fatalf("Issue 10: expected Epoch=42 stamped at completion, got %d", got)
	}
}

// TestKT_Issue10_SetCurrentEpoch_MonotonicAscending pins that
// SetCurrentEpoch only moves the cached epoch forward — out-of-order
// dispatch ticks (e.g. due to chain query lag) cannot regress the field.
func TestKT_Issue10_SetCurrentEpoch_MonotonicAscending(t *testing.T) {
	p := newTestProposerForEpoch()

	p.SetCurrentEpoch(50)
	p.SetCurrentEpoch(48) // older — must NOT regress
	if p.currentEpoch != 50 {
		t.Fatalf("Issue 10: epoch must be monotonic; expected 50 after 50→48 sequence, got %d", p.currentEpoch)
	}

	p.SetCurrentEpoch(60) // newer — must advance
	if p.currentEpoch != 60 {
		t.Fatalf("Issue 10: epoch must advance to 60, got %d", p.currentEpoch)
	}
}

// TestKT_Issue10_BuildAuditBatch_PropagatesEpochToBatchEntries: full path —
// SetCurrentEpoch → addThree → BuildAuditBatch → assert each entry's Epoch.
func TestKT_Issue10_BuildAuditBatch_PropagatesEpochToBatchEntries(t *testing.T) {
	p := newTestProposerForEpoch()
	p.SetCurrentEpoch(7)

	addThreeAuditResponses(t, p, []byte("issue10-batch-epoch-71"))

	msg := p.BuildAuditBatch()
	if msg == nil {
		t.Fatal("BuildAuditBatch must return a non-nil msg when readyAudits non-empty")
	}
	if len(msg.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(msg.Entries))
	}
	for i, entry := range msg.Entries {
		if entry.Epoch != 7 {
			t.Fatalf("Issue 10: entry %d Epoch should be 7, got %d", i, entry.Epoch)
		}
	}
}
