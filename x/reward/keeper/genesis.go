package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/reward/types"
)

func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) {
	if err := k.SetParams(ctx, gs.Params); err != nil {
		panic(err)
	}

	for _, record := range gs.RewardRecords {
		k.SetRewardRecord(ctx, record)
	}
}

func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params:        k.GetParams(ctx),
		RewardRecords: k.GetRewardRecords(ctx, ""),
	}
}
