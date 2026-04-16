package keeper

import (
	"context"

	sdkerrors "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/settlement/types"
)

type queryServer struct {
	Keeper
}

func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{Keeper: keeper}
}

var _ types.QueryServer = queryServer{}

func (q queryServer) InferenceAccount(goCtx context.Context, req *types.QueryInferenceAccountRequest) (*types.QueryInferenceAccountResponse, error) {
	if req == nil {
		return nil, sdkerrors.Wrap(types.ErrAccountNotFound, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, sdkerrors.Wrap(err, "invalid address")
	}

	ia, found := q.GetInferenceAccount(ctx, addr)
	if !found {
		return nil, types.ErrAccountNotFound
	}

	return &types.QueryInferenceAccountResponse{Account: ia}, nil
}

func (q queryServer) Batch(goCtx context.Context, req *types.QueryBatchRequest) (*types.QueryBatchResponse, error) {
	if req == nil {
		return nil, sdkerrors.Wrap(types.ErrTaskNotFound, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	br, found := q.GetBatchRecord(ctx, req.BatchId)
	if !found {
		return nil, types.ErrTaskNotFound
	}

	return &types.QueryBatchResponse{Batch: br}, nil
}

func (q queryServer) Params(goCtx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := q.GetParams(ctx)
	return &types.QueryParamsResponse{Params: params}, nil
}
