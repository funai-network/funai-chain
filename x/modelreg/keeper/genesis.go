package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/modelreg/types"
)

// InitGenesis initializes the modelreg module state from a genesis state.
func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) {
	k.SetParams(ctx, gs.Params)

	for _, model := range gs.Models {
		if model.ModelId == "" {
			panic("invalid model_id in genesis: empty")
		}
		k.SetModel(ctx, model)
	}
}

// ExportGenesis exports the current modelreg module state as a genesis state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params: k.GetParams(ctx),
		Models: k.GetAllModels(ctx),
	}
}
