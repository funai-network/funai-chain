package app

import (
	"encoding/json"
	"time"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	modelregtypes "github.com/funai-wiki/funai-chain/x/modelreg/types"
	rewardtypes "github.com/funai-wiki/funai-chain/x/reward/types"
	settlementtypes "github.com/funai-wiki/funai-chain/x/settlement/types"
	vrftypes "github.com/funai-wiki/funai-chain/x/vrf/types"
	workertypes "github.com/funai-wiki/funai-chain/x/worker/types"
)

// FunAIGenesisState returns the customized genesis state for the FunAI chain.
func FunAIGenesisState(appCodec interface{}) map[string]json.RawMessage {
	genState := make(map[string]json.RawMessage)

	// Auth module: standard accounts
	authGenesis := authtypes.DefaultGenesisState()
	bz, _ := json.Marshal(authGenesis)
	genState[authtypes.ModuleName] = bz

	// Bank module: set FAI denomination metadata
	bankGenesis := banktypes.DefaultGenesisState()
	bankGenesis.DenomMetadata = []banktypes.Metadata{
		{
			Description: "The native staking and inference token of FunAI Network",
			DenomUnits: []*banktypes.DenomUnit{
				{Denom: BondDenom, Exponent: 0, Aliases: []string{"microfai"}},
				{Denom: "mfai", Exponent: 3, Aliases: []string{"millifai"}},
				{Denom: DisplayDenom, Exponent: 6, Aliases: nil},
			},
			Base:    BondDenom,
			Display: DisplayDenom,
			Name:    "FunAI Token",
			Symbol:  "FAI",
		},
	}
	bankGenesis.Params.DefaultSendEnabled = true
	bz, _ = json.Marshal(bankGenesis)
	genState[banktypes.ModuleName] = bz

	// Staking module: 5-second block time, FAI bond denom
	stakingGenesis := stakingtypes.DefaultGenesisState()
	stakingGenesis.Params.BondDenom = BondDenom
	stakingGenesis.Params.UnbondingTime = 21 * 24 * time.Hour // 21 days matching V5.1 exit_wait
	stakingGenesis.Params.MaxValidators = 100                 // V5.1: committee_size = 100
	stakingGenesis.Params.MinCommissionRate = math.LegacyZeroDec()
	bz, _ = json.Marshal(stakingGenesis)
	genState[stakingtypes.ModuleName] = bz

	// Worker module: min_stake = 10,000 FAI
	workerGenesis := workertypes.DefaultGenesis()
	workerGenesis.Params.MinStake = sdk.NewCoin(BondDenom, math.NewInt(10_000_000_000))
	bz, _ = json.Marshal(workerGenesis)
	genState[workertypes.ModuleName] = bz

	// Model Registry module
	modelregGenesis := modelregtypes.DefaultGenesis()
	bz, _ = json.Marshal(modelregGenesis)
	genState[modelregtypes.ModuleName] = bz

	// Settlement module
	settlementGenesis := settlementtypes.DefaultGenesis()
	bz, _ = json.Marshal(settlementGenesis)
	genState[settlementtypes.ModuleName] = bz

	// Reward module
	rewardGenesis := rewardtypes.DefaultGenesis()
	bz, _ = json.Marshal(rewardGenesis)
	genState[rewardtypes.ModuleName] = bz

	// VRF module with initial seed
	vrfGenesis := vrftypes.DefaultGenesis()
	vrfGenesis.InitialSeed = vrftypes.VRFSeed{Value: []byte("funai-v5-genesis-seed-2026"), BlockHeight: 0}
	bz, _ = json.Marshal(vrfGenesis)
	genState[vrftypes.ModuleName] = bz

	return genState
}
