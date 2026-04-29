package types

import (
	"context"

	"github.com/cosmos/gogoproto/proto"
)

func init() {
	proto.RegisterType((*MsgDepositResponse)(nil), "funai.settlement.MsgDepositResponse")
	proto.RegisterType((*MsgWithdrawResponse)(nil), "funai.settlement.MsgWithdrawResponse")
	proto.RegisterType((*MsgBatchSettlementResponse)(nil), "funai.settlement.MsgBatchSettlementResponse")
	proto.RegisterType((*MsgBatchReserveResponse)(nil), "funai.settlement.MsgBatchReserveResponse")
	proto.RegisterType((*MsgFraudProofResponse)(nil), "funai.settlement.MsgFraudProofResponse")
	proto.RegisterType((*MsgSecondVerificationResultResponse)(nil), "funai.settlement.MsgSecondVerificationResultResponse")
	proto.RegisterType((*MsgSecondVerificationResultBatchResponse)(nil), "funai.settlement.MsgSecondVerificationResultBatchResponse")
}

type MsgServer interface {
	Deposit(context.Context, *MsgDeposit) (*MsgDepositResponse, error)
	Withdraw(context.Context, *MsgWithdraw) (*MsgWithdrawResponse, error)
	BatchSettle(context.Context, *MsgBatchSettlement) (*MsgBatchSettlementResponse, error)
	BatchReserve(context.Context, *MsgBatchReserve) (*MsgBatchReserveResponse, error)
	SubmitFraudProof(context.Context, *MsgFraudProof) (*MsgFraudProofResponse, error)
	SubmitSecondVerificationResult(context.Context, *MsgSecondVerificationResult) (*MsgSecondVerificationResultResponse, error)
	SubmitSecondVerificationResultBatch(context.Context, *MsgSecondVerificationResultBatch) (*MsgSecondVerificationResultBatchResponse, error)
}

type MsgDepositResponse struct{}

func (m *MsgDepositResponse) ProtoMessage()  {}
func (m *MsgDepositResponse) Reset()         { *m = MsgDepositResponse{} }
func (m *MsgDepositResponse) String() string { return "MsgDepositResponse" }

type MsgWithdrawResponse struct{}

func (m *MsgWithdrawResponse) ProtoMessage()  {}
func (m *MsgWithdrawResponse) Reset()         { *m = MsgWithdrawResponse{} }
func (m *MsgWithdrawResponse) String() string { return "MsgWithdrawResponse" }

type MsgBatchSettlementResponse struct {
	BatchId uint64 `protobuf:"varint,1,opt,name=batch_id,proto3" json:"batch_id"`
}

func (m *MsgBatchSettlementResponse) ProtoMessage()  {}
func (m *MsgBatchSettlementResponse) Reset()         { *m = MsgBatchSettlementResponse{} }
func (m *MsgBatchSettlementResponse) String() string { return "MsgBatchSettlementResponse" }

// MsgBatchReserveResponse reports per-batch reservation outcome. Bad rows
// (account missing, denom mismatch, expired, duplicate, insufficient
// available balance) are silently skipped so a single bad row cannot block
// the rest — RejectedCount tells the Leader how many were dropped.
type MsgBatchReserveResponse struct {
	AcceptedCount uint32 `protobuf:"varint,1,opt,name=accepted_count,proto3" json:"accepted_count"`
	RejectedCount uint32 `protobuf:"varint,2,opt,name=rejected_count,proto3" json:"rejected_count"`
}

func (m *MsgBatchReserveResponse) ProtoMessage()  {}
func (m *MsgBatchReserveResponse) Reset()         { *m = MsgBatchReserveResponse{} }
func (m *MsgBatchReserveResponse) String() string { return "MsgBatchReserveResponse" }

type MsgFraudProofResponse struct{}

func (m *MsgFraudProofResponse) ProtoMessage()  {}
func (m *MsgFraudProofResponse) Reset()         { *m = MsgFraudProofResponse{} }
func (m *MsgFraudProofResponse) String() string { return "MsgFraudProofResponse" }

type MsgSecondVerificationResultResponse struct{}

func (m *MsgSecondVerificationResultResponse) ProtoMessage() {}
func (m *MsgSecondVerificationResultResponse) Reset()        { *m = MsgSecondVerificationResultResponse{} }
func (m *MsgSecondVerificationResultResponse) String() string {
	return "MsgSecondVerificationResultResponse"
}

// MsgSecondVerificationResultBatchResponse reports how many entries were
// accepted; rejected entries (bad sig / unknown verifier) are logged but
// do not fail the tx, so a single bad entry cannot block the rest.
type MsgSecondVerificationResultBatchResponse struct {
	AcceptedCount uint32 `protobuf:"varint,1,opt,name=accepted_count,proto3" json:"accepted_count"`
	RejectedCount uint32 `protobuf:"varint,2,opt,name=rejected_count,proto3" json:"rejected_count"`
}

func (m *MsgSecondVerificationResultBatchResponse) ProtoMessage() {}
func (m *MsgSecondVerificationResultBatchResponse) Reset() {
	*m = MsgSecondVerificationResultBatchResponse{}
}
func (m *MsgSecondVerificationResultBatchResponse) String() string {
	return "MsgSecondVerificationResultBatchResponse"
}
