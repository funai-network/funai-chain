package types

import "context"

type QueryServer interface {
	InferenceAccount(context.Context, *QueryInferenceAccountRequest) (*QueryInferenceAccountResponse, error)
	Batch(context.Context, *QueryBatchRequest) (*QueryBatchResponse, error)
	Params(context.Context, *QueryParamsRequest) (*QueryParamsResponse, error)
}

// -------- InferenceAccount --------

type QueryInferenceAccountRequest struct {
	Address string `protobuf:"bytes,1,opt,name=address,proto3" json:"address"`
}

func (m *QueryInferenceAccountRequest) ProtoMessage()  {}
func (m *QueryInferenceAccountRequest) Reset()         { *m = QueryInferenceAccountRequest{} }
func (m *QueryInferenceAccountRequest) String() string { return "QueryInferenceAccountRequest" }

type QueryInferenceAccountResponse struct {
	Account InferenceAccount `protobuf:"bytes,1,opt,name=account,proto3" json:"account"`
}

func (m *QueryInferenceAccountResponse) ProtoMessage()  {}
func (m *QueryInferenceAccountResponse) Reset()         { *m = QueryInferenceAccountResponse{} }
func (m *QueryInferenceAccountResponse) String() string { return "QueryInferenceAccountResponse" }

// -------- Batch --------

type QueryBatchRequest struct {
	BatchId uint64 `protobuf:"varint,1,opt,name=batch_id,proto3" json:"batch_id"`
}

func (m *QueryBatchRequest) ProtoMessage()  {}
func (m *QueryBatchRequest) Reset()         { *m = QueryBatchRequest{} }
func (m *QueryBatchRequest) String() string { return "QueryBatchRequest" }

type QueryBatchResponse struct {
	Batch BatchRecord `protobuf:"bytes,1,opt,name=batch,proto3" json:"batch"`
}

func (m *QueryBatchResponse) ProtoMessage()  {}
func (m *QueryBatchResponse) Reset()         { *m = QueryBatchResponse{} }
func (m *QueryBatchResponse) String() string { return "QueryBatchResponse" }

// -------- Params --------

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
