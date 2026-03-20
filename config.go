package main

import (
	"os"

	scfgs "codeberg.org/lindenii/go-scfgs"
)

type Config struct {
	Discord struct {
		Token   string `scfgs:"token"`
		GuildID uint64 `scfgs:"guild_id"`
	} `scfgs:"discord"`

	Google struct {
		ClientID     string `scfgs:"client_id"`
		ClientSecret string `scfgs:"client_secret"`
		RedirectURL  string `scfgs:"redirect_url"`
	} `scfgs:"google"`

	HTTP struct {
		Net     string `scfgs:"net"`
		Addr    string `scfgs:"addr"`
		BaseURL string `scfgs:"base_url"`
	} `scfgs:"http"`

	Roles struct {
		Verified uint64 `scfgs:"verified"`
	} `scfgs:"roles"`

	CookieSecret       string `scfgs:"cookie_secret"`
	TokenExpirySeconds int    `scfgs:"token_expiry_seconds"`
}

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	if err := scfgs.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
