package leader

// Tests for KT 30-case Issue 1, PR-B — Leader-side reservation queue.
//
// Pre-PR-B: Leader.pendingFees existed locally but was never reflected on
// chain. PR #44 added the keeper-side MsgBatchReserve handler; this PR
// (PR-B) adds the per-request emitter that drains pendingFees into a
// MsgBatchReserve, marks entries Reserved=true on chain confirmation, and
// retries on broadcast failure.
//
// These tests exercise the in-memory side of that contract:
//   - BuildReserveEntries returns only per-request, not-yet-reserved entries
//   - CommitReservations flips Reserved=true and is idempotent on duplicate
//     keys (Leader can re-commit safely)
//   - Per-token entries are excluded (they have their own MsgRequestQuote
//     freeze and would double-freeze otherwise)
//   - Receipt cleanup (HandleReceiptBusyRelease) removes the entry regardless
//     of Reserved status, so a "commit then receipt" race is fine
//   - Re-running BuildReserveEntries twice with no commit returns the same
//     entries (the retry path)
//
// The actual broadcast/sign/encode is in p2p/dispatch.go and is exercised by
// the e2e mock; here we only validate the leader's queue surface.

import (
	"bytes"
	"testing"
)

func TestReservation_BuildReturnsOnlyPerRequestUnreserved(t *testing.T) {
	l := newTestLeader()

	user := "user-hex-1"
	l.pendingFees[user] = []PendingEntry{
		// per-request, unreserved → should appear
		{TaskId: []byte("task-A"), MaxFee: 100, ExpireBlock: 999999999999, UserAddress: "funai1aaa", IsPerRequest: true, Reserved: false},
		// per-request, already reserved → must NOT appear
		{TaskId: []byte("task-B"), MaxFee: 200, ExpireBlock: 999999999999, UserAddress: "funai1aaa", IsPerRequest: true, Reserved: true},
		// per-token → must NOT appear (own freeze via MsgRequestQuote)
		{TaskId: []byte("task-C"), MaxFee: 300, ExpireBlock: 999999999999, UserAddress: "funai1aaa", IsPerRequest: false, Reserved: false},
	}

	entries, keys := l.BuildReserveEntries()

	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 entry (per-request + unreserved), got %d", len(entries))
	}
	if len(keys) != 1 {
		t.Fatalf("entries and keys must align; got %d keys", len(keys))
	}
	if !bytes.Equal(entries[0].TaskId, []byte("task-A")) {
		t.Fatalf("expected task-A (per-request unreserved), got %s", entries[0].TaskId)
	}
	if entries[0].MaxFee != 100 || entries[0].UserAddress != "funai1aaa" {
		t.Fatalf("entry fields wrong: %+v", entries[0])
	}
}

func TestReservation_CommitMarksReservedAndIsIdempotent(t *testing.T) {
	l := newTestLeader()

	user := "user-hex-2"
	l.pendingFees[user] = []PendingEntry{
		{TaskId: []byte("c-task-1"), MaxFee: 100, ExpireBlock: 999999999999, UserAddress: "funai1aaa", IsPerRequest: true},
		{TaskId: []byte("c-task-2"), MaxFee: 200, ExpireBlock: 999999999999, UserAddress: "funai1aaa", IsPerRequest: true},
	}

	entries, keys := l.BuildReserveEntries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	l.CommitReservations(keys)

	// Both should now be Reserved=true.
	for _, e := range l.pendingFees[user] {
		if !e.Reserved {
			t.Fatalf("entry %s must be Reserved=true after commit", e.TaskId)
		}
	}

	// Re-build returns no entries (all reserved).
	entries2, _ := l.BuildReserveEntries()
	if len(entries2) != 0 {
		t.Fatalf("after commit, BuildReserveEntries must return 0, got %d", len(entries2))
	}

	// Idempotent: committing again does not change state.
	l.CommitReservations(keys)
	for _, e := range l.pendingFees[user] {
		if !e.Reserved {
			t.Fatalf("re-commit: entry %s must remain Reserved=true", e.TaskId)
		}
	}
}

func TestReservation_NoCommitMeansRetry(t *testing.T) {
	l := newTestLeader()

	user := "user-hex-3"
	l.pendingFees[user] = []PendingEntry{
		{TaskId: []byte("retry-task"), MaxFee: 100, ExpireBlock: 999999999999, UserAddress: "funai1aaa", IsPerRequest: true},
	}

	// First build (simulating a tick that broadcasts but fails before commit).
	entries1, keys1 := l.BuildReserveEntries()
	if len(entries1) != 1 || len(keys1) != 1 {
		t.Fatalf("first build: expected 1 entry, got %d / %d", len(entries1), len(keys1))
	}

	// No commit (broadcast failed).

	// Second build (next tick) — entry still unreserved, should reappear.
	entries2, keys2 := l.BuildReserveEntries()
	if len(entries2) != 1 || len(keys2) != 1 {
		t.Fatalf("retry build: expected 1 entry, got %d / %d", len(entries2), len(keys2))
	}
	if !bytes.Equal(entries2[0].TaskId, []byte("retry-task")) {
		t.Fatalf("retry build: wrong task, got %s", entries2[0].TaskId)
	}
}

func TestReservation_EmptyPendingReturnsNil(t *testing.T) {
	l := newTestLeader()

	entries, keys := l.BuildReserveEntries()
	if entries != nil || keys != nil {
		t.Fatalf("empty pendingFees must return (nil, nil), got %v / %v", entries, keys)
	}
}

func TestReservation_ExpiredEntriesExcluded(t *testing.T) {
	l := newTestLeader()

	user := "user-hex-4"
	// cleanExpiredPending uses time.Now()/5 as approximate block height.
	// Pick ExpireBlock=1 so it's definitely past — current "block" is huge.
	l.pendingFees[user] = []PendingEntry{
		{TaskId: []byte("expired-task"), MaxFee: 100, ExpireBlock: 1, UserAddress: "funai1aaa", IsPerRequest: true},
		{TaskId: []byte("fresh-task-x"), MaxFee: 200, ExpireBlock: 999999999999999, UserAddress: "funai1aaa", IsPerRequest: true},
	}

	entries, _ := l.BuildReserveEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 fresh entry (expired excluded), got %d", len(entries))
	}
	if !bytes.Equal(entries[0].TaskId, []byte("fresh-task-x")) {
		t.Fatalf("expected fresh-task, got %s", entries[0].TaskId)
	}
}

func TestReservation_ReceiptCleanupAfterCommit(t *testing.T) {
	l := newTestLeader()

	user := "user-hex-5"
	taskId := []byte("commit-then-receipt")
	l.pendingFees[user] = []PendingEntry{
		{TaskId: taskId, MaxFee: 100, ExpireBlock: 999999999999, UserAddress: "funai1aaa", IsPerRequest: true},
	}

	_, keys := l.BuildReserveEntries()
	l.CommitReservations(keys)

	// Receipt arrives and removes the entry — pendingFees for this user
	// should be empty afterwards regardless of Reserved status.
	l.removePendingEntry(user, taskId)

	if _, exists := l.pendingFees[user]; exists {
		t.Fatalf("pendingFees[user] must be deleted after receipt cleanup of last entry")
	}
}

func TestReservation_EmptyUserAddressDefensive(t *testing.T) {
	l := newTestLeader()

	user := "user-hex-6"
	// Defensive: an entry with no bech32 (would be a bug in append site)
	// must not be emitted — the chain message would be unparseable.
	l.pendingFees[user] = []PendingEntry{
		{TaskId: []byte("no-bech32-bug"), MaxFee: 100, ExpireBlock: 999999999999, UserAddress: "", IsPerRequest: true},
		{TaskId: []byte("good-task"), MaxFee: 200, ExpireBlock: 999999999999, UserAddress: "funai1aaa", IsPerRequest: true},
	}

	entries, _ := l.BuildReserveEntries()
	if len(entries) != 1 {
		t.Fatalf("expected only the good entry, got %d", len(entries))
	}
	if !bytes.Equal(entries[0].TaskId, []byte("good-task")) {
		t.Fatalf("expected good-task, got %s", entries[0].TaskId)
	}
}

func TestReservation_MultiUserBatching(t *testing.T) {
	l := newTestLeader()

	l.pendingFees["alice-hex"] = []PendingEntry{
		{TaskId: []byte("alice-1"), MaxFee: 100, ExpireBlock: 999999999999, UserAddress: "funai1alice", IsPerRequest: true},
		{TaskId: []byte("alice-2"), MaxFee: 200, ExpireBlock: 999999999999, UserAddress: "funai1alice", IsPerRequest: true},
	}
	l.pendingFees["bob-hex"] = []PendingEntry{
		{TaskId: []byte("bob-1"), MaxFee: 300, ExpireBlock: 999999999999, UserAddress: "funai1bob", IsPerRequest: true},
	}

	entries, keys := l.BuildReserveEntries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries across 2 users, got %d", len(entries))
	}
	if len(keys) != 3 {
		t.Fatalf("keys count must match entries, got %d", len(keys))
	}

	// Verify both users appear.
	users := map[string]int{}
	for _, e := range entries {
		users[e.UserAddress]++
	}
	if users["funai1alice"] != 2 || users["funai1bob"] != 1 {
		t.Fatalf("user distribution wrong: %v", users)
	}

	l.CommitReservations(keys)
	leftover, _ := l.BuildReserveEntries()
	if len(leftover) != 0 {
		t.Fatalf("after commit, BuildReserveEntries must return 0, got %d", len(leftover))
	}
}
