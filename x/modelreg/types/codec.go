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
	ServiceName: "funai.modelreg.Msg",
	HandlerType: (*MsgServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ProposeModel",
			Handler:    _Msg_ProposeModel_Handler,
		},
		{
			MethodName: "UpdateModelStats",
			Handler:    _Msg_UpdateModelStats_Handler,
		},
		{
			MethodName: "DeclareInstalled",
			Handler:    _Msg_DeclareInstalled_Handler,
		},
	},
	Streams: []grpc.StreamDesc{},
}

var _Query_serviceDesc = grpc.ServiceDesc{
	ServiceName: "funai.modelreg.Query",
	HandlerType: (*QueryServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Model",
			Handler:    _Query_Model_Handler,
		},
		{
			MethodName: "ModelByAlias",
			Handler:    _Query_ModelByAlias_Handler,
		},
		{
			MethodName: "Models",
			Handler:    _Query_Models_Handler,
		},
		{
			MethodName: "Params",
			Handler:    _Query_Params_Handler,
		},
	},
	Streams: []grpc.StreamDesc{},
}

// -------- Msg Handlers --------

func _Msg_ProposeModel_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgModelProposal)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).ProposeModel(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.modelreg.Msg/ProposeModel"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).ProposeModel(ctx, req.(*MsgModelProposal))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_UpdateModelStats_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgUpdateModelStats)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).UpdateModelStats(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.modelreg.Msg/UpdateModelStats"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).UpdateModelStats(ctx, req.(*MsgUpdateModelStats))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_DeclareInstalled_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgDeclareInstalled)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).DeclareInstalled(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.modelreg.Msg/DeclareInstalled"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).DeclareInstalled(ctx, req.(*MsgDeclareInstalled))
	}
	return interceptor(ctx, in, info, handler)
}

// -------- Query Handlers --------

func _Query_Model_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryModelRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).Model(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.modelreg.Query/Model"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).Model(ctx, req.(*QueryModelRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_ModelByAlias_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryModelByAliasRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).ModelByAlias(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.modelreg.Query/ModelByAlias"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).ModelByAlias(ctx, req.(*QueryModelByAliasRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_Models_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryModelsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).Models(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.modelreg.Query/Models"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).Models(ctx, req.(*QueryModelsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_Params_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryParamsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).Params(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.modelreg.Query/Params"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).Params(ctx, req.(*QueryParamsRequest))
	}
	return interceptor(ctx, in, info, handler)
}
