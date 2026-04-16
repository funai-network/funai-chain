package keeper

import (
	"context"

	sdkerrors "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/worker/types"
)

type queryServer struct {
	Keeper
}

// NewQueryServerImpl returns an implementation of the worker QueryServer interface.
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{Keeper: keeper}
}

var _ types.QueryServer = queryServer{}

func (q queryServer) Worker(goCtx context.Context, req *types.QueryWorkerRequest) (*types.QueryWorkerResponse, error) {
	if req == nil {
		return nil, sdkerrors.Wrap(types.ErrWorkerNotFound, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, sdkerrors.Wrap(err, "invalid address")
	}

	worker, found := q.GetWorker(ctx, addr)
	if !found {
		return nil, types.ErrWorkerNotFound
	}

	return &types.QueryWorkerResponse{Worker: worker}, nil
}

func (q queryServer) Workers(goCtx context.Context, req *types.QueryWorkersRequest) (*types.QueryWorkersResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	workers := q.GetAllWorkers(ctx)
	return &types.QueryWorkersResponse{Workers: workers}, nil
}

func (q queryServer) WorkersByModel(goCtx context.Context, req *types.QueryWorkersByModelRequest) (*types.QueryWorkersByModelResponse, error) {
	if req == nil || req.ModelId == "" {
		return nil, sdkerrors.Wrap(types.ErrInvalidModels, "model id is required")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	workers := q.GetWorkersByModel(ctx, req.ModelId)
	return &types.QueryWorkersByModelResponse{Workers: workers}, nil
}

func (q queryServer) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := q.GetParams(ctx)
	return &types.QueryParamsResponse{Params: params}, nil
}
