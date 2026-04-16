package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/vrf/types"
)

func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) {
	if err := k.SetParams(ctx, gs.Params); err != nil {
		panic(err)
	}

	k.SetCurrentSeed(ctx, gs.InitialSeed)

	for _, leader := range gs.Leaders {
		k.SetLeaderInfo(ctx, leader)
	}

	if gs.Committee != nil {
		k.SetCommitteeInfo(ctx, *gs.Committee)
	}
}

func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	seed := k.GetCurrentSeed(ctx)
	leaders := k.GetAllLeaders(ctx)
	committee, found := k.GetCommitteeInfo(ctx)

	gs := &types.GenesisState{
		Params:      k.GetParams(ctx),
		InitialSeed: seed,
		Leaders:     leaders,
	}
	if found {
		gs.Committee = &committee
	}

	return gs
}
