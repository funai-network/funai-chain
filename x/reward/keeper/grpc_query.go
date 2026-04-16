package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/reward/types"
)

var _ types.QueryServer = Keeper{}

func (k Keeper) QueryParams(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)
	return &types.QueryParamsResponse{Params: params}, nil
}

func (k Keeper) QueryRewardHistory(goCtx context.Context, req *types.QueryRewardHistoryRequest) (*types.QueryRewardHistoryResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	records := k.GetRewardRecords(ctx, req.WorkerAddress)
	return &types.QueryRewardHistoryResponse{Records: records}, nil
}
