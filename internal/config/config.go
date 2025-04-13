package config

import "time"

// Config holds the application configuration
type Config struct {
	EmailAccounts []EmailAccount
	Poll          PollConfig
}

// EmailAccount represents a single email account configuration
type EmailAccount struct {
	ID           string // Unique identifier for the account
	Name         string // Friendly name for the account
	Provider     string // "gmail" for now
	ClientID     string // OAuth2 client ID
	ClientSecret string // OAuth2 client secret
	Token        string // OAuth2 access token
	Enabled      bool   // Whether this account should be polled
}

// PollConfig holds polling-related configuration
type PollConfig struct {
	Interval time.Duration
	Rules    []Rule
}

// Rule represents an email processing rule
type Rule struct {
	SubjectContains string
	Action          string
	Label           string
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		EmailAccounts: []EmailAccount{
			{
				ID:       "primary",
				Name:     "Primary Gmail",
				Provider: "gmail",
				Enabled:  true,
				// OAuth2 credentials need to be set
				ClientID:     "",
				ClientSecret: "",
				Token:        "",
			},
		},
		Poll: PollConfig{
			Interval: 5 * time.Minute,
			Rules: []Rule{
				{
					SubjectContains: "job opportunity",
					Action:          "label",
					Label:           "imp",
				},
			},
		},
	}
}
