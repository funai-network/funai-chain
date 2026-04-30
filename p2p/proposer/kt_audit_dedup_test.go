package proposer

// Issue C (FunAI-non-state-machine-findings-2026-04-30): the proposer's audit
// pipeline used to append every CollectSecondVerificationResponse to its
// pending bucket without checking SecondVerifierAddr. A single second_verifier
// could submit 3 responses and complete the audit alone; the resulting
// MsgSecondVerificationResultBatch would carry the duplicates onto chain
// where the keeper-level dedup (also added in this PR) would reject the 2nd
// and 3rd. The defense-in-depth here drops them earlier so the on-chain
// batch never sees them.
//
// Tests:
//   - duplicate SecondVerifierAddr is silently dropped (returns false, false)
//   - the audit completes only when 3 distinct addrs have submitted

import (
	"testing"

	"github.com/cometbft/cometbft/crypto/secp256k1"
)

func TestKT_IssueC_Proposer_DuplicateSecondVerifier_Dropped(t *testing.T) {
	p := New("funai1test", nil, nil, 100, 100)
	taskId := []byte("ic-prop-task-001")
	v1 := secp256k1.GenPrivKey()
	v2 := secp256k1.GenPrivKey()

	r1 := makeAuditResponse(taskId, v1, true)
	complete, _ := p.CollectSecondVerificationResponse(&r1)
	if complete {
		t.Fatal("1st response must not complete the audit")
	}

	// Same v1 submits again — must be dropped, NOT counted.
	r1dup := makeAuditResponse(taskId, v1, false)
	complete, _ = p.CollectSecondVerificationResponse(&r1dup)
	if complete {
		t.Fatal("Issue C: duplicate v1 must be silently dropped, not advance the audit")
	}

	// v2 submits — count is now 2, still not complete.
	r2 := makeAuditResponse(taskId, v2, true)
	complete, _ = p.CollectSecondVerificationResponse(&r2)
	if complete {
		t.Fatal("after dedup, only 2 distinct verifiers — must not complete")
	}

	if n := p.ReadyAuditCount(); n != 0 {
		t.Fatalf("Issue C: ReadyAuditCount must be 0 with only 2 distinct verifiers (duplicate v1 was dropped), got %d", n)
	}
}

func TestKT_IssueC_Proposer_ThreeDistinctVerifiers_Completes(t *testing.T) {
	p := New("funai1test", nil, nil, 100, 100)
	taskId := []byte("ic-prop-task-002")
	v1, v2, v3 := secp256k1.GenPrivKey(), secp256k1.GenPrivKey(), secp256k1.GenPrivKey()

	r1 := makeAuditResponse(taskId, v1, true)
	if c, _ := p.CollectSecondVerificationResponse(&r1); c {
		t.Fatal("1st must not complete")
	}
	// v1 dup interleaved between v1 and v2 — must NOT advance.
	r1dup := makeAuditResponse(taskId, v1, true)
	if c, _ := p.CollectSecondVerificationResponse(&r1dup); c {
		t.Fatal("v1 duplicate must not complete")
	}
	r2 := makeAuditResponse(taskId, v2, true)
	if c, _ := p.CollectSecondVerificationResponse(&r2); c {
		t.Fatal("2nd distinct must not complete")
	}
	r3 := makeAuditResponse(taskId, v3, true)
	complete, pass := p.CollectSecondVerificationResponse(&r3)
	if !complete {
		t.Fatal("3rd distinct must complete the audit despite the v1 duplicate that was dropped")
	}
	if !pass {
		t.Fatal("3 PASS responses must aggregate to pass=true")
	}

	msg := p.BuildAuditBatch()
	if msg == nil {
		t.Fatal("BuildAuditBatch must return a msg after 3 distinct responses")
	}
	if len(msg.Entries) != 3 {
		t.Fatalf("expected 3 distinct entries, got %d", len(msg.Entries))
	}
}
