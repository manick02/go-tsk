package email

import (
	"context"
	"fmt"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GmailClient handles Gmail IMAP operations
type GmailClient struct {
	client     *client.Client
	oauth2Conf *oauth2.Config
	token      *oauth2.Token
}

// NewGmailClient creates a new Gmail client
func NewGmailClient(clientID, clientSecret, token string) (*GmailClient, error) {
	oauth2Conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes: []string{
			"https://mail.google.com/",
		},
	}

	// Parse the token
	// In production, you'd want to implement proper token management
	tok := &oauth2.Token{
		AccessToken: token,
	}

	return &GmailClient{
		oauth2Conf: oauth2Conf,
		token:      tok,
	}, nil
}

// Connect establishes a connection to Gmail's IMAP server
func (g *GmailClient) Connect() error {
	// Connect to Gmail IMAP server
	c, err := client.DialTLS("imap.gmail.com:993", nil)
	if err != nil {
		return fmt.Errorf("failed to connect to IMAP server: %w", err)
	}

	g.client = c
	return nil
}

// Authenticate performs OAuth2 authentication
func (g *GmailClient) Authenticate() error {
	if g.client == nil {
		return fmt.Errorf("client not connected")
	}

	// Use OAuth2 token for authentication
	if err := g.client.Authenticate(g.token); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	return nil
}

// FetchNewEmails retrieves emails newer than the given time
func (g *GmailClient) FetchNewEmails(ctx context.Context, since time.Time) ([]*Email, error) {
	if g.client == nil {
		return nil, fmt.Errorf("client not connected")
	}

	// Select INBOX
	mbox, err := g.client.Select("INBOX", false)
	if err != nil {
		return nil, fmt.Errorf("failed to select inbox: %w", err)
	}

	// Search criteria
	criteria := imap.NewSearchCriteria()
	criteria.Since = since

	// Search for messages
	uids, err := g.client.Search(criteria)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(uids) == 0 {
		return nil, nil
	}

	// Create sequence set for fetching
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uids...)

	// Define items to fetch
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid}

	// Fetch messages
	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	go func() {
		done <- g.client.Fetch(seqSet, items, messages)
	}()

	// Process messages
	var emails []*Email
	for msg := range messages {
		email := &Email{
			UID:     msg.Uid,
			Subject: msg.Envelope.Subject,
			From:    formatAddresses(msg.Envelope.From),
			Date:    msg.Envelope.Date,
			Flags:   msg.Flags,
		}
		emails = append(emails, email)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	return emails, nil
}

// ApplyLabel adds a label to an email
func (g *GmailClient) ApplyLabel(uid uint32, label string) error {
	if g.client == nil {
		return fmt.Errorf("client not connected")
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uid)

	// In Gmail, labels are implemented as IMAP flags
	return g.client.Store(seqSet, imap.AddFlags, []interface{}{label}, nil)
}

// Close closes the IMAP connection
func (g *GmailClient) Close() error {
	if g.client == nil {
		return nil
	}
	return g.client.Logout()
}

// Email represents an email message
type Email struct {
	UID     uint32
	Subject string
	From    string
	Date    time.Time
	Flags   []string
}

// formatAddresses formats email addresses for display
func formatAddresses(addrs []*imap.Address) string {
	if len(addrs) == 0 {
		return ""
	}
	addr := addrs[0]
	if addr.PersonalName != "" {
		return fmt.Sprintf("%s <%s@%s>", addr.PersonalName, addr.MailboxName, addr.HostName)
	}
	return fmt.Sprintf("%s@%s", addr.MailboxName, addr.HostName)
}
