package privacy

// config holds internal configuration for privacy transports.
type config struct {
	// TLS options
	localPrivKey []byte // X25519 private key (32 bytes)
	localPubKey  []byte // X25519 public key (32 bytes)

	// Tor options
	torSocksAddr string // SOCKS5 proxy address (default "127.0.0.1:9050")
	torEnabled   bool

	// Encryption options
	aeadKeySize int // AES key size in bytes (default 32 = AES-256)
}

func defaultConfig() *config {
	return &config{
		torSocksAddr: "127.0.0.1:9050",
		aeadKeySize:  32,
	}
}

// Option configures a privacy transport.
type Option func(*config)

// WithLocalKeys sets the X25519 keypair for TLS transport.
func WithLocalKeys(privKey, pubKey []byte) Option {
	return func(c *config) {
		c.localPrivKey = privKey
		c.localPubKey = pubKey
	}
}

// WithTorAddr sets the Tor SOCKS5 proxy address.
func WithTorAddr(addr string) Option {
	return func(c *config) {
		c.torSocksAddr = addr
		c.torEnabled = true
	}
}
