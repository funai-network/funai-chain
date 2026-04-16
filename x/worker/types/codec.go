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
	ServiceName: "funai.worker.Msg",
	HandlerType: (*MsgServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RegisterWorker",
			Handler:    _Msg_RegisterWorker_Handler,
		},
		{
			MethodName: "ExitWorker",
			Handler:    _Msg_ExitWorker_Handler,
		},
		{
			MethodName: "UpdateModels",
			Handler:    _Msg_UpdateModels_Handler,
		},
		{
			MethodName: "AddStake",
			Handler:    _Msg_AddStake_Handler,
		},
		{
			MethodName: "Unjail",
			Handler:    _Msg_Unjail_Handler,
		},
	},
	Streams: []grpc.StreamDesc{},
}

var _Query_serviceDesc = grpc.ServiceDesc{
	ServiceName: "funai.worker.Query",
	HandlerType: (*QueryServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Worker",
			Handler:    _Query_Worker_Handler,
		},
		{
			MethodName: "Workers",
			Handler:    _Query_Workers_Handler,
		},
		{
			MethodName: "WorkersByModel",
			Handler:    _Query_WorkersByModel_Handler,
		},
		{
			MethodName: "Params",
			Handler:    _Query_Params_Handler,
		},
	},
	Streams: []grpc.StreamDesc{},
}

// -------- Msg Handlers --------

func _Msg_RegisterWorker_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgRegisterWorker)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).RegisterWorker(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.worker.Msg/RegisterWorker"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).RegisterWorker(ctx, req.(*MsgRegisterWorker))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_ExitWorker_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgExitWorker)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).ExitWorker(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.worker.Msg/ExitWorker"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).ExitWorker(ctx, req.(*MsgExitWorker))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_UpdateModels_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgUpdateModels)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).UpdateModels(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.worker.Msg/UpdateModels"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).UpdateModels(ctx, req.(*MsgUpdateModels))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_AddStake_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgStake)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).AddStake(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.worker.Msg/AddStake"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).AddStake(ctx, req.(*MsgStake))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_Unjail_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgUnjail)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).Unjail(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.worker.Msg/Unjail"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).Unjail(ctx, req.(*MsgUnjail))
	}
	return interceptor(ctx, in, info, handler)
}

// -------- Query Handlers --------

func _Query_Worker_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryWorkerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).Worker(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.worker.Query/Worker"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).Worker(ctx, req.(*QueryWorkerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_Workers_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryWorkersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).Workers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.worker.Query/Workers"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).Workers(ctx, req.(*QueryWorkersRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_WorkersByModel_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryWorkersByModelRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).WorkersByModel(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.worker.Query/WorkersByModel"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).WorkersByModel(ctx, req.(*QueryWorkersByModelRequest))
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
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funai.worker.Query/Params"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).Params(ctx, req.(*QueryParamsRequest))
	}
	return interceptor(ctx, in, info, handler)
}
