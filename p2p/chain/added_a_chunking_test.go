package chain

// Tests for KT 30-case Added A — settlement batch chunking + live gas/bytes
// preflight.
//
// Pre-fix: BroadcastSettlement / BroadcastBatchReserve / BroadcastAuditBatch
// blindly submitted any encoded tx to /broadcast_tx_sync. A batch large
// enough to exceed CometBFT's mempool max-tx-bytes (default 1 MiB) would be
// silently rejected with a generic "tx too large" CheckTx code; the chain
// client surfaced no actionable error to the dispatch layer, and a Proposer
// with thousands of queued cleared-tasks would broadcast the same oversized
// payload on every tick forever.
//
// Post-fix: a preflight against the cached chain block.max_bytes (or the
// 750 000-byte fallback) returns chain.ErrTxTooLarge BEFORE submission.
// The dispatch layer recognises this sentinel and halves the proposer's
// batch ceiling so the next tick produces a chunk that fits.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================================
// Consensus params query — happy path.
// ============================================================

func TestRefreshConsensusParams_ParsesMaxBytesAndMaxGas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/consensus_params") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"consensus_params": map[string]any{
					"block": map[string]any{
						"max_bytes": "2097152",  // 2 MiB
						"max_gas":   "100000000",
					},
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.URL)
	maxBytes, maxGas, err := c.RefreshConsensusParams(context.Background())
	if err != nil {
		t.Fatalf("RefreshConsensusParams unexpected error: %v", err)
	}
	if maxBytes != 2097152 {
		t.Fatalf("expected max_bytes=2097152, got %d", maxBytes)
	}
	if maxGas != 100000000 {
		t.Fatalf("expected max_gas=100000000, got %d", maxGas)
	}
	// Cached.
	c.consensusMu.RLock()
	cached := c.consensusFetched
	c.consensusMu.RUnlock()
	if !cached {
		t.Fatal("consensus params not marked fetched after successful query")
	}
}

// ============================================================
// MaxTxBytesPreflight — fallback when not fetched, 90% of cache when fetched.
// ============================================================

func TestMaxTxBytesPreflight_FallbackBeforeRefresh(t *testing.T) {
	c := NewClient("http://nowhere", "http://nowhere")
	// Never refreshed.
	got := c.MaxTxBytesPreflight()
	if got != DefaultMaxTxBytes {
		t.Fatalf("expected fallback %d, got %d", DefaultMaxTxBytes, got)
	}
}

func TestMaxTxBytesPreflight_NinetyPercentOfCachedMax(t *testing.T) {
	c := NewClient("http://nowhere", "http://nowhere")
	c.consensusMu.Lock()
	c.consensusFetched = true
	c.consensusMaxBytes = 1_048_576 // 1 MiB
	c.consensusMu.Unlock()

	got := c.MaxTxBytesPreflight()
	// margin = 1_048_576 / 10 = 104_857; budget = 1_048_576 - 104_857 = 943_719
	if got != 943_719 {
		t.Fatalf("expected 943719 (90%% of 1MiB), got %d", got)
	}
}

func TestMaxTxBytesPreflight_HandlesNegativeUnlimitedFromGenesis(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Some chains set max_bytes=-1 ("unlimited"). RefreshConsensusParams
		// must coerce to fallback so preflight still bites.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"consensus_params": map[string]any{
					"block": map[string]any{"max_bytes": "-1", "max_gas": "100000000"},
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.URL)
	maxBytes, _, err := c.RefreshConsensusParams(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maxBytes != DefaultMaxTxBytes {
		t.Fatalf("max_bytes=-1 must coerce to fallback %d, got %d", DefaultMaxTxBytes, maxBytes)
	}
}

// ============================================================
// preflightTxSize — under budget passes; over budget returns ErrTxTooLarge.
// ============================================================

func TestPreflightTxSize_UnderBudgetPassesThrough(t *testing.T) {
	c := NewClient("http://nowhere", "http://nowhere")
	c.consensusMu.Lock()
	c.consensusFetched = true
	c.consensusMaxBytes = 1_048_576
	c.consensusMu.Unlock()

	// 100 KB tx: well within 90% of 1 MiB.
	tx := make([]byte, 100_000)
	if err := c.preflightTxSize(context.Background(), tx); err != nil {
		t.Fatalf("100KB tx must pass preflight: %v", err)
	}
}

func TestPreflightTxSize_OverBudgetReturnsErrTxTooLarge(t *testing.T) {
	c := NewClient("http://nowhere", "http://nowhere")
	c.consensusMu.Lock()
	c.consensusFetched = true
	c.consensusMaxBytes = 1_048_576
	c.consensusMu.Unlock()

	// 2 MiB tx: way over 90% of 1 MiB.
	tx := make([]byte, 2_000_000)
	err := c.preflightTxSize(context.Background(), tx)
	if err == nil {
		t.Fatal("2MB tx must return ErrTxTooLarge")
	}
	if !errors.Is(err, ErrTxTooLarge) {
		t.Fatalf("expected ErrTxTooLarge, got: %v", err)
	}
	if !strings.Contains(err.Error(), "encoded=2000000") {
		t.Fatalf("error must include actual size for diagnostics, got: %v", err)
	}
	if !strings.Contains(err.Error(), "budget=") {
		t.Fatalf("error must include budget for diagnostics, got: %v", err)
	}
}

// ============================================================
// preflightTxSize triggers a one-shot Refresh on first call.
// ============================================================

func TestPreflightTxSize_LazyRefreshOnFirstCall(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/consensus_params") {
			calls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"consensus_params": map[string]any{
						"block": map[string]any{"max_bytes": "1048576", "max_gas": "100000000"},
					},
				},
			})
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.URL)
	tx := make([]byte, 100)
	// Two preflights — the second should re-use the cache.
	if err := c.preflightTxSize(context.Background(), tx); err != nil {
		t.Fatalf("first preflight: %v", err)
	}
	if err := c.preflightTxSize(context.Background(), tx); err != nil {
		t.Fatalf("second preflight: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 /consensus_params call (cache hit on second), got %d", calls)
	}
}

// ============================================================
// preflightTxSize fallback when /consensus_params is unreachable.
// ============================================================

func TestPreflightTxSize_UsesFallbackWhenEndpointDown(t *testing.T) {
	// Point at a bogus URL with a tight HTTP timeout so the test stays fast.
	c := NewClient("http://127.0.0.1:1", "http://127.0.0.1:1")
	c.http.Timeout = 200 * time.Millisecond

	// 1 MB tx is over the 750 000-byte fallback budget → ErrTxTooLarge.
	tx := make([]byte, 1_000_000)
	err := c.preflightTxSize(context.Background(), tx)
	if err == nil {
		t.Fatal("expected ErrTxTooLarge under fallback budget")
	}
	if !errors.Is(err, ErrTxTooLarge) {
		t.Fatalf("expected ErrTxTooLarge, got: %v", err)
	}
	if !strings.Contains(err.Error(), "budget=750000") {
		t.Fatalf("expected fallback budget=750000 in error, got: %v", err)
	}
}
