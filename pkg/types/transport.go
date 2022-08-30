package types

type OnMessageReceived func(from string, msg []byte) (result []byte, err error)

type Transport interface {
	OnReceive(fn OnMessageReceived)
	Send(from, to string, msg []byte) error
	Close() error
}
