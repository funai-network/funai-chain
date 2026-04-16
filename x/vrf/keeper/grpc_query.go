package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/vrf/types"
)

var _ types.QueryServer = Keeper{}

func (k Keeper) QueryParams(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)
	return &types.QueryParamsResponse{Params: params}, nil
}

func (k Keeper) QueryCurrentSeed(goCtx context.Context, req *types.QueryCurrentSeedRequest) (*types.QueryCurrentSeedResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	seed := k.GetCurrentSeed(ctx)
	return &types.QueryCurrentSeedResponse{Seed: seed}, nil
}

func (k Keeper) QueryLeader(goCtx context.Context, req *types.QueryLeaderRequest) (*types.QueryLeaderResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	leader, found := k.GetLeaderInfo(ctx, req.ModelId)
	if !found {
		return nil, types.ErrLeaderNotFound
	}
	return &types.QueryLeaderResponse{Leader: leader}, nil
}

func (k Keeper) QueryCommittee(goCtx context.Context, req *types.QueryCommitteeRequest) (*types.QueryCommitteeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	committee, found := k.GetCommitteeInfo(ctx)
	if !found {
		return nil, types.ErrCommitteeNotFound
	}
	return &types.QueryCommitteeResponse{Committee: committee}, nil
}
