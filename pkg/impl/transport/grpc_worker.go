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
	client         pb.Transport_ConnectClient
	workerId       string
	handlers       []string
	hbInterval     time.Duration
	hbStarted      bool
}

func newGrpcWorkerTransport(cfg *config.TransportConfig) (*GrpcWorkerTransport, error) {
	lg := zerolog.New(os.Stdout).With().Timestamp().Str("tran", "grpc").Str("role", "worker").Logger()
	id := strings.TrimSpace(os.Getenv("KEEL_WORKER_ID"))
	if id == "" {
		hn, _ := os.Hostname()
		id = strings.TrimSpace(hn)
		if id == "" {
			id = "unknown"
		}
	}
	var handlers []string
	if raw := strings.TrimSpace(os.Getenv("KEEL_WORKER_HANDLERS")); raw != "" {
		parts := strings.Split(raw, ",")
		for _, p := range parts {
			v := strings.TrimSpace(p)
			if v != "" {
				handlers = append(handlers, v)
			}
		}
	}
	interval := 10 * time.Second
	if v := strings.TrimSpace(os.Getenv("KEEL_WORKER_HEARTBEAT_SECONDS")); v != "" {
		if n, err := time.ParseDuration(v + "s"); err == nil && n > 0 {
			interval = n
		}
	}
	t := &GrpcWorkerTransport{cfg: cfg, lg: &lg, workerId: id, handlers: handlers, hbInterval: interval}
	return t, nil
}

func (t *GrpcWorkerTransport) Start() error {
	target, err := t.resolveSchedulerTarget()
	if err != nil {
		return err
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, target, dialOpts...)
	if err != nil {
		return err
	}
	cli := pb.NewTransportClient(conn)
	hdr := map[string]string{apiKeyHeader: t.cfg.Grpc.APIKey, workerIdHeader: t.workerId}
	if len(t.handlers) > 0 {
		hdr[workerHandlersHeader] = strings.Join(t.handlers, ",")
	}
	md := metadata.New(hdr)
	cctx := metadata.NewOutgoingContext(context.Background(), md)
	stream, err := cli.Connect(cctx)
	if err != nil {
		return err
	}
	t.client = stream
	go t.consumeClient()
	t.ensureHeartbeat()
	t.lg.Info().Str("target", target).Msg("grpc transport client connected")
	return nil
}

func (t *GrpcWorkerTransport) OnReceive(omr types.OnMessageReceived) {
	t.omr = omr
}

func (t *GrpcWorkerTransport) Send(from, to string, msg []byte) error {
	attempt := 0
	max := t.retryMax()
	for {
		if t.client == nil {
			if !t.cfg.Grpc.ReconnectOnSendError {
				return fmt.Errorf("no scheduler stream")
			}
			if err := t.reconnectClient(); err != nil {
				return err
			}
		}
		env := &pb.Envelope{From: from, To: to, Payload: msg, TsSec: time.Now().Unix()}
		err := t.client.Send(env)
		if err == nil {
			t.lg.Trace().Str("from", from).Str("to", to).Str("sample", sampling(msg)).Msg("sent message to scheduler")
			return nil
		}
		attempt++
		if !t.cfg.Grpc.ReconnectOnSendError || attempt > max {
			return err
		}
		_ = t.reconnectClient()
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

func (t *GrpcWorkerTransport) consumeClient() {
	for {
		if t.closeReceiving {
			return
		}
		env, err := t.client.Recv()
		if err != nil {
			t.lg.Err(err).Msg("client recv error")
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

func (t *GrpcWorkerTransport) resolveSchedulerTarget() (string, error) {
	switch strings.ToLower(t.cfg.Grpc.Mode) {
	case "static":
		return t.cfg.Grpc.SchedulerEndpoints[rand.Intn(len(t.cfg.Grpc.SchedulerEndpoints))], nil
	case "dns":
		return resolveDNS(t.cfg.Grpc.DNSName)
	case "k8s":
		host := fmt.Sprintf("%s.%s.svc.cluster.local", t.cfg.Grpc.K8SService, t.cfg.Grpc.K8SNamespace)
		return resolveDNS(host)
	default:
		return "", fmt.Errorf("unsupported grpc discovery mode: %s", t.cfg.Grpc.Mode)
	}
}

func resolveDNS(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(ctx, name)
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("no address resolved")
	}
	return fmt.Sprintf("%s:443", addrs[rand.Intn(len(addrs))]), nil
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

func (t *GrpcWorkerTransport) reconnectClient() error {
	target, err := t.resolveSchedulerTarget()
	if err != nil {
		return err
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
	defer cancel()
	conn, err := grpc.DialContext(ctx, target, dialOpts...)
	if err != nil {
		return err
	}
	cli := pb.NewTransportClient(conn)
	hdr := map[string]string{apiKeyHeader: t.cfg.Grpc.APIKey, workerIdHeader: t.workerId}
	if len(t.handlers) > 0 {
		hdr[workerHandlersHeader] = strings.Join(t.handlers, ",")
	}
	md := metadata.New(hdr)
	cctx := metadata.NewOutgoingContext(context.Background(), md)
	stream, err := cli.Connect(cctx)
	if err != nil {
		return err
	}
	t.client = stream
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
	for {
		select {
		case <-tick.C:
			if t.closeSend || t.closeReceiving {
				return
			}
			payload, _ := json.Marshal(map[string]interface{}{"handlers": t.handlers})
			_ = t.Send(t.workerId, heartbeatTopic, payload)
		}
	}
}
