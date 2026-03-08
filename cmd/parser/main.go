package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/assefamaru/gmail-parser/internal/client/gmail"
	"github.com/assefamaru/gmail-parser/internal/parser/etf"
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
	parserOpts := &etf.ParserOptions{
		Client: client,
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
