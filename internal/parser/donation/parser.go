package donation

import (
	"context"
	"encoding/base64"
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
	client     *gmail.Client
	query      string
	bodyRegexp *regexp.Regexp
	user       string
}

// Donation ...
type Donation struct {
	From    *User  // Parsed from snippet
	To      *User  // Static
	Date    string // message.Payload.Headers["X-Date"]
	Amount  string // Parsed from snippet
	RefID   string // Parsed from snippet
	Purpose string // Parsed from snippet

	Subject string // message.Payload.Headers["Subject"]
}

// User represents e-transfer sender/receiver information.
type User struct {
	Name  string
	Email string
}

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
	bodyRegexp, err := regexp.Compile(`Member Name:\s*(.*?)\s+Member ID:\s*#?(\d*)[^\n]*\s+Contribution Amount:\s*\$([\d.]+)\s+Contribution For:\s*(.*?)\s+Date:\s*(\d{4}-\d{2}-\d{2})\s+Payment Method:\s*(\w+)\s+Reference Number:\s*#?(\d+)`)
	if err != nil {
		return nil, fmt.Errorf("compile body regexp: %w", err)
	}
	return &Parser{
		client:     opts.Client,
		query:      buildQuery(opts.FromDate, opts.ToDate),
		bodyRegexp: bodyRegexp,
		user:       "me",
	}, nil
}

// Parse implements the extraction logic. It returns a
// list of extracted messages in structured format.
func (p *Parser) Parse(_ context.Context) ([]*Donation, error) {
	var parsedMessages []*Donation
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

func (p *Parser) parseMessages(messages []*apiv1.Message) ([]*Donation, error) {
	var parsedMessages []*Donation
	for _, message := range messages {
		msg, err := p.client.Client().Users.Messages.Get(p.user, message.Id).Do()
		if err != nil {
			return nil, fmt.Errorf("fetch full message for %q: %w", msg.Id, err)
		}
		parsedMessage, err := p.parseMessage(msg)
		if err != nil {
			return nil, fmt.Errorf("parse message %q: %w", msg.Id, err)
		}
		if parsedMessage != nil {
			parsedMessages = append(parsedMessages, parsedMessage)
		}
	}
	return parsedMessages, nil
}

func (p *Parser) parseMessage(message *apiv1.Message) (*Donation, error) {
	parsed := &Donation{
		From: &User{},
		To:   &User{},
	}
	for _, part := range message.Payload.Parts {
		if part.Body.Data != "" {
			// Best effort.
			re := regexp.MustCompile(`([^= \n]+)`)
			matches := re.FindStringSubmatch(part.Body.Data)
			decoded, _ := base64.URLEncoding.DecodeString(matches[1])
			decodedStr := string(decoded)
			parsed.From.Name = extract(`Member Name:\s*([^\n<]*)`, decodedStr)
			parsed.To.Email = "donation@kidanemihret.org"
			parsed.Purpose = extract(`Contribution For:\s*([^\n<]*)`, decodedStr)
			parsed.RefID = extract(`Reference Number:\s*#?(\d+)`, decodedStr)
			parsed.Amount = extract(`Contribution Amount:\s*\$?([\d.]*)`, decodedStr)
			parsed.Amount = strings.TrimSuffix(parsed.Amount, ".00")
			parsed.Amount = strings.ReplaceAll(parsed.Amount, ",", "")
			parsed.Amount = strings.ReplaceAll(parsed.Amount, "$", "")
			if parsed.From.Name != "" && parsed.Amount != "" {
				break
			}
		}
	}
	if parsed.Amount == "" {
		fmt.Println("Skipped", message.Id, parsed.From.Name)
		return nil, nil
	}
	for _, header := range message.Payload.Headers {
		if header.Name == "Subject" {
			parsed.Subject = header.Value
		}
		if header.Name == "Date" {
			parsedDate, err := time.Parse(time.RFC1123Z, header.Value)
			if err != nil {
				parsedDate, err = time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", header.Value)
				if err != nil {
					return nil, fmt.Errorf("parse Date %q: %w", header.Value, err)
				}
			}
			parsed.Date = parsedDate.Format(time.DateOnly)
		}
		if parsed.Subject != "" && parsed.Date != "" {
			break
		}
	}
	return parsed, nil
}

func buildQuery(fromDate, toDate time.Time) string {
	from := fromDate.Format(time.DateOnly)
	to := toDate.Format(time.DateOnly)
	return fmt.Sprintf("donation@kidanemihret.org after:%s before:%s", from, to)
}

func extract(pattern, text string) string {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(text)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}
