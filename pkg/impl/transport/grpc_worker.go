package transport

import (
	"context"
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
	"github.com/mykube-run/keel/pkg/pb"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type GrpcWorkerTransport struct {
	cfg            *config.TransportConfig
	lg             *zerolog.Logger
	omr            types.OnMessageReceived
	closeSend      bool
	closeReceiving bool
	clientsMu      sync.RWMutex
	clients        map[string]pb.Transport_ConnectClient
	endpointById   map[string]string
	handlers       []string
	hbInterval     time.Duration
	hbStarted      bool
}

func newGrpcWorkerTransport(cfg *config.TransportConfig) (*GrpcWorkerTransport, error) {
	lg := zerolog.New(os.Stdout).With().Timestamp().Str("tran", "grpc").Str("role", "worker").Logger()

	interval := 10 * time.Second
	if cfg.Grpc.HeartbeatInterval > 0 {
		interval = time.Duration(cfg.Grpc.HeartbeatInterval) * time.Second
	}
	t := &GrpcWorkerTransport{
		cfg:          cfg,
		lg:           &lg,
		clients:      make(map[string]pb.Transport_ConnectClient),
		endpointById: make(map[string]string),
		handlers:     []string{},
		hbInterval:   interval,
	}
	return t, nil
}

func (t *GrpcWorkerTransport) Start() error {
	targets, err := t.resolveSchedulerTargets()
	if err != nil {
		return err
	}
	for _, target := range targets {
		dialOpts := []grpc.DialOption{grpc.WithBlock()}
		if t.cfg.Grpc.TLSEnable {
			tlsCfg, err := clientTLSConfig(t.cfg.Grpc)
			if err != nil {
				return err
			}
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
		} else {
			dialOpts = append(dialOpts, grpc.WithInsecure())
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		conn, err := grpc.DialContext(ctx, target, dialOpts...)
		cancel()
		if err != nil {
			return err
		}
		cli := pb.NewTransportClient(conn)
		hdr := map[string]string{
			apiKeyHeader:     t.cfg.Grpc.APIKey,
			identifierHeader: t.cfg.Identifier,
		}
		if len(t.handlers) > 0 {
			hdr[workerHandlersHeader] = strings.Join(t.handlers, ",")
		}
		md := metadata.New(hdr)
		cctx := metadata.NewOutgoingContext(context.Background(), md)
		stream, err := cli.Connect(cctx)
		if err != nil {
			return err
		}
		h, _ := stream.Header()
		sid := ""
		if h != nil {
			if vs := h.Get(identifierHeader); len(vs) > 0 {
				sid = strings.TrimSpace(vs[0])
			}
		}
		if sid == "" {
			sid = target
		}
		t.clientsMu.Lock()
		t.clients[sid] = stream
		t.endpointById[sid] = target
		t.clientsMu.Unlock()
		go t.consumeClient(sid, stream)
		t.lg.Info().Str("target", target).Str("schedulerId", sid).Msg("grpc transport client connected")
	}
	t.ensureHeartbeat()
	return nil
}

func (t *GrpcWorkerTransport) OnReceive(omr types.OnMessageReceived) {
	t.omr = omr
}

func (t *GrpcWorkerTransport) Send(from, to string, msg []byte) error {
	if to == heartbeatTopic {
		t.clientsMu.RLock()
		for sid, c := range t.clients {
			env := &pb.Envelope{From: from, To: to, Payload: msg, TsSec: time.Now().Unix()}
			if err := c.Send(env); err != nil && t.cfg.Grpc.ReconnectOnSendError {
				_ = t.reconnectClientById(sid)
				t.clientsMu.RUnlock()
				t.clientsMu.RLock()
			}
		}
		t.clientsMu.RUnlock()
		return nil
	}
	parts := strings.SplitN(to, ":", 2)
	sid := strings.TrimSpace(parts[0])
	attempt := 0
	max := t.retryMax()
	for {
		t.clientsMu.RLock()
		c := t.clients[sid]
		t.clientsMu.RUnlock()
		if c == nil {
			return fmt.Errorf("no scheduler stream: %s", sid)
		}
		env := &pb.Envelope{From: from, To: to, Payload: msg, TsSec: time.Now().Unix()}
		err := c.Send(env)
		if err == nil {
			t.lg.Trace().Str("from", from).Str("to", to).Str("sample", sampling(msg)).Msg("sent message to scheduler")
			return nil
		}
		attempt++
		if !t.cfg.Grpc.ReconnectOnSendError || attempt > max {
			return err
		}
		_ = t.reconnectClientById(sid)
		t.backoff(attempt)
	}
}

func (t *GrpcWorkerTransport) CloseReceive() error {
	t.closeReceiving = true
	return nil
}

func (t *GrpcWorkerTransport) CloseSend() error {
	t.closeSend = true
	return nil
}

func (t *GrpcWorkerTransport) consumeClient(sid string, c pb.Transport_ConnectClient) {
	for {
		if t.closeReceiving {
			return
		}
		env, err := c.Recv()
		if err != nil {
			t.lg.Err(err).Str("schedulerId", sid).Msg("client recv error")
			if e := t.reconnectClientById(sid); e == nil {
				t.clientsMu.RLock()
				nc := t.clients[sid]
				t.clientsMu.RUnlock()
				if nc != nil {
					go t.consumeClient(sid, nc)
				}
			}
			return
		}
		if t.omr != nil {
			res, e := t.omr(env.From, env.To, env.Payload)
			if e != nil {
				t.lg.Err(e).Bytes("result", res).Msg("error handling message")
			}
		}
	}
}

func (t *GrpcWorkerTransport) resolveSchedulerTargets() ([]string, error) {
	switch strings.ToLower(t.cfg.Grpc.Mode) {
	case "static":
		return append([]string{}, t.cfg.Grpc.SchedulerEndpoints...), nil
	case "dns":
		port := t.cfg.Grpc.Port
		if port <= 0 {
			port = 443
		}
		return resolveDNSAll(t.cfg.Grpc.DNSName, port)
	case "k8s":
		host := fmt.Sprintf("%s.%s.svc.cluster.local", t.cfg.Grpc.K8SService, t.cfg.Grpc.K8SNamespace)
		port := t.cfg.Grpc.Port
		if port <= 0 {
			port = 443
		}
		return resolveDNSAll(host, port)
	default:
		return nil, fmt.Errorf("unsupported grpc discovery mode: %s", t.cfg.Grpc.Mode)
	}
}

func resolveDNSAll(name string, port int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no address resolved")
	}
	res := make([]string, 0, len(addrs))
	for _, a := range addrs {
		res = append(res, fmt.Sprintf("%s:%d", a, port))
	}
	return res, nil
}

func clientTLSConfig(gcfg config.GrpcConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{}
	if gcfg.TLSCAFile != "" {
		ca, err := os.ReadFile(gcfg.TLSCAFile)
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(ca) {
			return nil, fmt.Errorf("failed to append ca")
		}
		tlsCfg.RootCAs = pool
	}
	if gcfg.TLSCertFile != "" && gcfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(gcfg.TLSCertFile, gcfg.TLSKeyFile)
		if err != nil {
			return nil, err
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	tlsCfg.InsecureSkipVerify = gcfg.InsecureSkipVerify
	return tlsCfg, nil
}

func (t *GrpcWorkerTransport) retryMax() int {
	if t.cfg.Grpc.SendRetryMax > 0 {
		return t.cfg.Grpc.SendRetryMax
	}
	return 3
}

func (t *GrpcWorkerTransport) backoff(attempt int) {
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

func (t *GrpcWorkerTransport) reconnectClientById(sid string) error {
	t.clientsMu.RLock()
	target := t.endpointById[sid]
	t.clientsMu.RUnlock()
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("unknown scheduler: %s", sid)
	}
	dialOpts := []grpc.DialOption{grpc.WithBlock()}
	if t.cfg.Grpc.TLSEnable {
		tlsCfg, err := clientTLSConfig(t.cfg.Grpc)
		if err != nil {
			return err
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	conn, err := grpc.DialContext(ctx, target, dialOpts...)
	cancel()
	if err != nil {
		return err
	}
	cli := pb.NewTransportClient(conn)
	hdr := map[string]string{apiKeyHeader: t.cfg.Grpc.APIKey, identifierHeader: t.cfg.Identifier}
	if len(t.handlers) > 0 {
		hdr[workerHandlersHeader] = strings.Join(t.handlers, ",")
	}
	md := metadata.New(hdr)
	cctx := metadata.NewOutgoingContext(context.Background(), md)
	stream, err := cli.Connect(cctx)
	if err != nil {
		return err
	}
	t.clientsMu.Lock()
	t.clients[sid] = stream
	t.clientsMu.Unlock()
	t.ensureHeartbeat()
	return nil
}

func (t *GrpcWorkerTransport) ensureHeartbeat() {
	if t.hbStarted {
		return
	}
	t.hbStarted = true
	go t.startHeartbeat()
}

func (t *GrpcWorkerTransport) startHeartbeat() {
	tick := time.NewTicker(t.hbInterval)
	for range tick.C {
		if t.closeSend || t.closeReceiving {
			return
		}
		payload, _ := json.Marshal(map[string]interface{}{"handlers": t.handlers})
		_ = t.Send(t.cfg.Identifier, heartbeatTopic, payload)
	}
}

func (t *GrpcWorkerTransport) setHandlers(handlers []string) {
	t.handlers = handlers
	payload, _ := json.Marshal(map[string]interface{}{"handlers": t.handlers})
	_ = t.Send(t.cfg.Identifier, heartbeatTopic, payload)
}
