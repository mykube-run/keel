package transport

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	authpkg "github.com/mykube-run/keel/pkg/impl/auth"
	"github.com/mykube-run/keel/pkg/pb"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type GrpcSchedulerTransport struct {
	cfg            *config.TransportConfig
	lg             *zerolog.Logger
	omr            types.OnMessageReceived
	closeSend      bool
	closeReceiving bool
	srv            *grpc.Server
	lis            net.Listener
	streamsMu      sync.Mutex
	workerStreams  map[string]pb.Transport_ConnectServer
	provider       types.AuthProvider
	handlerWorkers map[string]map[string]struct{}
	workerHandlers map[string][]string
	workerLastBeat map[string]time.Time
}

func newGrpcSchedulerTransport(cfg *config.TransportConfig) (*GrpcSchedulerTransport, error) {
	lg := zerolog.New(os.Stdout).With().Timestamp().Str("tran", "grpc").Str("role", "scheduler").Logger()
	t := &GrpcSchedulerTransport{
		cfg:            cfg,
		lg:             &lg,
		workerStreams:  make(map[string]pb.Transport_ConnectServer),
		handlerWorkers: make(map[string]map[string]struct{}),
		workerHandlers: make(map[string][]string),
		workerLastBeat: make(map[string]time.Time),
	}
	if strings.EqualFold(cfg.Role, string(enum.TransportRoleScheduler)) {
		if strings.EqualFold(cfg.Grpc.Auth.Type, "simple") && len(cfg.Grpc.Auth.APIKeys) > 0 {
			t.provider = authpkg.NewSimpleProvider(cfg.Grpc.Auth.APIKeys)
		}
	}
	return t, nil
}

func (t *GrpcSchedulerTransport) Start() error {
	addr := t.cfg.Grpc.ListenAddress
	if strings.TrimSpace(addr) == "" {
		return fmt.Errorf("transport grpc listen address not specified")
	}
	lc, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	t.lis = lc

	opts := []grpc.ServerOption{}
	if t.cfg.Grpc.TLSEnable {
		tlsCfg, err := serverTLSConfig(t.cfg.Grpc)
		if err != nil {
			return err
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}
	opts = append(opts, grpc.StreamInterceptor(t.streamAuthInterceptor))

	t.srv = grpc.NewServer(opts...)
	pb.RegisterTransportServer(t.srv, &schedulerServer{t: t})
	go func() {
		if err := t.srv.Serve(t.lis); err != nil {
			t.lg.Error().Err(err).Msg("grpc transport server stopped")
		}
	}()
	t.lg.Info().Str("address", addr).Msg("grpc transport server started")
	go t.checkWorkers()
	return nil
}

func (t *GrpcSchedulerTransport) OnReceive(omr types.OnMessageReceived) {
	t.omr = omr
}

func (t *GrpcSchedulerTransport) Send(from, to string, msg []byte) error {
	var (
		attempt   = 0
		max       = t.retryMax()
		handler   string
		targetIds []string
		lastErr   error
	)

	for {
		if handler == "" {
			var task types.Task
			if err := json.Unmarshal(msg, &task); err == nil {
				handler = strings.TrimSpace(task.Handler)
			}
		}
		t.streamsMu.Lock()
		if handler != "" {
			if set, ok := t.handlerWorkers[handler]; ok && len(set) > 0 {
				targetIds = targetIds[:0]
				for id := range set {
					targetIds = append(targetIds, id)
				}
			} else {
				t.streamsMu.Unlock()
				return fmt.Errorf("no worker supports handler: %s", handler)
			}
		}

		var (
			pickId string
			sel    pb.Transport_ConnectServer
		)
		if len(targetIds) > 0 {
			pickId = targetIds[rand.Intn(len(targetIds))]
			sel = t.workerStreams[pickId]
		} else {
			n := len(t.workerStreams)
			if n == 0 {
				t.streamsMu.Unlock()
				return fmt.Errorf("no active worker streams")
			}
			i := rand.Intn(n)
			j := 0
			for k, s := range t.workerStreams {
				if j == i {
					sel = s
					pickId = k
					break
				}
				j++
			}
		}
		t.streamsMu.Unlock()
		env := &pb.Envelope{From: from, To: to, Payload: msg, TsSec: time.Now().Unix()}
		err := sel.Send(env)
		if err == nil {
			t.lg.Trace().Str("from", from).Str("to", to).Str("sample", sampling(msg)).Msg("sent message to worker")
			return nil
		}
		lastErr = err
		attempt++
		t.removeWorker(pickId)
		if attempt > max {
			return lastErr
		}
		t.backoff(attempt)
	}
}

func (t *GrpcSchedulerTransport) CloseReceive() error {
	t.closeReceiving = true
	return nil
}

func (t *GrpcSchedulerTransport) CloseSend() error {
	t.closeSend = true
	return nil
}

type schedulerServer struct {
	pb.UnimplementedTransportServer
	t *GrpcSchedulerTransport
}

func (s *schedulerServer) Connect(stream pb.Transport_ConnectServer) error {
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return fmt.Errorf("missing metadata")
	}
	vals := md.Get(identifierHeader)
	if len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
		return fmt.Errorf("missing worker id")
	}

	var (
		workerId = strings.TrimSpace(vals[0])
		handlers []string
	)

	if hv := md.Get(workerHandlersHeader); len(hv) > 0 && strings.TrimSpace(hv[0]) != "" {
		parts := strings.Split(hv[0], ",")
		for _, p := range parts {
			v := strings.TrimSpace(p)
			if v != "" {
				handlers = append(handlers, v)
			}
		}
	}
	s.t.registerWorker(workerId, stream, handlers)
	for {
		if s.t.closeReceiving {
			s.t.removeWorker(workerId)
			return nil
		}
		env, err := stream.Recv()
		if err != nil {
			s.t.lg.Err(err).Msg("stream recv error")
			s.t.removeWorker(workerId)
			return err
		}
		s.t.lg.Trace().Str("from", env.From).Str("to", env.To).Str("sample", sampling(env.Payload)).Msg("received message from worker")
		s.t.touchWorker(workerId)
		if env.To == heartbeatTopic {
			var hb struct {
				Handlers []string `json:"handlers"`
			}
			_ = json.Unmarshal(env.Payload, &hb)
			if len(hb.Handlers) > 0 {
				s.t.updateWorkerHandlers(workerId, hb.Handlers)
			}
			continue
		}
		if s.t.omr != nil {
			res, e := s.t.omr(env.From, env.To, env.Payload)
			if e != nil {
				s.t.lg.Err(e).Bytes("result", res).Msg("error handling message")
			}
		}
	}
}

func (t *GrpcSchedulerTransport) streamAuthInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	md, ok := metadata.FromIncomingContext(ss.Context())
	if !ok {
		return fmt.Errorf("missing metadata")
	}
	headers := make(map[string]string)
	for k, v := range md {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}
	if t.provider != nil {
		if err := t.provider.Authenticate(ss.Context(), headers); err != nil {
			return err
		}
	} else {
		vals := md.Get(apiKeyHeader)
		if len(vals) == 0 || strings.TrimSpace(vals[0]) == "" || vals[0] != t.cfg.Grpc.APIKey {
			return fmt.Errorf("unauthenticated")
		}
	}
	return handler(srv, ss)
}

func (t *GrpcSchedulerTransport) retryMax() int {
	if t.cfg.Grpc.SendRetryMax > 0 {
		return t.cfg.Grpc.SendRetryMax
	}
	return 3
}

func (t *GrpcSchedulerTransport) backoff(attempt int) {
	base := t.cfg.Grpc.SendRetryInitialBackoffMs
	if base <= 0 {
		base = 100
	}
	max := t.cfg.Grpc.SendRetryMaxBackoffMs
	if max <= 0 {
		max = 2000
	}
	jitter := t.cfg.Grpc.SendRetryJitterPct
	if jitter < 0 {
		jitter = 0
	}
	d := time.Duration(base) * time.Millisecond
	for i := 1; i < attempt; i++ {
		d = d * 2
		if d > time.Duration(max)*time.Millisecond {
			d = time.Duration(max) * time.Millisecond
			break
		}
	}
	if jitter > 0 {
		delta := int64(d) * int64(jitter) / 100
		j := rand.Int63n(delta*2) - delta
		d = time.Duration(int64(d) + j)
	}
	time.Sleep(d)
}

func (t *GrpcSchedulerTransport) registerWorker(workerId string, stream pb.Transport_ConnectServer, handlers []string) {
	t.streamsMu.Lock()
	t.workerStreams[workerId] = stream
	if len(handlers) > 0 {
		t.workerHandlers[workerId] = handlers
		for _, h := range handlers {
			set, ok := t.handlerWorkers[h]
			if !ok {
				set = make(map[string]struct{})
				t.handlerWorkers[h] = set
			}
			set[workerId] = struct{}{}
		}
	}
	t.streamsMu.Unlock()
	t.lg.Info().Str("workerId", workerId).Strs("handlers", handlers).Msg("worker stream registered")
}

func (t *GrpcSchedulerTransport) removeWorker(workerId string) {
	t.streamsMu.Lock()
	delete(t.workerStreams, workerId)
	delete(t.workerLastBeat, workerId)
	if hs, ok := t.workerHandlers[workerId]; ok {
		for _, h := range hs {
			if set, ok := t.handlerWorkers[h]; ok {
				delete(set, workerId)
				if len(set) == 0 {
					delete(t.handlerWorkers, h)
				}
			}
		}
	}
	delete(t.workerHandlers, workerId)
	t.streamsMu.Unlock()
}

func (t *GrpcSchedulerTransport) updateWorkerHandlers(workerId string, handlers []string) {
	t.streamsMu.Lock()
	if old, ok := t.workerHandlers[workerId]; ok {
		for _, h := range old {
			if set, ok := t.handlerWorkers[h]; ok {
				delete(set, workerId)
				if len(set) == 0 {
					delete(t.handlerWorkers, h)
				}
			}
		}
	}
	t.workerHandlers[workerId] = handlers
	for _, h := range handlers {
		set, ok := t.handlerWorkers[h]
		if !ok {
			set = make(map[string]struct{})
			t.handlerWorkers[h] = set
		}
		set[workerId] = struct{}{}
	}
	t.streamsMu.Unlock()
}

func (t *GrpcSchedulerTransport) touchWorker(workerId string) {
	t.streamsMu.Lock()
	t.workerLastBeat[workerId] = time.Now()
	t.streamsMu.Unlock()
}

func (t *GrpcSchedulerTransport) checkWorkers() {
	tick := time.NewTicker(15 * time.Second)
	for range tick.C {
		now := time.Now()
		t.streamsMu.Lock()
		for id, ts := range t.workerLastBeat {
			if now.Sub(ts) > 30*time.Second {
				delete(t.workerLastBeat, id)
				delete(t.workerStreams, id)
				if hs, ok := t.workerHandlers[id]; ok {
					for _, h := range hs {
						if set, ok := t.handlerWorkers[h]; ok {
							delete(set, id)
							if len(set) == 0 {
								delete(t.handlerWorkers, h)
							}
						}
					}
				}
				delete(t.workerHandlers, id)
			}
		}
		t.streamsMu.Unlock()
	}
}

func serverTLSConfig(gcfg config.GrpcConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(gcfg.TLSCertFile, gcfg.TLSKeyFile)
	if err != nil {
		return nil, err
	}
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	if gcfg.TLSCAFile != "" {
		ca, err := os.ReadFile(gcfg.TLSCAFile)
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(ca) {
			return nil, fmt.Errorf("failed to append ca")
		}
		tlsCfg.ClientCAs = pool
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	tlsCfg.InsecureSkipVerify = gcfg.InsecureSkipVerify
	return tlsCfg, nil
}
