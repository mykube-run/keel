package types

type OnMessageReceived func(from string, msg []byte) (result []byte, err error)

type Transport interface {
	// Start starts the transport
	Start() error

	// OnReceive registers a message handler, which is called every time a message is received
	OnReceive(fn OnMessageReceived)

	// Send sends message in bytes
	Send(from, to string, msg []byte) error

	// CloseReceive closes receive side, stops receiving or handling messages
	CloseReceive() error

	// CloseSend closes send side, stops sending messages
	CloseSend() error
}
