package utils

import (
	"testing"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_authz/pb"
)

// TODO: Implement the test cases

func TestIpClientConvert(t *testing.T) {
	type args struct {
		ip     string
		client string
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 string
	}{
		{
			name: "Test 1: Undefined client",
			args: args{
				ip:     "127.0.0.1",
				client: "test",
			},
			want:  "127.0.0.1",
			want1: "Others",
		},

		{
			name: "Test 1: No Ip",
			args: args{
				ip:     "",
				client: "Chrome",
			},
			want:  "127.0.0.1",
			want1: "Chrome",
		},
		{
			name: "Test 1: Correct Ip and Client",
			args: args{
				ip:     "127.0.0.1",
				client: "Safari",
			},
			want:  "127.0.0.1",
			want1: "Safari",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := IpClientConvert(tt.args.ip, tt.args.client)
			if got != tt.want {
				t.Errorf("IpClientConvert() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("IpClientConvert() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestValidateRegisterUserRequest(t *testing.T) {
	type args struct {
		req *pb.RegisterUserRequest
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Test 1",
			args: args{
				req: &pb.RegisterUserRequest{
					FirstName: "john",
					LastName:  "mayer",
					Email:     "john.mayer",
					Password:  "123",
				},
			},
			wantErr: false,
		},
		{
			name: "Test 2",
			args: args{
				req: &pb.RegisterUserRequest{
					FirstName: "",
					LastName:  "",
					Email:     ".",
					Password:  "",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateRegisterUserRequest(tt.args.req); (err != nil) != tt.wantErr {
				t.Errorf("ValidateRegisterUserRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
