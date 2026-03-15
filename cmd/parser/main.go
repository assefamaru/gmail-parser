package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/assefamaru/gmail-parser/internal/client/gmail"
	"github.com/assefamaru/gmail-parser/internal/parser/etf"
)

var (
	fromDateStr = flag.String("from", "2026-01-01", "Date filter for earliest message to fetch")
	toDateStr   = flag.String("to", "2026-12-31", "Date filter for latest message to fetch")
)

func main() {
	flag.Parse()
	ctx := context.Background()
	if err := runParser(ctx); err != nil {
		log.Fatal(err)
	}
}

func runParser(ctx context.Context) error {
	gmailOpts := &gmail.Options{
		CredFilePath: "credentials.json",
	}
	client, err := gmail.NewClient(ctx, gmailOpts)
	if err != nil {
		return fmt.Errorf("create gmail client: %w", err)
	}
	fromDate, err := time.Parse("2006-01-02", *fromDateStr)
	if err != nil {
		return fmt.Errorf("parse fromDate: %w", err)
	}
	toDate, err := time.Parse("2006-01-02", *toDateStr)
	if err != nil {
		return fmt.Errorf("parse toDate: %w", err)
	}
	parserOpts := &etf.ParserOptions{
		Client:   client,
		FromDate: fromDate,
		ToDate:   toDate,
	}
	parser, err := etf.NewParser(parserOpts)
	if err != nil {
		return fmt.Errorf("create parser: %w", err)
	}
	etfData, err := parser.Parse(ctx)
	if err != nil {
		return fmt.Errorf("parse data: %w", err)
	}

	// Write data to CSV and JSON for now.
	if err := writeData(etfData); err != nil {
		return fmt.Errorf("write data: %w", err)
	}

	return nil
}

func writeData(etfData []*etf.ETransfer) error {
	var sent, received, unknown []*etf.ETransfer
	for _, entry := range etfData {
		switch entry.TransferType {
		case etf.Sent:
			sent = append(sent, entry)
		case etf.Received:
			received = append(received, entry)
		case etf.Unknown:
			unknown = append(unknown, entry)
		}
	}
	if err := writeCSV(sent, "sent.csv"); err != nil {
		return fmt.Errorf("write sent csv: %w", err)
	}
	if err := writeCSV(received, "received.csv"); err != nil {
		return fmt.Errorf("write received csv: %w", err)
	}
	if err := writeCSV(unknown, "unknown.csv"); err != nil {
		return fmt.Errorf("write unknown csv: %w", err)
	}
	out, err := json.Marshal(etfData)
	if err != nil {
		return fmt.Errorf("marshal parsed data: %w", err)
	}
	if err := os.WriteFile("data.json", out, 0600); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Sent: %v\nReceived: %v\nUnknown: %v\n", len(sent), len(received), len(unknown))
	return nil
}

func writeCSV(data []*etf.ETransfer, dest string) error {
	if len(data) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("ReferenceNumber,Date,Amount,SenderName,SenderEmail,Message,ReceiverName,ReceiverEmail\n")
	for _, d := range data {
		fmt.Fprintf(&sb, "%s,%s,%s,%s,%s,%s,%s,%s\n", d.RefID, d.Date, d.Amount, d.From.Name, d.From.Email, d.Message, d.To.Name, d.To.Email)
	}
	return os.WriteFile(dest, []byte(sb.String()), 0600)
}
