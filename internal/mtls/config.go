package mtls

// Config holds mTLS configuration for clients/servers.
type Config struct {
	Enabled bool
	Cert    string
}

// New creates a new Config.
func New(enabled bool, cert string) *Config {
	return &Config{Enabled: enabled, Cert: cert}
}

// Valid returns true if the configuration is valid.
func (c *Config) Valid() bool {
	if c == nil {
		return false
	}
	if !c.Enabled {
		return true
	}
	return c.Cert != ""
}
