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
	"github.com/assefamaru/gmail-parser/internal/parser/donation"
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
	etfParser, err := etf.NewParser(parserOpts)
	if err != nil {
		return fmt.Errorf("create etf parser: %w", err)
	}
	etfData, err := etfParser.Parse(ctx)
	if err != nil {
		return fmt.Errorf("parse etf data: %w", err)
	}
	donateOpts := &donation.ParserOptions{
		Client:   client,
		FromDate: fromDate,
		ToDate:   toDate,
	}
	donateParser, err := donation.NewParser(donateOpts)
	if err != nil {
		return fmt.Errorf("create donation parser: %w", err)
	}
	donateData, err := donateParser.Parse(ctx)
	if err != nil {
		return fmt.Errorf("parse donation data: %w", err)
	}

	// Write data to CSV and JSON for now.
	if err := writeData(etfData, donateData); err != nil {
		return fmt.Errorf("write data: %w", err)
	}

	return nil
}

func writeData(etfData []*etf.ETransfer, donateData []*donation.Donation) error {
	var sent, receivedETF, unknownETF []*etf.ETransfer
	for _, entry := range etfData {
		switch entry.TransferType {
		case etf.Sent:
			sent = append(sent, entry)
		case etf.Received:
			receivedETF = append(receivedETF, entry)
		case etf.Unknown:
			unknownETF = append(unknownETF, entry)
		}
	}
	if err := writeCSV(sent, nil, "sent.csv"); err != nil {
		return fmt.Errorf("write sent csv: %w", err)
	}
	if err := writeCSV(receivedETF, donateData, "received.csv"); err != nil {
		return fmt.Errorf("write received csv: %w", err)
	}
	if err := writeCSV(unknownETF, nil, "unknown.csv"); err != nil {
		return fmt.Errorf("write unknown csv: %w", err)
	}

	var all []any
	all = append(all, etfData)
	all = append(all, donateData)
	allData, _ := json.Marshal(all)
	os.WriteFile("data.json", allData, 0600)

	fmt.Fprintf(os.Stderr, "Sent: %v\nReceived: %v\nUnknown: %v\n", len(sent), len(receivedETF)+len(donateData), len(unknownETF))
	return nil
}

func writeCSV(etfData []*etf.ETransfer, donateData []*donation.Donation, dest string) error {
	if len(etfData) == 0 && len(donateData) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("ReferenceNumber,Date,Amount,SenderName,SenderEmail,Type,ContributionFor,ReceiverEmail,ReceiverName\n")
	for _, d := range etfData {
		fmt.Fprintf(&sb, "%s,%s,%s,%s,%s,E-Transfer,%s,%s,%s\n", d.RefID, d.Date, d.Amount, d.From.Name, d.From.Email, d.Message, d.To.Email, d.To.Name)
	}
	for _, d := range donateData {
		fmt.Fprintf(&sb, "%s,%s,%s,%s,%s,Online Donation,%s,%s,%s\n", d.RefID, d.Date, d.Amount, d.From.Name, d.From.Email, d.Purpose, d.To.Email, d.To.Name)
	}
	return os.WriteFile(dest, []byte(sb.String()), 0600)
}
