package keeper

import (
	"encoding/json"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/settlement/types"
)

func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) {
	k.SetParams(ctx, gs.Params)

	for _, ia := range gs.InferenceAccounts {
		k.SetInferenceAccount(ctx, ia)
	}

	for _, br := range gs.BatchRecords {
		k.SetBatchRecord(ctx, br)
		if br.BatchId > 0 {
			k.SetBatchCounter(ctx, br.BatchId)
		}
	}
}

func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params:            k.GetParams(ctx),
		InferenceAccounts: k.GetAllInferenceAccounts(ctx),
		BatchRecords:      k.GetAllBatchRecords(ctx),
	}
}

func (k Keeper) GetAllBatchRecords(ctx sdk.Context) []types.BatchRecord {
	store := ctx.KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.BatchRecordKeyPrefix)
	defer iter.Close()

	var records []types.BatchRecord
	for ; iter.Valid(); iter.Next() {
		var br types.BatchRecord
		if err := json.Unmarshal(iter.Value(), &br); err != nil {
			continue
		}
		records = append(records, br)
	}
	return records
}
