package keeper_test

// Edge-case and boundary-condition tests for the reward module.

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/reward/types"
)

// ============================================================
// 1. Block reward at exact halving boundary
// ============================================================

func TestBlockReward_ExactHalvingBoundary(t *testing.T) {
	k, ctx, _ := setupRewardKeeper(t)

	base := types.DefaultBaseBlockReward
	hp := types.DefaultHalvingPeriod

	// Block hp-1: full reward (last block before halving)
	r := k.CalculateBlockReward(ctx, hp-1)
	if !r.Equal(base) {
		t.Fatalf("block %d: expected full reward %s, got %s", hp-1, base, r)
	}

	// Block hp: first halved block
	r = k.CalculateBlockReward(ctx, hp)
	if !r.Equal(base.QuoRaw(2)) {
		t.Fatalf("block %d: expected halved %s, got %s", hp, base.QuoRaw(2), r)
	}

	// Block hp+1: still halved
	r = k.CalculateBlockReward(ctx, hp+1)
	if !r.Equal(base.QuoRaw(2)) {
		t.Fatalf("block %d: expected halved, got %s", hp+1, r)
	}
}

// ============================================================
// 2. Block reward at block 0 and 1
// ============================================================

func TestBlockReward_FirstBlocks(t *testing.T) {
	k, ctx, _ := setupRewardKeeper(t)

	base := types.DefaultBaseBlockReward

	r0 := k.CalculateBlockReward(ctx, 0)
	if !r0.Equal(base) {
		t.Fatalf("block 0: expected %s, got %s", base, r0)
	}

	r1 := k.CalculateBlockReward(ctx, 1)
	if !r1.Equal(base) {
		t.Fatalf("block 1: expected %s, got %s", base, r1)
	}
}

// ============================================================
// 3. Epoch reward across halving boundary (precise check)
// ============================================================

func TestEpochReward_AcrossHalving_Precise(t *testing.T) {
	k, ctx, _ := setupRewardKeeper(t)

	params := types.DefaultParams()
	params.EpochBlocks = 20
	_ = k.SetParams(ctx, params)

	hp := types.DefaultHalvingPeriod
	base := types.DefaultBaseBlockReward

	// Epoch ending at hp+10: epochStart = hp+10-20+1 = hp-9
	// Blocks hp-9 to hp-1 = 9 blocks at full reward
	// Blocks hp to hp+10 = 11 blocks at half reward
	epochEnd := hp + 10
	reward := k.CalculateEpochReward(ctx, epochEnd)

	expected := base.MulRaw(9).Add(base.QuoRaw(2).MulRaw(11))
	if !reward.Equal(expected) {
		t.Fatalf("epoch across halving: expected %s, got %s", expected, reward)
	}
}

// ============================================================
// 4. Distribute with only verification contributors
// ============================================================

func TestDistributeRewards_WithVerifiers(t *testing.T) {
	k, ctx, bk := setupRewardKeeper(t)

	w1 := sdk.AccAddress([]byte("worker1_____________"))
	v1 := sdk.AccAddress([]byte("verifier1___________"))
	v2 := sdk.AccAddress([]byte("verifier2___________"))

	// Need inference contributions for the verification pool to be distributed
	contributions := []types.WorkerContribution{
		{WorkerAddress: w1.String(), FeeAmount: math.NewInt(1000), TaskCount: 10},
	}

	verifiers := []types.VerificationContribution{
		{WorkerAddress: v1.String(), VerificationCount: 50, AuditCount: 5},
		{WorkerAddress: v2.String(), VerificationCount: 50, AuditCount: 5},
	}

	err := k.DistributeRewards(ctx, contributions, verifiers, nil, nil)
	if err != nil {
		t.Fatalf("distribute with verifiers: %v", err)
	}

	sent1, ok1 := bk.sent[v1.String()]
	sent2, ok2 := bk.sent[v2.String()]
	if !ok1 || !ok2 {
		t.Fatal("both verifiers should receive rewards")
	}

	// Equal contributions → equal rewards
	if !sent1.Equal(sent2) {
		t.Fatalf("equal verifiers should get equal rewards: %s vs %s", sent1, sent2)
	}
}

// ============================================================
// 5. Distribute with extreme fee weight disparity
// ============================================================

func TestDistributeRewards_ExtremeWeightDisparity(t *testing.T) {
	k, ctx, bk := setupRewardKeeper(t)

	addr1 := sdk.AccAddress([]byte("worker1_____________"))
	addr2 := sdk.AccAddress([]byte("worker2_____________"))

	// Worker1: 99% fees, 1% tasks. Worker2: 1% fees, 99% tasks
	contributions := []types.WorkerContribution{
		{WorkerAddress: addr1.String(), FeeAmount: math.NewInt(9900), TaskCount: 1},
		{WorkerAddress: addr2.String(), FeeAmount: math.NewInt(100), TaskCount: 99},
	}

	err := k.DistributeRewards(ctx, contributions, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent1 := bk.sent[addr1.String()]
	sent2 := bk.sent[addr2.String()]

	// Worker1: 0.8*(9900/10000) + 0.2*(1/100) = 0.792 + 0.002 = 0.794
	// Worker2: 0.8*(100/10000) + 0.2*(99/100) = 0.008 + 0.198 = 0.206
	// Worker1 should get significantly more
	if sent1.LTE(sent2) {
		t.Fatalf("worker1 (high fees) should get more: w1=%s, w2=%s", sent1, sent2)
	}
}

// ============================================================
// 6. Distribute by stake with single worker → gets everything
// ============================================================

func TestDistributeRewards_ByStake_SingleWorker(t *testing.T) {
	k, ctx, bk := setupRewardKeeper(t)

	addr := sdk.AccAddress([]byte("worker1_____________"))
	onlineWorkers := []types.OnlineWorkerStake{
		{WorkerAddress: addr.String(), Stake: math.NewInt(50000)},
	}

	err := k.DistributeRewards(ctx, nil, nil, nil, onlineWorkers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent := bk.sent[addr.String()]
	epochReward := k.CalculateEpochReward(ctx, 100)
	if !sent.Equal(epochReward) {
		t.Fatalf("single staker should get full reward: expected %s, got %s", epochReward, sent)
	}
}

// ============================================================
// 7. Epoch reward at far future (many halvings) → zero
// ============================================================

func TestEpochReward_FarFuture_Zero(t *testing.T) {
	k, ctx, _ := setupRewardKeeper(t)

	farHeight := int64(100) * types.DefaultHalvingPeriod
	reward := k.CalculateEpochReward(ctx, farHeight)
	if !reward.IsZero() {
		t.Fatalf("expected zero reward after many halvings, got %s", reward)
	}
}

// ============================================================
// 8. Multiple reward records for same worker, different epochs
// ============================================================

func TestRewardRecords_MultipleEpochs(t *testing.T) {
	k, ctx, _ := setupRewardKeeper(t)

	addr := sdk.AccAddress([]byte("worker1_____________"))
	for i := 0; i < 10; i++ {
		k.SetRewardRecord(ctx, types.RewardRecord{
			Epoch:         int64(i),
			WorkerAddress: addr.String(),
			Amount:        sdk.NewCoin(types.BondDenom, math.NewInt(int64(i*1000))),
		})
	}

	records := k.GetRewardRecords(ctx, addr.String())
	if len(records) != 10 {
		t.Fatalf("expected 10 records, got %d", len(records))
	}
}

// ============================================================
// 9. Params: weights sum check
// ============================================================

func TestParams_WeightsConsistency(t *testing.T) {
	params := types.DefaultParams()

	feeWeight, _ := params.FeeWeight.Float64()
	countWeight, _ := params.CountWeight.Float64()

	sum := feeWeight + countWeight
	if sum < 0.99 || sum > 1.01 {
		t.Fatalf("fee_weight + count_weight should be ~1.0, got %f", sum)
	}
}

// ============================================================
// 10. Epoch blocks = 1 (minimum meaningful epoch)
// ============================================================

func TestEpochReward_EpochBlocksOne(t *testing.T) {
	k, ctx, _ := setupRewardKeeper(t)

	params := types.DefaultParams()
	params.EpochBlocks = 1
	_ = k.SetParams(ctx, params)

	reward := k.CalculateEpochReward(ctx, 100)
	// With epoch=1 block, reward should equal 1 block's reward
	blockReward := k.CalculateBlockReward(ctx, 100)
	if !reward.Equal(blockReward) {
		t.Fatalf("epoch=1 block: expected %s, got %s", blockReward, reward)
	}
}

// ============================================================
// 11. Distribute with zero fee but non-zero task count
// ============================================================

func TestDistributeRewards_ZeroFee_NonZeroTasks(t *testing.T) {
	k, ctx, bk := setupRewardKeeper(t)

	addr := sdk.AccAddress([]byte("worker1_____________"))
	contributions := []types.WorkerContribution{
		{WorkerAddress: addr.String(), FeeAmount: math.ZeroInt(), TaskCount: 100},
	}

	err := k.DistributeRewards(ctx, contributions, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Worker should still get reward (from task count weight)
	sent := bk.sent[addr.String()]
	if sent.IsZero() {
		t.Fatal("worker with 0 fee but 100 tasks should still get reward from count weight")
	}
}

// ============================================================
// 12. Genesis with empty records
// ============================================================

func TestGenesis_EmptyRecords(t *testing.T) {
	k, ctx, _ := setupRewardKeeper(t)

	gs := types.GenesisState{
		Params:        types.DefaultParams(),
		RewardRecords: nil,
	}

	k.InitGenesis(ctx, gs)

	exported := k.ExportGenesis(ctx)
	if len(exported.RewardRecords) != 0 {
		t.Fatalf("expected 0 records, got %d", len(exported.RewardRecords))
	}
}

// ============================================================
// V2. Reward mint conservation: total minted == total distributed
// ============================================================

func TestRewardMintConservation(t *testing.T) {
	k, ctx, bk := setupRewardKeeper(t)

	addr1 := sdk.AccAddress([]byte("worker1_____________"))
	addr2 := sdk.AccAddress([]byte("worker2_____________"))
	addr3 := sdk.AccAddress([]byte("worker3_____________"))

	contributions := []types.WorkerContribution{
		{WorkerAddress: addr1.String(), FeeAmount: math.NewInt(5000), TaskCount: 50},
		{WorkerAddress: addr2.String(), FeeAmount: math.NewInt(3000), TaskCount: 30},
		{WorkerAddress: addr3.String(), FeeAmount: math.NewInt(2000), TaskCount: 20},
	}

	err := k.DistributeRewards(ctx, contributions, nil, nil, nil)
	if err != nil {
		t.Fatalf("DistributeRewards failed: %v", err)
	}

	// Sum all rewards sent to workers
	totalDistributed := math.ZeroInt()
	for _, addr := range []sdk.AccAddress{addr1, addr2, addr3} {
		sent, ok := bk.sent[addr.String()]
		if !ok {
			t.Fatalf("worker %s should have received reward", addr)
		}
		if !sent.IsPositive() {
			t.Fatalf("worker %s reward should be positive, got %s", addr, sent)
		}
		totalDistributed = totalDistributed.Add(sent)
	}

	// Total minted should equal total distributed (no leakage)
	totalMinted := math.ZeroInt()
	for _, v := range bk.minted {
		totalMinted = totalMinted.Add(v)
	}

	if !totalMinted.Equal(totalDistributed) {
		t.Fatalf("conservation violated: minted=%s, distributed=%s", totalMinted, totalDistributed)
	}

	// Also verify against expected epoch reward (99% inference pool)
	epochReward := k.CalculateEpochReward(ctx, 100)
	inferenceReward := types.DefaultInferenceWeight.MulInt(epochReward).TruncateInt()
	if !totalDistributed.Equal(inferenceReward) {
		t.Fatalf("total distributed %s should equal inference reward (99%%) %s",
			totalDistributed, inferenceReward)
	}
}
