package types

import (
	"context"

	"github.com/cosmos/gogoproto/proto"
)

func init() {
	proto.RegisterType((*MsgRegisterWorkerResponse)(nil), "funai.worker.MsgRegisterWorkerResponse")
	proto.RegisterType((*MsgExitWorkerResponse)(nil), "funai.worker.MsgExitWorkerResponse")
	proto.RegisterType((*MsgUpdateModelsResponse)(nil), "funai.worker.MsgUpdateModelsResponse")
	proto.RegisterType((*MsgStakeResponse)(nil), "funai.worker.MsgStakeResponse")
	proto.RegisterType((*MsgUnjailResponse)(nil), "funai.worker.MsgUnjailResponse")
}

type MsgServer interface {
	RegisterWorker(context.Context, *MsgRegisterWorker) (*MsgRegisterWorkerResponse, error)
	ExitWorker(context.Context, *MsgExitWorker) (*MsgExitWorkerResponse, error)
	UpdateModels(context.Context, *MsgUpdateModels) (*MsgUpdateModelsResponse, error)
	AddStake(context.Context, *MsgStake) (*MsgStakeResponse, error)
	Unjail(context.Context, *MsgUnjail) (*MsgUnjailResponse, error)
}

type MsgRegisterWorkerResponse struct{}

func (m *MsgRegisterWorkerResponse) ProtoMessage()  {}
func (m *MsgRegisterWorkerResponse) Reset()         { *m = MsgRegisterWorkerResponse{} }
func (m *MsgRegisterWorkerResponse) String() string { return "MsgRegisterWorkerResponse" }

type MsgExitWorkerResponse struct{}

func (m *MsgExitWorkerResponse) ProtoMessage()  {}
func (m *MsgExitWorkerResponse) Reset()         { *m = MsgExitWorkerResponse{} }
func (m *MsgExitWorkerResponse) String() string { return "MsgExitWorkerResponse" }

type MsgUpdateModelsResponse struct{}

func (m *MsgUpdateModelsResponse) ProtoMessage()  {}
func (m *MsgUpdateModelsResponse) Reset()         { *m = MsgUpdateModelsResponse{} }
func (m *MsgUpdateModelsResponse) String() string { return "MsgUpdateModelsResponse" }

type MsgStakeResponse struct{}

func (m *MsgStakeResponse) ProtoMessage()  {}
func (m *MsgStakeResponse) Reset()         { *m = MsgStakeResponse{} }
func (m *MsgStakeResponse) String() string { return "MsgStakeResponse" }

type MsgUnjailResponse struct{}

func (m *MsgUnjailResponse) ProtoMessage()  {}
func (m *MsgUnjailResponse) Reset()         { *m = MsgUnjailResponse{} }
func (m *MsgUnjailResponse) String() string { return "MsgUnjailResponse" }
