// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v4.25.1
// source: apis/serviceconn/gateway_authz/pb/gw_auth.proto

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
	AuthService_RegisterUser_FullMethodName                = "/auth_svc.AuthService/RegisterUser"
	AuthService_Validate_FullMethodName                    = "/auth_svc.AuthService/Validate"
	AuthService_CheckAccessLevel_FullMethodName            = "/auth_svc.AuthService/CheckAccessLevel"
	AuthService_Login_FullMethodName                       = "/auth_svc.AuthService/Login"
	AuthService_ForgotPassword_FullMethodName              = "/auth_svc.AuthService/ForgotPassword"
	AuthService_ResetPassword_FullMethodName               = "/auth_svc.AuthService/ResetPassword"
	AuthService_UpdatePassword_FullMethodName              = "/auth_svc.AuthService/UpdatePassword"
	AuthService_RequestForEmailVerification_FullMethodName = "/auth_svc.AuthService/RequestForEmailVerification"
	AuthService_VerifyEmail_FullMethodName                 = "/auth_svc.AuthService/VerifyEmail"
	AuthService_UpdateUsername_FullMethodName              = "/auth_svc.AuthService/UpdateUsername"
	AuthService_UpdatePasswordWithPassword_FullMethodName  = "/auth_svc.AuthService/UpdatePasswordWithPassword"
	AuthService_UpdateEmailId_FullMethodName               = "/auth_svc.AuthService/UpdateEmailId"
	AuthService_GoogleLogin_FullMethodName                 = "/auth_svc.AuthService/GoogleLogin"
	AuthService_DecodeSignedJWT_FullMethodName             = "/auth_svc.AuthService/DecodeSignedJWT"
)

// AuthServiceClient is the client API for AuthService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type AuthServiceClient interface {
	RegisterUser(ctx context.Context, in *RegisterUserRequest, opts ...grpc.CallOption) (*RegisterUserResponse, error)
	Validate(ctx context.Context, in *ValidateRequest, opts ...grpc.CallOption) (*ValidateResponse, error)
	CheckAccessLevel(ctx context.Context, in *AccessCheckReq, opts ...grpc.CallOption) (*AccessCheckRes, error)
	Login(ctx context.Context, in *LoginUserRequest, opts ...grpc.CallOption) (*LoginUserResponse, error)
	ForgotPassword(ctx context.Context, in *ForgotPasswordReq, opts ...grpc.CallOption) (*ForgotPasswordRes, error)
	ResetPassword(ctx context.Context, in *ResetPasswordReq, opts ...grpc.CallOption) (*ResetPasswordRes, error)
	UpdatePassword(ctx context.Context, in *UpdatePasswordReq, opts ...grpc.CallOption) (*UpdatePasswordRes, error)
	RequestForEmailVerification(ctx context.Context, in *EmailVerificationReq, opts ...grpc.CallOption) (*EmailVerificationRes, error)
	VerifyEmail(ctx context.Context, in *VerifyEmailReq, opts ...grpc.CallOption) (*VerifyEmailRes, error)
	UpdateUsername(ctx context.Context, in *UpdateUsernameReq, opts ...grpc.CallOption) (*UpdateUsernameRes, error)
	UpdatePasswordWithPassword(ctx context.Context, in *UpdatePasswordWithPasswordReq, opts ...grpc.CallOption) (*UpdatePasswordWithPasswordRes, error)
	UpdateEmailId(ctx context.Context, in *UpdateEmailIdReq, opts ...grpc.CallOption) (*UpdateEmailIdRes, error)
	GoogleLogin(ctx context.Context, in *RegisterUserRequest, opts ...grpc.CallOption) (*RegisterUserResponse, error)
	DecodeSignedJWT(ctx context.Context, in *DecodeSignedJWTRequest, opts ...grpc.CallOption) (*DecodeSignedJWTResponse, error)
}

type authServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewAuthServiceClient(cc grpc.ClientConnInterface) AuthServiceClient {
	return &authServiceClient{cc}
}

func (c *authServiceClient) RegisterUser(ctx context.Context, in *RegisterUserRequest, opts ...grpc.CallOption) (*RegisterUserResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RegisterUserResponse)
	err := c.cc.Invoke(ctx, AuthService_RegisterUser_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) Validate(ctx context.Context, in *ValidateRequest, opts ...grpc.CallOption) (*ValidateResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ValidateResponse)
	err := c.cc.Invoke(ctx, AuthService_Validate_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) CheckAccessLevel(ctx context.Context, in *AccessCheckReq, opts ...grpc.CallOption) (*AccessCheckRes, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AccessCheckRes)
	err := c.cc.Invoke(ctx, AuthService_CheckAccessLevel_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) Login(ctx context.Context, in *LoginUserRequest, opts ...grpc.CallOption) (*LoginUserResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LoginUserResponse)
	err := c.cc.Invoke(ctx, AuthService_Login_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) ForgotPassword(ctx context.Context, in *ForgotPasswordReq, opts ...grpc.CallOption) (*ForgotPasswordRes, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ForgotPasswordRes)
	err := c.cc.Invoke(ctx, AuthService_ForgotPassword_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) ResetPassword(ctx context.Context, in *ResetPasswordReq, opts ...grpc.CallOption) (*ResetPasswordRes, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ResetPasswordRes)
	err := c.cc.Invoke(ctx, AuthService_ResetPassword_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) UpdatePassword(ctx context.Context, in *UpdatePasswordReq, opts ...grpc.CallOption) (*UpdatePasswordRes, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpdatePasswordRes)
	err := c.cc.Invoke(ctx, AuthService_UpdatePassword_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) RequestForEmailVerification(ctx context.Context, in *EmailVerificationReq, opts ...grpc.CallOption) (*EmailVerificationRes, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(EmailVerificationRes)
	err := c.cc.Invoke(ctx, AuthService_RequestForEmailVerification_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) VerifyEmail(ctx context.Context, in *VerifyEmailReq, opts ...grpc.CallOption) (*VerifyEmailRes, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(VerifyEmailRes)
	err := c.cc.Invoke(ctx, AuthService_VerifyEmail_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) UpdateUsername(ctx context.Context, in *UpdateUsernameReq, opts ...grpc.CallOption) (*UpdateUsernameRes, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpdateUsernameRes)
	err := c.cc.Invoke(ctx, AuthService_UpdateUsername_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) UpdatePasswordWithPassword(ctx context.Context, in *UpdatePasswordWithPasswordReq, opts ...grpc.CallOption) (*UpdatePasswordWithPasswordRes, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpdatePasswordWithPasswordRes)
	err := c.cc.Invoke(ctx, AuthService_UpdatePasswordWithPassword_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) UpdateEmailId(ctx context.Context, in *UpdateEmailIdReq, opts ...grpc.CallOption) (*UpdateEmailIdRes, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpdateEmailIdRes)
	err := c.cc.Invoke(ctx, AuthService_UpdateEmailId_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) GoogleLogin(ctx context.Context, in *RegisterUserRequest, opts ...grpc.CallOption) (*RegisterUserResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RegisterUserResponse)
	err := c.cc.Invoke(ctx, AuthService_GoogleLogin_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authServiceClient) DecodeSignedJWT(ctx context.Context, in *DecodeSignedJWTRequest, opts ...grpc.CallOption) (*DecodeSignedJWTResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DecodeSignedJWTResponse)
	err := c.cc.Invoke(ctx, AuthService_DecodeSignedJWT_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AuthServiceServer is the server API for AuthService service.
// All implementations must embed UnimplementedAuthServiceServer
// for forward compatibility.
type AuthServiceServer interface {
	RegisterUser(context.Context, *RegisterUserRequest) (*RegisterUserResponse, error)
	Validate(context.Context, *ValidateRequest) (*ValidateResponse, error)
	CheckAccessLevel(context.Context, *AccessCheckReq) (*AccessCheckRes, error)
	Login(context.Context, *LoginUserRequest) (*LoginUserResponse, error)
	ForgotPassword(context.Context, *ForgotPasswordReq) (*ForgotPasswordRes, error)
	ResetPassword(context.Context, *ResetPasswordReq) (*ResetPasswordRes, error)
	UpdatePassword(context.Context, *UpdatePasswordReq) (*UpdatePasswordRes, error)
	RequestForEmailVerification(context.Context, *EmailVerificationReq) (*EmailVerificationRes, error)
	VerifyEmail(context.Context, *VerifyEmailReq) (*VerifyEmailRes, error)
	UpdateUsername(context.Context, *UpdateUsernameReq) (*UpdateUsernameRes, error)
	UpdatePasswordWithPassword(context.Context, *UpdatePasswordWithPasswordReq) (*UpdatePasswordWithPasswordRes, error)
	UpdateEmailId(context.Context, *UpdateEmailIdReq) (*UpdateEmailIdRes, error)
	GoogleLogin(context.Context, *RegisterUserRequest) (*RegisterUserResponse, error)
	DecodeSignedJWT(context.Context, *DecodeSignedJWTRequest) (*DecodeSignedJWTResponse, error)
	mustEmbedUnimplementedAuthServiceServer()
}

// UnimplementedAuthServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedAuthServiceServer struct{}

func (UnimplementedAuthServiceServer) RegisterUser(context.Context, *RegisterUserRequest) (*RegisterUserResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterUser not implemented")
}
func (UnimplementedAuthServiceServer) Validate(context.Context, *ValidateRequest) (*ValidateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Validate not implemented")
}
func (UnimplementedAuthServiceServer) CheckAccessLevel(context.Context, *AccessCheckReq) (*AccessCheckRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CheckAccessLevel not implemented")
}
func (UnimplementedAuthServiceServer) Login(context.Context, *LoginUserRequest) (*LoginUserResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Login not implemented")
}
func (UnimplementedAuthServiceServer) ForgotPassword(context.Context, *ForgotPasswordReq) (*ForgotPasswordRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ForgotPassword not implemented")
}
func (UnimplementedAuthServiceServer) ResetPassword(context.Context, *ResetPasswordReq) (*ResetPasswordRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ResetPassword not implemented")
}
func (UnimplementedAuthServiceServer) UpdatePassword(context.Context, *UpdatePasswordReq) (*UpdatePasswordRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdatePassword not implemented")
}
func (UnimplementedAuthServiceServer) RequestForEmailVerification(context.Context, *EmailVerificationReq) (*EmailVerificationRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RequestForEmailVerification not implemented")
}
func (UnimplementedAuthServiceServer) VerifyEmail(context.Context, *VerifyEmailReq) (*VerifyEmailRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method VerifyEmail not implemented")
}
func (UnimplementedAuthServiceServer) UpdateUsername(context.Context, *UpdateUsernameReq) (*UpdateUsernameRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateUsername not implemented")
}
func (UnimplementedAuthServiceServer) UpdatePasswordWithPassword(context.Context, *UpdatePasswordWithPasswordReq) (*UpdatePasswordWithPasswordRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdatePasswordWithPassword not implemented")
}
func (UnimplementedAuthServiceServer) UpdateEmailId(context.Context, *UpdateEmailIdReq) (*UpdateEmailIdRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateEmailId not implemented")
}
func (UnimplementedAuthServiceServer) GoogleLogin(context.Context, *RegisterUserRequest) (*RegisterUserResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GoogleLogin not implemented")
}
func (UnimplementedAuthServiceServer) DecodeSignedJWT(context.Context, *DecodeSignedJWTRequest) (*DecodeSignedJWTResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DecodeSignedJWT not implemented")
}
func (UnimplementedAuthServiceServer) mustEmbedUnimplementedAuthServiceServer() {}
func (UnimplementedAuthServiceServer) testEmbeddedByValue()                     {}

// UnsafeAuthServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to AuthServiceServer will
// result in compilation errors.
type UnsafeAuthServiceServer interface {
	mustEmbedUnimplementedAuthServiceServer()
}

func RegisterAuthServiceServer(s grpc.ServiceRegistrar, srv AuthServiceServer) {
	// If the following call pancis, it indicates UnimplementedAuthServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&AuthService_ServiceDesc, srv)
}

func _AuthService_RegisterUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).RegisterUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_RegisterUser_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).RegisterUser(ctx, req.(*RegisterUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_Validate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ValidateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).Validate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_Validate_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).Validate(ctx, req.(*ValidateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_CheckAccessLevel_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AccessCheckReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).CheckAccessLevel(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_CheckAccessLevel_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).CheckAccessLevel(ctx, req.(*AccessCheckReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_Login_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LoginUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).Login(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_Login_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).Login(ctx, req.(*LoginUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_ForgotPassword_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ForgotPasswordReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).ForgotPassword(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_ForgotPassword_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).ForgotPassword(ctx, req.(*ForgotPasswordReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_ResetPassword_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ResetPasswordReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).ResetPassword(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_ResetPassword_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).ResetPassword(ctx, req.(*ResetPasswordReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_UpdatePassword_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdatePasswordReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).UpdatePassword(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_UpdatePassword_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).UpdatePassword(ctx, req.(*UpdatePasswordReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_RequestForEmailVerification_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(EmailVerificationReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).RequestForEmailVerification(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_RequestForEmailVerification_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).RequestForEmailVerification(ctx, req.(*EmailVerificationReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_VerifyEmail_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(VerifyEmailReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).VerifyEmail(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_VerifyEmail_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).VerifyEmail(ctx, req.(*VerifyEmailReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_UpdateUsername_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateUsernameReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).UpdateUsername(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_UpdateUsername_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).UpdateUsername(ctx, req.(*UpdateUsernameReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_UpdatePasswordWithPassword_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdatePasswordWithPasswordReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).UpdatePasswordWithPassword(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_UpdatePasswordWithPassword_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).UpdatePasswordWithPassword(ctx, req.(*UpdatePasswordWithPasswordReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_UpdateEmailId_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateEmailIdReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).UpdateEmailId(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_UpdateEmailId_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).UpdateEmailId(ctx, req.(*UpdateEmailIdReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_GoogleLogin_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).GoogleLogin(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_GoogleLogin_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).GoogleLogin(ctx, req.(*RegisterUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthService_DecodeSignedJWT_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DecodeSignedJWTRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).DecodeSignedJWT(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuthService_DecodeSignedJWT_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).DecodeSignedJWT(ctx, req.(*DecodeSignedJWTRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// AuthService_ServiceDesc is the grpc.ServiceDesc for AuthService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var AuthService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "auth_svc.AuthService",
	HandlerType: (*AuthServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RegisterUser",
			Handler:    _AuthService_RegisterUser_Handler,
		},
		{
			MethodName: "Validate",
			Handler:    _AuthService_Validate_Handler,
		},
		{
			MethodName: "CheckAccessLevel",
			Handler:    _AuthService_CheckAccessLevel_Handler,
		},
		{
			MethodName: "Login",
			Handler:    _AuthService_Login_Handler,
		},
		{
			MethodName: "ForgotPassword",
			Handler:    _AuthService_ForgotPassword_Handler,
		},
		{
			MethodName: "ResetPassword",
			Handler:    _AuthService_ResetPassword_Handler,
		},
		{
			MethodName: "UpdatePassword",
			Handler:    _AuthService_UpdatePassword_Handler,
		},
		{
			MethodName: "RequestForEmailVerification",
			Handler:    _AuthService_RequestForEmailVerification_Handler,
		},
		{
			MethodName: "VerifyEmail",
			Handler:    _AuthService_VerifyEmail_Handler,
		},
		{
			MethodName: "UpdateUsername",
			Handler:    _AuthService_UpdateUsername_Handler,
		},
		{
			MethodName: "UpdatePasswordWithPassword",
			Handler:    _AuthService_UpdatePasswordWithPassword_Handler,
		},
		{
			MethodName: "UpdateEmailId",
			Handler:    _AuthService_UpdateEmailId_Handler,
		},
		{
			MethodName: "GoogleLogin",
			Handler:    _AuthService_GoogleLogin_Handler,
		},
		{
			MethodName: "DecodeSignedJWT",
			Handler:    _AuthService_DecodeSignedJWT_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "apis/serviceconn/gateway_authz/pb/gw_auth.proto",
}
