package privacy

import (
	"context"
	"fmt"
	"io"
)

// Mode represents the privacy transport mode.
type Mode int

const (
	ModePlain Mode = iota // no encryption, direct connection
	ModeTLS               // end-to-end encrypted messages over P2P
	ModeTor               // route through Tor SOCKS5 proxy for IP anonymity
	ModeFull              // Tor + TLS combined (maximum privacy)
)

func (m Mode) String() string {
	switch m {
	case ModePlain:
		return "plain"
	case ModeTLS:
		return "tls"
	case ModeTor:
		return "tor"
	case ModeFull:
		return "full"
	default:
		return "unknown"
	}
}

// ParseMode converts a string to a Mode.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "plain":
		return ModePlain, nil
	case "":
		return ModeTLS, nil
	case "tls":
		return ModeTLS, nil
	case "tor":
		return ModeTor, nil
	case "full":
		return ModeFull, nil
	default:
		return ModePlain, fmt.Errorf("unknown privacy mode: %s", s)
	}
}

// Transport is the interface for privacy-enhanced message transport.
// All P2P messages in the SDK pass through a Transport before being sent.
type Transport interface {
	// Wrap encrypts or wraps outgoing message data.
	Wrap(ctx context.Context, data []byte, recipientPubkey []byte) ([]byte, error)

	// Unwrap decrypts or unwraps incoming message data.
	Unwrap(ctx context.Context, data []byte) ([]byte, error)

	// Close releases any resources held by the transport.
	Close() error
}

// ProxyDialer provides SOCKS5-compatible dialing for Tor integration.
type ProxyDialer interface {
	// DialContext connects through the proxy.
	DialContext(ctx context.Context, network, addr string) (io.ReadWriteCloser, error)

	// ProxyAddr returns the SOCKS5 proxy address (e.g., "127.0.0.1:9050").
	ProxyAddr() string
}

// NewTransport creates a Transport for the given mode.
func NewTransport(mode Mode, opts ...Option) (Transport, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	switch mode {
	case ModePlain:
		return &plainTransport{}, nil
	case ModeTLS:
		return newTLSTransport(cfg)
	case ModeTor:
		return newTorTransport(cfg)
	case ModeFull:
		return newFullTransport(cfg)
	default:
		return &plainTransport{}, nil
	}
}

// plainTransport passes messages through unmodified.
type plainTransport struct{}

func (p *plainTransport) Wrap(_ context.Context, data []byte, _ []byte) ([]byte, error) {
	return data, nil
}

func (p *plainTransport) Unwrap(_ context.Context, data []byte) ([]byte, error) {
	return data, nil
}

func (p *plainTransport) Close() error { return nil }
