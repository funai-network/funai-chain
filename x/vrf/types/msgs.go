package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

func init() {
	proto.RegisterType((*MsgSubmitVRFProof)(nil), "funai.vrf.MsgSubmitVRFProof")
	proto.RegisterType((*MsgSubmitVRFProofResponse)(nil), "funai.vrf.MsgSubmitVRFProofResponse")
	proto.RegisterType((*MsgLeaderHeartbeat)(nil), "funai.vrf.MsgLeaderHeartbeat")
	proto.RegisterType((*MsgLeaderHeartbeatResponse)(nil), "funai.vrf.MsgLeaderHeartbeatResponse")
	proto.RegisterType((*MsgReportLeaderTimeout)(nil), "funai.vrf.MsgReportLeaderTimeout")
	proto.RegisterType((*MsgReportLeaderTimeoutResponse)(nil), "funai.vrf.MsgReportLeaderTimeoutResponse")
}

type MsgSubmitVRFProof struct {
	Creator string `protobuf:"bytes,1,opt,name=creator,proto3" json:"creator"`
	Proof   []byte `protobuf:"bytes,2,opt,name=proof,proto3" json:"proof"`
	Value   []byte `protobuf:"bytes,3,opt,name=value,proto3" json:"value"`
}

func (m *MsgSubmitVRFProof) ProtoMessage()  {}
func (m *MsgSubmitVRFProof) Reset()         { *m = MsgSubmitVRFProof{} }
func (m *MsgSubmitVRFProof) String() string { return "MsgSubmitVRFProof" }

func (m *MsgSubmitVRFProof) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Creator); err != nil {
		return ErrInvalidAddress.Wrapf("invalid creator address: %s", err)
	}
	if len(m.Proof) == 0 {
		return ErrInvalidVRFProof.Wrap("proof cannot be empty")
	}
	if len(m.Value) == 0 {
		return ErrInvalidVRFProof.Wrap("value cannot be empty")
	}
	return nil
}

func (m *MsgSubmitVRFProof) GetSigners() []sdk.AccAddress {
	creator, _ := sdk.AccAddressFromBech32(m.Creator)
	return []sdk.AccAddress{creator}
}

type MsgSubmitVRFProofResponse struct{}

func (m *MsgSubmitVRFProofResponse) ProtoMessage()  {}
func (m *MsgSubmitVRFProofResponse) Reset()         { *m = MsgSubmitVRFProofResponse{} }
func (m *MsgSubmitVRFProofResponse) String() string { return "MsgSubmitVRFProofResponse" }

type MsgLeaderHeartbeat struct {
	Creator string `protobuf:"bytes,1,opt,name=creator,proto3" json:"creator"`
	ModelId string `protobuf:"bytes,2,opt,name=model_id,proto3" json:"model_id"`
}

func (m *MsgLeaderHeartbeat) ProtoMessage()  {}
func (m *MsgLeaderHeartbeat) Reset()         { *m = MsgLeaderHeartbeat{} }
func (m *MsgLeaderHeartbeat) String() string { return "MsgLeaderHeartbeat" }

func (m *MsgLeaderHeartbeat) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Creator); err != nil {
		return ErrInvalidAddress.Wrapf("invalid creator address: %s", err)
	}
	if m.ModelId == "" {
		return ErrInvalidModelId.Wrap("model id cannot be empty")
	}
	return nil
}

func (m *MsgLeaderHeartbeat) GetSigners() []sdk.AccAddress {
	creator, _ := sdk.AccAddressFromBech32(m.Creator)
	return []sdk.AccAddress{creator}
}

type MsgLeaderHeartbeatResponse struct{}

func (m *MsgLeaderHeartbeatResponse) ProtoMessage()  {}
func (m *MsgLeaderHeartbeatResponse) Reset()         { *m = MsgLeaderHeartbeatResponse{} }
func (m *MsgLeaderHeartbeatResponse) String() string { return "MsgLeaderHeartbeatResponse" }

type MsgReportLeaderTimeout struct {
	Creator       string   `protobuf:"bytes,1,opt,name=creator,proto3" json:"creator"`
	ModelId       string   `protobuf:"bytes,2,opt,name=model_id,proto3" json:"model_id"`
	TimeoutProofs [][]byte `protobuf:"bytes,3,rep,name=timeout_proofs,proto3" json:"timeout_proofs"`
}

func (m *MsgReportLeaderTimeout) ProtoMessage()  {}
func (m *MsgReportLeaderTimeout) Reset()         { *m = MsgReportLeaderTimeout{} }
func (m *MsgReportLeaderTimeout) String() string { return "MsgReportLeaderTimeout" }

func (m *MsgReportLeaderTimeout) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Creator); err != nil {
		return ErrInvalidAddress.Wrapf("invalid creator address: %s", err)
	}
	if m.ModelId == "" {
		return ErrInvalidModelId.Wrap("model id cannot be empty")
	}
	if len(m.TimeoutProofs) == 0 {
		return ErrInsufficientProofs.Wrap("timeout proofs cannot be empty")
	}
	return nil
}

func (m *MsgReportLeaderTimeout) GetSigners() []sdk.AccAddress {
	creator, _ := sdk.AccAddressFromBech32(m.Creator)
	return []sdk.AccAddress{creator}
}

type MsgReportLeaderTimeoutResponse struct {
	NewLeader string `protobuf:"bytes,1,opt,name=new_leader,proto3" json:"new_leader"`
}

func (m *MsgReportLeaderTimeoutResponse) ProtoMessage()  {}
func (m *MsgReportLeaderTimeoutResponse) Reset()         { *m = MsgReportLeaderTimeoutResponse{} }
func (m *MsgReportLeaderTimeoutResponse) String() string { return "MsgReportLeaderTimeoutResponse" }

// MsgServer defines the vrf module's message service.
type MsgServer interface {
	SubmitVRFProof(ctx context.Context, msg *MsgSubmitVRFProof) (*MsgSubmitVRFProofResponse, error)
	LeaderHeartbeat(ctx context.Context, msg *MsgLeaderHeartbeat) (*MsgLeaderHeartbeatResponse, error)
	ReportLeaderTimeout(ctx context.Context, msg *MsgReportLeaderTimeout) (*MsgReportLeaderTimeoutResponse, error)
}

// QueryServer defines the vrf module's query service.
type QueryServer interface {
	QueryParams(ctx context.Context, req *QueryParamsRequest) (*QueryParamsResponse, error)
	QueryCurrentSeed(ctx context.Context, req *QueryCurrentSeedRequest) (*QueryCurrentSeedResponse, error)
	QueryLeader(ctx context.Context, req *QueryLeaderRequest) (*QueryLeaderResponse, error)
	QueryCommittee(ctx context.Context, req *QueryCommitteeRequest) (*QueryCommitteeResponse, error)
}

type QueryParamsRequest struct{}

func (m *QueryParamsRequest) ProtoMessage()  {}
func (m *QueryParamsRequest) Reset()         { *m = QueryParamsRequest{} }
func (m *QueryParamsRequest) String() string { return "QueryParamsRequest" }

type QueryParamsResponse struct {
	Params Params `protobuf:"bytes,1,opt,name=params,proto3" json:"params"`
}

func (m *QueryParamsResponse) ProtoMessage()  {}
func (m *QueryParamsResponse) Reset()         { *m = QueryParamsResponse{} }
func (m *QueryParamsResponse) String() string { return "QueryParamsResponse" }

type QueryCurrentSeedRequest struct{}

func (m *QueryCurrentSeedRequest) ProtoMessage()  {}
func (m *QueryCurrentSeedRequest) Reset()         { *m = QueryCurrentSeedRequest{} }
func (m *QueryCurrentSeedRequest) String() string { return "QueryCurrentSeedRequest" }

type QueryCurrentSeedResponse struct {
	Seed VRFSeed `protobuf:"bytes,1,opt,name=seed,proto3" json:"seed"`
}

func (m *QueryCurrentSeedResponse) ProtoMessage()  {}
func (m *QueryCurrentSeedResponse) Reset()         { *m = QueryCurrentSeedResponse{} }
func (m *QueryCurrentSeedResponse) String() string { return "QueryCurrentSeedResponse" }

type QueryLeaderRequest struct {
	ModelId string `protobuf:"bytes,1,opt,name=model_id,proto3" json:"model_id"`
}

func (m *QueryLeaderRequest) ProtoMessage()  {}
func (m *QueryLeaderRequest) Reset()         { *m = QueryLeaderRequest{} }
func (m *QueryLeaderRequest) String() string { return "QueryLeaderRequest" }

type QueryLeaderResponse struct {
	Leader LeaderInfo `protobuf:"bytes,1,opt,name=leader,proto3" json:"leader"`
}

func (m *QueryLeaderResponse) ProtoMessage()  {}
func (m *QueryLeaderResponse) Reset()         { *m = QueryLeaderResponse{} }
func (m *QueryLeaderResponse) String() string { return "QueryLeaderResponse" }

type QueryCommitteeRequest struct{}

func (m *QueryCommitteeRequest) ProtoMessage()  {}
func (m *QueryCommitteeRequest) Reset()         { *m = QueryCommitteeRequest{} }
func (m *QueryCommitteeRequest) String() string { return "QueryCommitteeRequest" }

type QueryCommitteeResponse struct {
	Committee CommitteeInfo `protobuf:"bytes,1,opt,name=committee,proto3" json:"committee"`
}

func (m *QueryCommitteeResponse) ProtoMessage()  {}
func (m *QueryCommitteeResponse) Reset()         { *m = QueryCommitteeResponse{} }
func (m *QueryCommitteeResponse) String() string { return "QueryCommitteeResponse" }
