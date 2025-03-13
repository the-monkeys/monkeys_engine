package service

import (
	"context"
	"time"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_cache/pb"
)

type GRPCServer struct {
	pb.UnimplementedCacheServiceServer
	cache *CacheServer
}

func NewGRPCServer(cache *CacheServer) *GRPCServer {
	return &GRPCServer{
		cache: cache,
	}
}

func (s *GRPCServer) Set(ctx context.Context, req *pb.SetRequest) (*pb.SetResponse, error) {
	expiration := time.Duration(req.ExpirationSeconds) * time.Second
	err := s.cache.Set(ctx, req.Key, req.Value, expiration)
	if err != nil {
		return &pb.SetResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.SetResponse{
		Success: true,
	}, nil
}

func (s *GRPCServer) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	value, err := s.cache.Get(ctx, req.Key)
	if err != nil {
		return &pb.GetResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return &pb.GetResponse{
			Success: false,
			Error:   "value is not []byte",
		}, nil
	}

	return &pb.GetResponse{
		Success: true,
		Value:   bytes,
	}, nil
}

func (s *GRPCServer) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	err := s.cache.Delete(ctx, req.Key)
	if err != nil {
		return &pb.DeleteResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.DeleteResponse{
		Success: true,
	}, nil
}

func (s *GRPCServer) Clear(ctx context.Context, req *pb.ClearRequest) (*pb.ClearResponse, error) {
	err := s.cache.Clear(ctx)
	if err != nil {
		return &pb.ClearResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.ClearResponse{
		Success: true,
	}, nil
}

func (s *GRPCServer) GetStats(ctx context.Context, req *pb.GetStatsRequest) (*pb.GetStatsResponse, error) {
	stats, err := s.cache.GetStats(ctx)
	if err != nil {
		return &pb.GetStatsResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	totalItems, _ := stats["total_items"].(int)
	expired, _ := stats["expired"].(int)
	active, _ := stats["active"].(int)

	return &pb.GetStatsResponse{
		Success:    true,
		TotalItems: int32(totalItems),
		Expired:    int32(expired),
		Active:     int32(active),
	}, nil
}
