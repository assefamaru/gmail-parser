// package etf implements a parser for canadian e-transfer email data.
package etf

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/assefamaru/gmail-parser/internal/client/gmail"
	apiv1 "google.golang.org/api/gmail/v1"
)

var (
	ErrInvalidOptions = errors.New("invalid options")
)

// ParserOptions is used to configure a new parser.
type ParserOptions struct {
	Client   *gmail.Client
	FromDate time.Time
	ToDate   time.Time
}

// Parser is used to instrument parsing e-transfer information
// from gmail messages.
type Parser struct {
	client       *gmail.Client
	query        string
	dollarRegexp *regexp.Regexp
	userRegexp   *regexp.Regexp
	user         string
}

// ETransfer represents the structure of a single e-transfer payload.
type ETransfer struct {
	From   *User  // message.Payload.Headers["Reply-To"]
	To     *User  // message.Payload.Headers["To"]
	Date   string // message.Payload.Headers["X-Date"]
	Amount string // message.Payload.Headers["Subject"]
	RefID  string // message.Payload.Headers["X-PaymentKey"]

	TransferType TransferType
	Subject      string // message.Payload.Headers["Subject"]
}

// User represents e-transfer sender/receiver information.
type User struct {
	Name  string
	Email string
}

type TransferType string

const (
	Sent     TransferType = "Sent"
	Received TransferType = "Received"
	Unknown  TransferType = "Unknown"
)

// NewParser creates a new Parser.
func NewParser(opts *ParserOptions) (*Parser, error) {
	if opts == nil {
		return nil, fmt.Errorf("%w: opts is nil", ErrInvalidOptions)
	}
	if opts.Client == nil {
		return nil, fmt.Errorf("%w: nil opts.Client", ErrInvalidOptions)
	}
	if opts.ToDate.Before(opts.FromDate) {
		return nil, fmt.Errorf("%w: opts.ToDate is earlier than opts.FromDate", ErrInvalidOptions)
	}
	dollarRegexp, err := regexp.Compile(`\$[0-9,.]+`)
	if err != nil {
		return nil, fmt.Errorf("compile dollar amount regexp: %w", err)
	}
	userRegexp, err := regexp.Compile(`^([^<]+)\s*<([^>]+)>$`)
	if err != nil {
		return nil, fmt.Errorf("compile user parse regexp: %w", err)
	}
	return &Parser{
		client:       opts.Client,
		query:        buildQuery(opts.FromDate, opts.ToDate),
		dollarRegexp: dollarRegexp,
		userRegexp:   userRegexp,
		user:         "me",
	}, nil
}

// Parse implements the extraction logic. It returns a
// list of extracted messages in structured format.
func (p *Parser) Parse(_ context.Context) ([]*ETransfer, error) {
	var parsedMessages []*ETransfer
	var nextPageToken string
	for range 1000 {
		query := p.query
		if nextPageToken != "" {
			query = nextPageToken
		}
		resp, err := p.client.Client().Users.Messages.List(p.user).MaxResults(500).Q(query).Do()
		if err != nil {
			return nil, fmt.Errorf("list messages: %w", err)
		}
		messages, err := p.parseMessages(resp.Messages)
		if err != nil {
			return nil, fmt.Errorf("build entries: %w", err)
		}
		parsedMessages = append(parsedMessages, messages...)
		nextPageToken = resp.NextPageToken
		if nextPageToken == "" {
			break
		}
	}
	return parsedMessages, nil
}

func (p *Parser) parseMessages(messages []*apiv1.Message) ([]*ETransfer, error) {
	var parsedMessages []*ETransfer
	for _, message := range messages {
		msg, err := p.client.Client().Users.Messages.Get(p.user, message.Id).Do()
		if err != nil {
			return nil, fmt.Errorf("fetch full message for %q: %w", msg.Id, err)
		}
		parsedMessage, err := p.parseMessage(msg)
		if err != nil {
			return nil, fmt.Errorf("parse message %q: %w", msg.Id, err)
		}
		parsedMessages = append(parsedMessages, parsedMessage)
	}
	return parsedMessages, nil
}

func (p *Parser) parseMessage(message *apiv1.Message) (*ETransfer, error) {
	parsed := &ETransfer{}
	for _, header := range message.Payload.Headers {
		if header.Name == "Reply-To" {
			parsed.From = p.parseUser(header.Value)
		}
		if header.Name == "To" {
			parsed.To = p.parseUser(header.Value)
		}
		if header.Name == "X-Date" {
			parsedDate, err := time.Parse("02-01-2006 15:04", header.Value)
			if err != nil {
				return nil, fmt.Errorf("parse X-Date %q: %w", header.Value, err)
			}
			parsed.Date = parsedDate.Format(time.DateOnly)
		}
		if header.Name == "X-PaymentKey" {
			parsed.RefID = header.Value
		}
		if header.Name == "Subject" {
			parsed.Subject = header.Value
			parsed.Amount = p.dollarRegexp.FindString(header.Value)
			parsed.TransferType = transferType(header.Value)
		}
	}
	return parsed, nil
}

func (p *Parser) parseUser(s string) *User {
	user := &User{}
	matches := p.userRegexp.FindStringSubmatch(strings.TrimSpace(s))
	if len(matches) == 3 {
		user.Name = strings.TrimSpace(matches[1])
		user.Email = matches[2]
	}
	return user
}

func buildQuery(fromDate, toDate time.Time) string {
	from := fromDate.Format(time.DateOnly)
	to := toDate.Format(time.DateOnly)
	return fmt.Sprintf("@payments.interac.ca after:%s before:%s", from, to)
}

func transferType(subject string) TransferType {
	if strings.Contains(subject, "You've received") {
		return Received
	}
	if strings.Contains(subject, "has been successfully deposited.") {
		return Sent
	}
	return Unknown
}
