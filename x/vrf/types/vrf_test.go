package types_test

import (
	"math/big"
	"testing"

	"cosmossdk.io/math"

	"github.com/funai-wiki/funai-chain/x/vrf/types"
)

func TestComputeScore_AlphaDispatch(t *testing.T) {
	seed := []byte("test_seed_for_vrf")
	pubkey := []byte("pubkey_A")
	stake := math.NewInt(10000)

	score := types.ComputeScore(seed, pubkey, stake, types.AlphaDispatch)
	if score == nil {
		t.Fatal("score should not be nil")
	}
	if score.Sign() < 0 {
		t.Fatal("score should not be negative")
	}
}

func TestComputeScore_AlphaVerification(t *testing.T) {
	seed := []byte("test_seed_for_vrf")
	pubkey := []byte("pubkey_A")
	stake := math.NewInt(10000)

	score := types.ComputeScore(seed, pubkey, stake, types.AlphaVerification)
	if score == nil {
		t.Fatal("score should not be nil")
	}
	if score.Sign() < 0 {
		t.Fatal("score should not be negative")
	}
}

func TestComputeScore_AlphaZero_IgnoresStake(t *testing.T) {
	seed := []byte("test_seed")
	pubkey := []byte("pubkey_A")

	score1 := types.ComputeScore(seed, pubkey, math.NewInt(1), types.AlphaAudit)
	score2 := types.ComputeScore(seed, pubkey, math.NewInt(1000000), types.AlphaAudit)

	if score1.Cmp(score2) != 0 {
		t.Fatal("with alpha=0.0, stake should not affect score")
	}
}

func TestComputeScore_HigherStake_LowerScore(t *testing.T) {
	seed := []byte("test_seed")
	pubkey := []byte("pubkey_A")

	scoreSmall := types.ComputeScore(seed, pubkey, math.NewInt(1000), types.AlphaDispatch)
	scoreLarge := types.ComputeScore(seed, pubkey, math.NewInt(100000), types.AlphaDispatch)

	if scoreSmall.Cmp(scoreLarge) <= 0 {
		t.Fatal("higher stake should produce lower score (better rank)")
	}
}

func TestComputeScore_DifferentPubkeys_DifferentScores(t *testing.T) {
	seed := []byte("test_seed")
	stake := math.NewInt(10000)

	scoreA := types.ComputeScore(seed, []byte("pubkey_A"), stake, types.AlphaDispatch)
	scoreB := types.ComputeScore(seed, []byte("pubkey_B"), stake, types.AlphaDispatch)

	if scoreA.Cmp(scoreB) == 0 {
		t.Fatal("different pubkeys should produce different scores")
	}
}

func TestComputeScore_DifferentSeeds_DifferentScores(t *testing.T) {
	pubkey := []byte("pubkey_A")
	stake := math.NewInt(10000)

	score1 := types.ComputeScore([]byte("seed_1"), pubkey, stake, types.AlphaDispatch)
	score2 := types.ComputeScore([]byte("seed_2"), pubkey, stake, types.AlphaDispatch)

	if score1.Cmp(score2) == 0 {
		t.Fatal("different seeds should produce different scores")
	}
}

func TestComputeScore_Deterministic(t *testing.T) {
	seed := []byte("test_seed")
	pubkey := []byte("pubkey_A")
	stake := math.NewInt(10000)

	s1 := types.ComputeScore(seed, pubkey, stake, types.AlphaDispatch)
	s2 := types.ComputeScore(seed, pubkey, stake, types.AlphaDispatch)

	if s1.Cmp(s2) != 0 {
		t.Fatal("same inputs should produce the same score")
	}
}

func TestComputeScore_ZeroStake_ReturnsHash(t *testing.T) {
	seed := []byte("test_seed")
	pubkey := []byte("pubkey_A")

	score := types.ComputeScore(seed, pubkey, math.ZeroInt(), types.AlphaDispatch)
	if score == nil {
		t.Fatal("score should not be nil even with zero stake")
	}
}

func TestComputeScore_SqrtStake_VerificationAlpha(t *testing.T) {
	seed := []byte("test_seed")
	pubkey := []byte("pubkey_A")

	scoreSmall := types.ComputeScore(seed, pubkey, math.NewInt(1000), types.AlphaVerification)
	scoreLarge := types.ComputeScore(seed, pubkey, math.NewInt(100000), types.AlphaVerification)

	if scoreSmall.Cmp(scoreLarge) <= 0 {
		t.Fatal("higher stake should produce lower score with verification alpha (sqrt weight)")
	}

	scoreDispatchSmall := types.ComputeScore(seed, pubkey, math.NewInt(1000), types.AlphaDispatch)
	scoreDispatchLarge := types.ComputeScore(seed, pubkey, math.NewInt(100000), types.AlphaDispatch)

	dispatchRatio := new(big.Float).Quo(scoreDispatchSmall, scoreDispatchLarge)
	verifyRatio := new(big.Float).Quo(scoreSmall, scoreLarge)

	if dispatchRatio.Cmp(verifyRatio) <= 0 {
		t.Fatal("dispatch alpha should separate scores more than verification alpha")
	}
}

func TestRankWorkers_Ordering(t *testing.T) {
	seed := []byte("test_seed_for_ranking")
	workers := []types.RankedWorker{
		{Address: "worker_A", Pubkey: []byte("pubA"), Stake: math.NewInt(1000)},
		{Address: "worker_B", Pubkey: []byte("pubB"), Stake: math.NewInt(50000)},
		{Address: "worker_C", Pubkey: []byte("pubC"), Stake: math.NewInt(10000)},
	}

	ranked := types.RankWorkers(seed, workers, types.AlphaDispatch)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked workers, got %d", len(ranked))
	}

	for i := 0; i < len(ranked)-1; i++ {
		if ranked[i].Score.Cmp(ranked[i+1].Score) > 0 {
			t.Fatalf("workers not sorted by ascending score at index %d", i)
		}
	}
}

func TestRankWorkers_AlphaVerification(t *testing.T) {
	seed := []byte("test_seed")
	workers := []types.RankedWorker{
		{Address: "worker_A", Pubkey: []byte("pubA"), Stake: math.NewInt(1000)},
		{Address: "worker_B", Pubkey: []byte("pubB"), Stake: math.NewInt(100000)},
	}

	ranked := types.RankWorkers(seed, workers, types.AlphaVerification)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 ranked workers, got %d", len(ranked))
	}

	for i := 0; i < len(ranked)-1; i++ {
		if ranked[i].Score.Cmp(ranked[i+1].Score) > 0 {
			t.Fatal("workers not sorted by ascending score")
		}
	}
}

func TestRankWorkers_AlphaAudit(t *testing.T) {
	seed := []byte("test_seed")
	workers := []types.RankedWorker{
		{Address: "worker_A", Pubkey: []byte("pubA"), Stake: math.NewInt(1)},
		{Address: "worker_B", Pubkey: []byte("pubB"), Stake: math.NewInt(1000000)},
	}

	ranked := types.RankWorkers(seed, workers, types.AlphaAudit)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 ranked workers, got %d", len(ranked))
	}

	for i := 0; i < len(ranked)-1; i++ {
		if ranked[i].Score.Cmp(ranked[i+1].Score) > 0 {
			t.Fatal("workers not sorted by ascending score")
		}
	}
}

func TestRankWorkers_EmptyInput(t *testing.T) {
	seed := []byte("test_seed")
	var workers []types.RankedWorker

	ranked := types.RankWorkers(seed, workers, types.AlphaDispatch)
	if len(ranked) != 0 {
		t.Fatalf("expected 0 ranked workers for empty input, got %d", len(ranked))
	}
}

func TestRankWorkers_SingleWorker(t *testing.T) {
	seed := []byte("test_seed")
	workers := []types.RankedWorker{
		{Address: "worker_A", Pubkey: []byte("pubA"), Stake: math.NewInt(10000)},
	}

	ranked := types.RankWorkers(seed, workers, types.AlphaDispatch)
	if len(ranked) != 1 {
		t.Fatalf("expected 1 ranked worker, got %d", len(ranked))
	}
	if ranked[0].Address != "worker_A" {
		t.Fatalf("expected worker_A, got %s", ranked[0].Address)
	}
}

func TestRankWorkers_Deterministic(t *testing.T) {
	seed := []byte("test_seed_deterministic")
	workers1 := []types.RankedWorker{
		{Address: "worker_A", Pubkey: []byte("pubA"), Stake: math.NewInt(1000)},
		{Address: "worker_B", Pubkey: []byte("pubB"), Stake: math.NewInt(50000)},
		{Address: "worker_C", Pubkey: []byte("pubC"), Stake: math.NewInt(10000)},
	}
	workers2 := []types.RankedWorker{
		{Address: "worker_A", Pubkey: []byte("pubA"), Stake: math.NewInt(1000)},
		{Address: "worker_B", Pubkey: []byte("pubB"), Stake: math.NewInt(50000)},
		{Address: "worker_C", Pubkey: []byte("pubC"), Stake: math.NewInt(10000)},
	}

	ranked1 := types.RankWorkers(seed, workers1, types.AlphaDispatch)
	ranked2 := types.RankWorkers(seed, workers2, types.AlphaDispatch)

	for i := range ranked1 {
		if ranked1[i].Address != ranked2[i].Address {
			t.Fatalf("ranking should be deterministic: index %d got %s vs %s", i, ranked1[i].Address, ranked2[i].Address)
		}
	}
}
