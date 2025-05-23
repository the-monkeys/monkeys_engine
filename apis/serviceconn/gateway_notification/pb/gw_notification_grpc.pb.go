// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v4.25.1
// source: apis/serviceconn/gateway_notification/pb/gw_notification.proto

package pb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	NotificationService_SendNotification_FullMethodName      = "/notification_svc.NotificationService/SendNotification"
	NotificationService_GetNotification_FullMethodName       = "/notification_svc.NotificationService/GetNotification"
	NotificationService_DeleteNotification_FullMethodName    = "/notification_svc.NotificationService/DeleteNotification"
	NotificationService_NotificationSeen_FullMethodName      = "/notification_svc.NotificationService/NotificationSeen"
	NotificationService_GetNotificationStream_FullMethodName = "/notification_svc.NotificationService/GetNotificationStream"
)

// NotificationServiceClient is the client API for NotificationService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type NotificationServiceClient interface {
	SendNotification(ctx context.Context, in *SendNotificationReq, opts ...grpc.CallOption) (*SendNotificationRes, error)
	GetNotification(ctx context.Context, in *GetNotificationReq, opts ...grpc.CallOption) (*GetNotificationRes, error)
	DeleteNotification(ctx context.Context, in *DeleteNotificationReq, opts ...grpc.CallOption) (*DeleteNotificationRes, error)
	NotificationSeen(ctx context.Context, in *WatchNotificationReq, opts ...grpc.CallOption) (*NotificationResponse, error)
	GetNotificationStream(ctx context.Context, in *GetNotificationReq, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GetNotificationRes], error)
}

type notificationServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewNotificationServiceClient(cc grpc.ClientConnInterface) NotificationServiceClient {
	return &notificationServiceClient{cc}
}

func (c *notificationServiceClient) SendNotification(ctx context.Context, in *SendNotificationReq, opts ...grpc.CallOption) (*SendNotificationRes, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SendNotificationRes)
	err := c.cc.Invoke(ctx, NotificationService_SendNotification_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *notificationServiceClient) GetNotification(ctx context.Context, in *GetNotificationReq, opts ...grpc.CallOption) (*GetNotificationRes, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetNotificationRes)
	err := c.cc.Invoke(ctx, NotificationService_GetNotification_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *notificationServiceClient) DeleteNotification(ctx context.Context, in *DeleteNotificationReq, opts ...grpc.CallOption) (*DeleteNotificationRes, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteNotificationRes)
	err := c.cc.Invoke(ctx, NotificationService_DeleteNotification_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *notificationServiceClient) NotificationSeen(ctx context.Context, in *WatchNotificationReq, opts ...grpc.CallOption) (*NotificationResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(NotificationResponse)
	err := c.cc.Invoke(ctx, NotificationService_NotificationSeen_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *notificationServiceClient) GetNotificationStream(ctx context.Context, in *GetNotificationReq, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GetNotificationRes], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &NotificationService_ServiceDesc.Streams[0], NotificationService_GetNotificationStream_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[GetNotificationReq, GetNotificationRes]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type NotificationService_GetNotificationStreamClient = grpc.ServerStreamingClient[GetNotificationRes]

// NotificationServiceServer is the server API for NotificationService service.
// All implementations must embed UnimplementedNotificationServiceServer
// for forward compatibility.
type NotificationServiceServer interface {
	SendNotification(context.Context, *SendNotificationReq) (*SendNotificationRes, error)
	GetNotification(context.Context, *GetNotificationReq) (*GetNotificationRes, error)
	DeleteNotification(context.Context, *DeleteNotificationReq) (*DeleteNotificationRes, error)
	NotificationSeen(context.Context, *WatchNotificationReq) (*NotificationResponse, error)
	GetNotificationStream(*GetNotificationReq, grpc.ServerStreamingServer[GetNotificationRes]) error
	mustEmbedUnimplementedNotificationServiceServer()
}

// UnimplementedNotificationServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedNotificationServiceServer struct{}

func (UnimplementedNotificationServiceServer) SendNotification(context.Context, *SendNotificationReq) (*SendNotificationRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendNotification not implemented")
}
func (UnimplementedNotificationServiceServer) GetNotification(context.Context, *GetNotificationReq) (*GetNotificationRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetNotification not implemented")
}
func (UnimplementedNotificationServiceServer) DeleteNotification(context.Context, *DeleteNotificationReq) (*DeleteNotificationRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteNotification not implemented")
}
func (UnimplementedNotificationServiceServer) NotificationSeen(context.Context, *WatchNotificationReq) (*NotificationResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method NotificationSeen not implemented")
}
func (UnimplementedNotificationServiceServer) GetNotificationStream(*GetNotificationReq, grpc.ServerStreamingServer[GetNotificationRes]) error {
	return status.Errorf(codes.Unimplemented, "method GetNotificationStream not implemented")
}
func (UnimplementedNotificationServiceServer) mustEmbedUnimplementedNotificationServiceServer() {}
func (UnimplementedNotificationServiceServer) testEmbeddedByValue()                             {}

// UnsafeNotificationServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to NotificationServiceServer will
// result in compilation errors.
type UnsafeNotificationServiceServer interface {
	mustEmbedUnimplementedNotificationServiceServer()
}

func RegisterNotificationServiceServer(s grpc.ServiceRegistrar, srv NotificationServiceServer) {
	// If the following call pancis, it indicates UnimplementedNotificationServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&NotificationService_ServiceDesc, srv)
}

func _NotificationService_SendNotification_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SendNotificationReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NotificationServiceServer).SendNotification(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: NotificationService_SendNotification_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NotificationServiceServer).SendNotification(ctx, req.(*SendNotificationReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _NotificationService_GetNotification_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetNotificationReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NotificationServiceServer).GetNotification(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: NotificationService_GetNotification_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NotificationServiceServer).GetNotification(ctx, req.(*GetNotificationReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _NotificationService_DeleteNotification_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteNotificationReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NotificationServiceServer).DeleteNotification(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: NotificationService_DeleteNotification_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NotificationServiceServer).DeleteNotification(ctx, req.(*DeleteNotificationReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _NotificationService_NotificationSeen_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(WatchNotificationReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NotificationServiceServer).NotificationSeen(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: NotificationService_NotificationSeen_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NotificationServiceServer).NotificationSeen(ctx, req.(*WatchNotificationReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _NotificationService_GetNotificationStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetNotificationReq)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(NotificationServiceServer).GetNotificationStream(m, &grpc.GenericServerStream[GetNotificationReq, GetNotificationRes]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type NotificationService_GetNotificationStreamServer = grpc.ServerStreamingServer[GetNotificationRes]

// NotificationService_ServiceDesc is the grpc.ServiceDesc for NotificationService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var NotificationService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "notification_svc.NotificationService",
	HandlerType: (*NotificationServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SendNotification",
			Handler:    _NotificationService_SendNotification_Handler,
		},
		{
			MethodName: "GetNotification",
			Handler:    _NotificationService_GetNotification_Handler,
		},
		{
			MethodName: "DeleteNotification",
			Handler:    _NotificationService_DeleteNotification_Handler,
		},
		{
			MethodName: "NotificationSeen",
			Handler:    _NotificationService_NotificationSeen_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GetNotificationStream",
			Handler:       _NotificationService_GetNotificationStream_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "apis/serviceconn/gateway_notification/pb/gw_notification.proto",
}
