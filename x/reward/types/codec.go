package types

import (
	"context"

	"github.com/cosmos/cosmos-sdk/types/module"

	"google.golang.org/grpc"
)

func RegisterMsgServer(cfg module.Configurator, srv MsgServer) {
	cfg.MsgServer().RegisterService(&_Msg_serviceDesc, srv)
}

func RegisterQueryServer(cfg module.Configurator, srv QueryServer) {
	cfg.QueryServer().RegisterService(&_Query_serviceDesc, srv)
}

var _Msg_serviceDesc = grpc.ServiceDesc{
	ServiceName: "funai.reward.Msg",
	HandlerType: (*MsgServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "UpdateParams", Handler: _Msg_UpdateParams_Handler},
	},
	Streams: []grpc.StreamDesc{},
}

var _Query_serviceDesc = grpc.ServiceDesc{
	ServiceName: "funai.reward.Query",
	HandlerType: (*QueryServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "Params", Handler: _Query_Params_Handler},
		{MethodName: "RewardHistory", Handler: _Query_RewardHistory_Handler},
	},
	Streams: []grpc.StreamDesc{},
}

// -------- Msg Handlers --------

func _Msg_UpdateParams_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgUpdateParams)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).UpdateParams(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.reward.Msg/UpdateParams"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).UpdateParams(ctx, req.(*MsgUpdateParams))
	}
	return interceptor(ctx, in, info, handler)
}

// -------- Query Handlers --------

func _Query_Params_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryParamsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).QueryParams(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.reward.Query/Params"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).QueryParams(ctx, req.(*QueryParamsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_RewardHistory_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryRewardHistoryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).QueryRewardHistory(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.reward.Query/RewardHistory"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).QueryRewardHistory(ctx, req.(*QueryRewardHistoryRequest))
	}
	return interceptor(ctx, in, info, handler)
}
