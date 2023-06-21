package transport

import (
	"context"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/impl/transport/pb"
	"github.com/mykube-run/keel/pkg/queue"
	"github.com/mykube-run/keel/pkg/types"
	"google.golang.org/grpc"
	"net"
)

type GRPCTransport struct {
	cs  map[string]*pb.TransportServiceClient
	cfg *config.TransportConfig
	q   *queue.PriorityQueue
	s   *GRPCTransportServer
}

func NewGRPCTransport(cfg *config.TransportConfig) (*GRPCTransport, error) {
	t := &GRPCTransport{
		s: new(GRPCTransportServer),
	}
	return t, nil
}

func (t *GRPCTransport) Start() error {
	return nil
}

func (t *GRPCTransport) OnReceive(omr types.OnMessageReceived) {
	t.s.hdl = omr
}

func (t *GRPCTransport) Send(from, to string, msg []byte) error {
	return nil
}

func (t *GRPCTransport) CloseSend() error {
	return nil
}

func (t *GRPCTransport) CloseReceive() error {
	return nil
}

type GRPCTransportServer struct {
	pb.UnimplementedTransportServiceServer

	srv *grpc.Server
	hdl types.OnMessageReceived
}

func (s *GRPCTransportServer) Send(ctx context.Context, req *pb.Request) (resp *pb.Response, err error) {
	msg, err := s.hdl(req.From.Name, req.To.Name, req.Msg)
	if err != nil {
		return nil, err
	}
	resp = &pb.Response{
		Msg: msg,
	}
	return resp, nil
}

func (s *GRPCTransportServer) start(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.srv = grpc.NewServer()
	pb.RegisterTransportServiceServer(s.srv, s)
	return s.srv.Serve(lis)
}

func (s *GRPCTransportServer) close() error {
	s.srv.GracefulStop()
	return nil
}
