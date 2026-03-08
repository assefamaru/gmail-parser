package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/assefamaru/gmail-parser/internal/client/gmail"
	"github.com/assefamaru/gmail-parser/internal/parser/etf"
)

const (
	fromDateStr = "2026-01-01"
	toDateStr   = "2026-12-31"
)

func main() {
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
	fromDate, err := time.Parse("2006-01-02", fromDateStr)
	if err != nil {
		return fmt.Errorf("parse fromDate: %w", err)
	}
	toDate, err := time.Parse("2006-01-02", toDateStr)
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

	// Print data for now.
	out, err := json.Marshal(etfData)
	if err != nil {
		return fmt.Errorf("marshal parsed data: %w", err)
	}
	fmt.Println(string(out))

	return nil
}
