package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/funai-wiki/funai-chain/x/vrf/types"
)

type msgServer struct {
	Keeper
}

func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

func (m msgServer) SubmitVRFProof(goCtx context.Context, msg *types.MsgSubmitVRFProof) (*types.MsgSubmitVRFProofResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	m.UpdateSeed(ctx, [][]byte{msg.Value})

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeVRFProofSubmitted,
		sdk.NewAttribute(types.AttributeKeyProofSubmitter, msg.Creator),
	))

	return &types.MsgSubmitVRFProofResponse{}, nil
}

func (m msgServer) LeaderHeartbeat(goCtx context.Context, msg *types.MsgLeaderHeartbeat) (*types.MsgLeaderHeartbeatResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := m.UpdateLeaderHeartbeat(ctx, msg.ModelId, msg.Creator); err != nil {
		return nil, err
	}

	return &types.MsgLeaderHeartbeatResponse{}, nil
}

func (m msgServer) ReportLeaderTimeout(goCtx context.Context, msg *types.MsgReportLeaderTimeout) (*types.MsgReportLeaderTimeoutResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	params := m.GetParams(ctx)
	requiredProofs := int(params.TimeoutProofPercent) * int(params.CommitteeSize) / 100
	if len(msg.TimeoutProofs) < requiredProofs {
		return nil, types.ErrInsufficientProofs.Wrapf(
			"need %d proofs, got %d", requiredProofs, len(msg.TimeoutProofs))
	}

	leader, found := m.GetLeaderInfo(ctx, msg.ModelId)
	if !found {
		return nil, types.ErrLeaderNotFound
	}

	if ctx.BlockHeight()-leader.LastHeartbeat <= params.LeaderTimeoutBlocks {
		return nil, types.ErrLeaderTimeout.Wrap("leader has not timed out")
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeReElection,
		sdk.NewAttribute(types.AttributeKeyModelId, msg.ModelId),
		sdk.NewAttribute(types.AttributeKeyLeaderAddress, leader.Address),
	))

	return &types.MsgReportLeaderTimeoutResponse{
		NewLeader: "",
	}, nil
}
