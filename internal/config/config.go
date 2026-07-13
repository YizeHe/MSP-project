package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// BuiltInSeeds used only for first join; disconnected after mesh has peers.
var BuiltInSeeds = []string{
	"https://msp.forbiddenx.top",
	"https://msp-seed.tangent2533.workers.dev",
}

// Config is local node settings (no messages).
type Config struct {
	// Seeds is the user-managed list (manual). Prefer first for primary.
	Seeds []string `json:"seeds"`
	// SeedURL legacy single seed (migrated into Seeds).
	SeedURL string `json:"seed_url,omitempty"`

	IdentityPath  string `json:"identity_path"`
	PoWDifficulty int    `json:"pow_difficulty"`
	// PoWTargetMS target mine time for adaptive difficulty (0=use fixed bits).
	PoWTargetMS int `json:"pow_target_ms"`
	// Alert target uses higher multiplier.
	AlertPoWTargetMS int `json:"alert_pow_target_ms"`

	PollIntervalS int    `json:"poll_interval_sec"`
	DataDir       string `json:"data_dir"`
	UDPPort       int    `json:"udp_port"`

	// AlertPubKeyB64 optional network alert authority (Ed25519). Empty = accept any alert flag (dev).
	AlertPubKeyB64 string `json:"alert_pubkey,omitempty"`

	// EnforceClock enables ±500ms skew check.
	EnforceClock bool `json:"enforce_clock"`
	// ForwardDelay enables random 10ms-3s forward delay.
	ForwardDelay bool `json:"forward_delay"`
	// PadDatagram enables fixed-size UDP padding.
	PadDatagram bool `json:"pad_datagram"`

	// UseBuiltInSeeds allows fallback to BuiltInSeeds when Seeds empty / first boot.
	UseBuiltInSeeds bool `json:"use_builtin_seeds"`
	// StopBuiltInAfterPeers once mesh has N peers, stop refreshing built-in seeds.
	StopBuiltInAfterPeers int `json:"stop_builtin_after_peers"`

	// IdentityPassphrase empty = plaintext mnemonic file (not recommended).
	// If set in env MSP_PASSPHRASE preferred at runtime, not stored.
}

// DefaultDir returns ./msp-data relative to cwd.
func DefaultDir() string { return "msp-data" }

// Default returns v0.3 defaults.
func Default() *Config {
	dir := DefaultDir()
	return &Config{
		Seeds:                 nil,
		IdentityPath:          filepath.Join(dir, "identity.json"),
		PoWDifficulty:         16,
		PoWTargetMS:           0, // 0 = fixed bits mode for interactive UX
		AlertPoWTargetMS:      0,
		PollIntervalS:         2,
		DataDir:               dir,
		UDPPort:               0,
		EnforceClock:          true,
		ForwardDelay:          true,
		PadDatagram:           true,
		UseBuiltInSeeds:       true,
		StopBuiltInAfterPeers: 1,
	}
}

// Path of config file.
func Path(dataDir string) string {
	if dataDir == "" {
		dataDir = DefaultDir()
	}
	return filepath.Join(dataDir, "config.json")
}

// Load reads config or returns defaults.
func Load(dataDir string) (*Config, error) {
	p := Path(dataDir)
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			c := Default()
			if dataDir != "" {
				c.DataDir = dataDir
				c.IdentityPath = filepath.Join(dataDir, "identity.json")
			}
			return c, nil
		}
		return nil, err
	}
	c := Default()
	if err := json.Unmarshal(raw, c); err != nil {
		return nil, err
	}
	if c.DataDir == "" {
		c.DataDir = dataDir
		if c.DataDir == "" {
			c.DataDir = DefaultDir()
		}
	}
	if c.IdentityPath == "" {
		c.IdentityPath = filepath.Join(c.DataDir, "identity.json")
	}
	if c.PoWDifficulty <= 0 {
		c.PoWDifficulty = 16
	}
	if c.PollIntervalS <= 0 {
		c.PollIntervalS = 2
	}
	// migrate legacy SeedURL
	if c.SeedURL != "" {
		c.AddSeed(c.SeedURL)
		c.SeedURL = ""
	}
	return c, nil
}

// Save persists config.
func Save(c *Config) error {
	if err := os.MkdirAll(c.DataDir, 0o700); err != nil {
		return err
	}
	// do not persist legacy field
	c.SeedURL = ""
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(c.DataDir), raw, 0o600)
}

// PrimarySeed returns first custom seed or first built-in.
func (c *Config) PrimarySeed() string {
	if len(c.Seeds) > 0 {
		return c.Seeds[0]
	}
	if c.UseBuiltInSeeds && len(BuiltInSeeds) > 0 {
		return BuiltInSeeds[0]
	}
	return ""
}

// AllSeeds returns custom + optionally built-in (deduped).
func (c *Config) AllSeeds(includeBuiltIn bool) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, s := range c.Seeds {
		add(s)
	}
	if includeBuiltIn && c.UseBuiltInSeeds {
		for _, s := range BuiltInSeeds {
			add(s)
		}
	}
	return out
}

// AddSeed appends unique seed URL.
func (c *Config) AddSeed(url string) {
	url = strings.TrimSpace(url)
	if url == "" {
		return
	}
	for _, s := range c.Seeds {
		if s == url {
			return
		}
	}
	c.Seeds = append(c.Seeds, url)
}

// RemoveSeed removes url from list.
func (c *Config) RemoveSeed(url string) bool {
	url = strings.TrimSpace(url)
	out := c.Seeds[:0]
	found := false
	for _, s := range c.Seeds {
		if s == url {
			found = true
			continue
		}
		out = append(out, s)
	}
	c.Seeds = out
	return found
}
