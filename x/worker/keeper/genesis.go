package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/worker/types"
)

// InitGenesis initializes the worker module state from a genesis state.
func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) {
	k.SetParams(ctx, gs.Params)

	for _, worker := range gs.Workers {
		addr, err := sdk.AccAddressFromBech32(worker.Address)
		if err != nil {
			panic("invalid worker address in genesis: " + worker.Address)
		}
		k.SetWorker(ctx, worker)
		k.SetModelIndices(ctx, addr, worker.SupportedModels)
	}
}

// ExportGenesis exports the current worker module state as a genesis state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params:  k.GetParams(ctx),
		Workers: k.GetAllWorkers(ctx),
	}
}
