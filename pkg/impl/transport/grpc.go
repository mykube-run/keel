package transport

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "github.com/mykube-run/keel/pkg/config"
    "github.com/mykube-run/keel/pkg/enum"
    "github.com/mykube-run/keel/pkg/pb"
    "github.com/mykube-run/keel/pkg/types"
    authpkg "github.com/mykube-run/keel/pkg/impl/auth"
    "github.com/rs/zerolog"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/metadata"
    "math/rand"
    "net"
    "os"
    "strings"
    "sync"
    "time"
)

const apiKeyHeader = "x-api-key"

type GrpcTransport struct {
    cfg            *config.TransportConfig
    lg             *zerolog.Logger
    omr            types.OnMessageReceived
    closeSend      bool
    closeReceiving bool
    srv            *grpc.Server
    lis            net.Listener
    streamsMu      sync.Mutex
    workerStreams  map[string]pb.Transport_ConnectServer
    client         pb.Transport_ConnectClient
    provider       types.AuthProvider
}

func NewGrpcTransport(cfg *config.TransportConfig) (*GrpcTransport, error) {
    if err := validateGrpcConfig(cfg); err != nil {
        return nil, err
    }
    lg := zerolog.New(os.Stdout).With().Timestamp().Str("tran", "grpc").Logger()
    t := &GrpcTransport{cfg: cfg, lg: &lg, workerStreams: make(map[string]pb.Transport_ConnectServer)}
    if strings.ToLower(cfg.Role) == strings.ToLower(string(enum.TransportRoleScheduler)) {
        if strings.ToLower(cfg.Grpc.Auth.Type) == "simple" && len(cfg.Grpc.Auth.APIKeys) > 0 {
            t.provider = authpkg.NewSimpleProvider(cfg.Grpc.Auth.APIKeys)
        }
    }
    return t, nil
}

func (t *GrpcTransport) Start() error {
    if t.cfg.Role == string(enum.TransportRoleScheduler) {
        return t.startServer()
    }
    return t.startClient()
}

func (t *GrpcTransport) OnReceive(omr types.OnMessageReceived) {
    t.omr = omr
}

func (t *GrpcTransport) Send(from, to string, msg []byte) error {
    if t.cfg.Role == string(enum.TransportRoleScheduler) {
        return t.sendToWorker(from, to, msg)
    }
    return t.sendToScheduler(from, to, msg)
}

func (t *GrpcTransport) CloseReceive() error {
    t.closeReceiving = true
    return nil
}

func (t *GrpcTransport) CloseSend() error {
    t.closeSend = true
    return nil
}

func (t *GrpcTransport) startServer() error {
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
    pb.RegisterTransportServer(t.srv, &transportServer{t: t})
    go func() {
        if err := t.srv.Serve(t.lis); err != nil {
            t.lg.Error().Err(err).Msg("grpc transport server stopped")
        }
    }()
    t.lg.Info().Str("listen", addr).Msg("grpc transport server started")
    return nil
}

func (t *GrpcTransport) startClient() error {
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

    md := metadata.New(map[string]string{apiKeyHeader: t.cfg.Grpc.APIKey})
    cctx := metadata.NewOutgoingContext(context.Background(), md)
    stream, err := cli.Connect(cctx)
    if err != nil {
        return err
    }
    t.client = stream
    go t.consumeClient()
    t.lg.Info().Str("target", target).Msg("grpc transport client connected")
    return nil
}

func (t *GrpcTransport) sendToWorker(from, to string, msg []byte) error {
    attempt := 0
    max := t.retryMax()
    for {
        t.streamsMu.Lock()
        n := len(t.workerStreams)
        if n == 0 {
            t.streamsMu.Unlock()
            return fmt.Errorf("no active worker streams")
        }
        i := rand.Intn(n)
        var sel pb.Transport_ConnectServer
        var pickKey string
        j := 0
        for k, s := range t.workerStreams {
            if j == i {
                sel = s
                pickKey = k
                break
            }
            j++
        }
        t.streamsMu.Unlock()
        env := &pb.Envelope{From: from, To: to, Payload: msg, TsSec: time.Now().Unix()}
        err := sel.Send(env)
        if err == nil {
            t.lg.Trace().Str("from", from).Str("to", to).Str("sample", sampling(msg)).Msg("sent message to worker")
            return nil
        }
        attempt++
        t.removeWorkerStream(pickKey)
        if attempt > max {
            return err
        }
        t.backoff(attempt)
    }
}

func (t *GrpcTransport) sendToScheduler(from, to string, msg []byte) error {
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

type transportServer struct{ pb.UnimplementedTransportServer; t *GrpcTransport }

func (s *transportServer) Connect(stream pb.Transport_ConnectServer) error {
    var workerId string
    for {
        if s.t.closeReceiving {
            return nil
        }
        env, err := stream.Recv()
        if err != nil {
            s.t.lg.Err(err).Msg("stream recv error")
            return err
        }
        if workerId == "" && strings.TrimSpace(env.From) != "" {
            s.t.streamsMu.Lock()
            s.t.workerStreams[env.From] = stream
            s.t.streamsMu.Unlock()
            workerId = env.From
            s.t.lg.Info().Str("workerId", workerId).Msg("worker stream registered")
        }
        if s.t.omr != nil {
            res, e := s.t.omr(env.From, env.To, env.Payload)
            if e != nil {
                s.t.lg.Err(e).Bytes("result", res).Msg("error handling message")
            }
        }
    }
}

func (t *GrpcTransport) consumeClient() {
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

func (t *GrpcTransport) streamAuthInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
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

func validateGrpcConfig(cfg *config.TransportConfig) error {
    if cfg.Role != string(enum.TransportRoleScheduler) && cfg.Role != string(enum.TransportRoleWorker) {
        return fmt.Errorf("TransportConfig.Role was not specified")
    }
    if cfg.Role == string(enum.TransportRoleScheduler) {
        if strings.TrimSpace(cfg.Grpc.ListenAddress) == "" {
            return fmt.Errorf("TransportConfig.Grpc.ListenAddress was not specified")
        }
    } else {
        if strings.TrimSpace(cfg.Grpc.Mode) == "" {
            return fmt.Errorf("TransportConfig.Grpc.Mode was not specified")
        }
        if cfg.Grpc.Mode == "static" && len(cfg.Grpc.SchedulerEndpoints) == 0 {
            return fmt.Errorf("TransportConfig.Grpc.SchedulerEndpoints was not specified")
        }
        if cfg.Grpc.Mode == "dns" && strings.TrimSpace(cfg.Grpc.DNSName) == "" {
            return fmt.Errorf("TransportConfig.Grpc.DNSName was not specified")
        }
        if cfg.Grpc.Mode == "k8s" && (strings.TrimSpace(cfg.Grpc.K8SNamespace) == "" || strings.TrimSpace(cfg.Grpc.K8SService) == "") {
            return fmt.Errorf("TransportConfig.Grpc.K8S fields were not specified")
        }
    }
    if strings.TrimSpace(cfg.Grpc.APIKey) == "" {
        return fmt.Errorf("TransportConfig.Grpc.APIKey was not specified")
    }
    return nil
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

func (t *GrpcTransport) resolveSchedulerTarget() (string, error) {
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

func (t *GrpcTransport) retryMax() int {
    if t.cfg.Grpc.SendRetryMax > 0 {
        return t.cfg.Grpc.SendRetryMax
    }
    return 3
}

func (t *GrpcTransport) backoff(attempt int) {
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

func (t *GrpcTransport) removeWorkerStream(key string) {
    t.streamsMu.Lock()
    delete(t.workerStreams, key)
    t.streamsMu.Unlock()
}

func (t *GrpcTransport) reconnectClient() error {
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
    md := metadata.New(map[string]string{apiKeyHeader: t.cfg.Grpc.APIKey})
    cctx := metadata.NewOutgoingContext(context.Background(), md)
    stream, err := cli.Connect(cctx)
    if err != nil {
        return err
    }
    t.client = stream
    return nil
}