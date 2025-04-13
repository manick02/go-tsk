package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/mshan/go-tsk/internal/config"
	"github.com/mshan/go-tsk/internal/email"
)

// AccountState tracks the state for each email account
type AccountState struct {
	lastSync  time.Time
	isActive  bool
	stopChan  chan struct{}
	client    *email.GmailClient
}

// EmailPoller handles the email polling logic
type EmailPoller struct {
	config       *config.Config
	accountState map[string]*AccountState // key is account ID
	mu          sync.RWMutex
}

// NewEmailPoller creates a new email poller
func NewEmailPoller(cfg *config.Config) *EmailPoller {
	accountState := make(map[string]*AccountState)
	for _, account := range cfg.EmailAccounts {
		accountState[account.ID] = &AccountState{
			stopChan: make(chan struct{}),
		}
	}
	
	return &EmailPoller{
		config:       cfg,
		accountState: accountState,
	}
}

// Start begins the polling process for all enabled accounts
func (p *EmailPoller) Start(ctx context.Context) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(p.config.EmailAccounts))

	// Start polling for each enabled account
	for _, account := range p.config.EmailAccounts {
		if !account.Enabled {
			log.Printf("Account %s (%s) is disabled, skipping", account.ID, account.Name)
			continue
		}

		wg.Add(1)
		go func(acc config.EmailAccount) {
			defer wg.Done()
			if err := p.pollAccount(ctx, acc); err != nil {
				errChan <- fmt.Errorf("error polling account %s: %w", acc.ID, err)
			}
		}(account)
	}

	// Wait for all polling goroutines to finish
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Return first error if any
	for err := range errChan {
		return err
	}

	return nil
}

// Stop stops all polling processes and closes connections
func (p *EmailPoller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, state := range p.accountState {
		if state.isActive {
			if state.client != nil {
				if err := state.client.Close(); err != nil {
					log.Printf("Error closing email client: %v", err)
				}
			}
			close(state.stopChan)
			state.isActive = false
		}
	}
}

// pollAccount handles polling for a single account
func (p *EmailPoller) pollAccount(ctx context.Context, account config.EmailAccount) error {
	state := p.accountState[account.ID]
	
	p.mu.Lock()
	if state.isActive {
		p.mu.Unlock()
		return fmt.Errorf("polling already active for account %s", account.ID)
	}
	state.isActive = true
	p.mu.Unlock()

	ticker := time.NewTicker(p.config.Poll.Interval)
	defer ticker.Stop()

	// Do initial poll
	if err := p.poll(ctx, account); err != nil {
		log.Printf("Initial poll failed for account %s: %v", account.ID, err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-state.stopChan:
			return nil
		case <-ticker.C:
			if err := p.poll(ctx, account); err != nil {
				log.Printf("Poll failed for account %s: %v", account.ID, err)
				continue
			}
		}
	}
}

// poll performs a single polling operation for one account
func (p *EmailPoller) poll(ctx context.Context, account config.EmailAccount) error {
	p.mu.Lock()
	state := p.accountState[account.ID]
	lastSync := state.lastSync
	p.mu.Unlock()

	// Initialize client if needed
	if state.client == nil {
		client, err := email.NewGmailClient(
			account.ClientID,
			account.ClientSecret,
			account.Token,
		)
		if err != nil {
			return fmt.Errorf("failed to create Gmail client: %w", err)
		}

		if err := client.Connect(); err != nil {
			return fmt.Errorf("failed to connect to Gmail: %w", err)
		}

		if err := client.Authenticate(); err != nil {
			return fmt.Errorf("failed to authenticate with Gmail: %w", err)
		}

		state.client = client
	}

	// Fetch new emails
	emails, err := state.client.FetchNewEmails(ctx, lastSync)
	if err != nil {
		return fmt.Errorf("failed to fetch emails: %w", err)
	}

	// Process emails according to rules
	for _, email := range emails {
		for _, rule := range p.config.Poll.Rules {
			if containsIgnoreCase(email.Subject, rule.SubjectContains) {
				if err := state.client.ApplyLabel(email.UID, rule.Label); err != nil {
					log.Printf("Failed to apply label to email %d: %v", email.UID, err)
					continue
				}
				log.Printf("Applied label '%s' to email with subject: %s", rule.Label, email.Subject)
			}
		}
	}

	p.mu.Lock()
	state.lastSync = time.Now()
	p.mu.Unlock()
	
	return nil
}

// containsIgnoreCase checks if substr is in s, case-insensitive
func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
}
