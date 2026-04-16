package keeper

import (
	"context"

	sdkerrors "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/modelreg/types"
)

type queryServer struct {
	Keeper
}

// NewQueryServerImpl returns an implementation of the modelreg QueryServer interface.
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{Keeper: keeper}
}

var _ types.QueryServer = queryServer{}

func (q queryServer) Model(goCtx context.Context, req *types.QueryModelRequest) (*types.QueryModelResponse, error) {
	if req == nil || req.ModelId == "" {
		return nil, sdkerrors.Wrap(types.ErrInvalidModelId, "model_id is required")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	model, found := q.GetModel(ctx, req.ModelId)
	if !found {
		return nil, types.ErrModelNotFound
	}

	return &types.QueryModelResponse{Model: model}, nil
}

func (q queryServer) ModelByAlias(goCtx context.Context, req *types.QueryModelByAliasRequest) (*types.QueryModelResponse, error) {
	if req == nil || req.Alias == "" {
		return nil, sdkerrors.Wrap(types.ErrInvalidAlias, "alias is required")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	model, found := q.GetModelByAlias(ctx, req.Alias)
	if !found {
		return nil, types.ErrModelNotFound
	}

	return &types.QueryModelResponse{Model: model}, nil
}

func (q queryServer) Models(goCtx context.Context, _ *types.QueryModelsRequest) (*types.QueryModelsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	models := q.GetAllModels(ctx)
	return &types.QueryModelsResponse{Models: models}, nil
}

func (q queryServer) Params(goCtx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := q.GetParams(ctx)
	return &types.QueryParamsResponse{Params: params}, nil
}
