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
	ServiceName: "funai.vrf.Msg",
	HandlerType: (*MsgServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "SubmitVRFProof", Handler: _Msg_SubmitVRFProof_Handler},
		{MethodName: "LeaderHeartbeat", Handler: _Msg_LeaderHeartbeat_Handler},
		{MethodName: "ReportLeaderTimeout", Handler: _Msg_ReportLeaderTimeout_Handler},
	},
	Streams: []grpc.StreamDesc{},
}

var _Query_serviceDesc = grpc.ServiceDesc{
	ServiceName: "funai.vrf.Query",
	HandlerType: (*QueryServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "Params", Handler: _Query_Params_Handler},
		{MethodName: "CurrentSeed", Handler: _Query_CurrentSeed_Handler},
		{MethodName: "Leader", Handler: _Query_Leader_Handler},
		{MethodName: "Committee", Handler: _Query_Committee_Handler},
	},
	Streams: []grpc.StreamDesc{},
}

// -------- Msg Handlers --------

func _Msg_SubmitVRFProof_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgSubmitVRFProof)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).SubmitVRFProof(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.vrf.Msg/SubmitVRFProof"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).SubmitVRFProof(ctx, req.(*MsgSubmitVRFProof))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_LeaderHeartbeat_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgLeaderHeartbeat)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).LeaderHeartbeat(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.vrf.Msg/LeaderHeartbeat"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).LeaderHeartbeat(ctx, req.(*MsgLeaderHeartbeat))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_ReportLeaderTimeout_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgReportLeaderTimeout)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).ReportLeaderTimeout(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.vrf.Msg/ReportLeaderTimeout"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).ReportLeaderTimeout(ctx, req.(*MsgReportLeaderTimeout))
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
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.vrf.Query/Params"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).QueryParams(ctx, req.(*QueryParamsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_CurrentSeed_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryCurrentSeedRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).QueryCurrentSeed(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.vrf.Query/CurrentSeed"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).QueryCurrentSeed(ctx, req.(*QueryCurrentSeedRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_Leader_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryLeaderRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).QueryLeader(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.vrf.Query/Leader"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).QueryLeader(ctx, req.(*QueryLeaderRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_Committee_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryCommitteeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).QueryCommittee(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.vrf.Query/Committee"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).QueryCommittee(ctx, req.(*QueryCommitteeRequest))
	}
	return interceptor(ctx, in, info, handler)
}
