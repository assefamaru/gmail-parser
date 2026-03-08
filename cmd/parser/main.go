package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assefamaru/gmail-parser/internal/client/gmail"
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
	gmailClient, err := gmail.NewClient(ctx, gmailOpts)
	if err != nil {
		return fmt.Errorf("create gmail client: %w", err)
	}

	// Temp.
	user := "me"
	r, err := gmailClient.Client().Users.Labels.List(user).Do()
	if err != nil {
		return fmt.Errorf("unable to retrieve labels: %w", err)
	}
	if len(r.Labels) == 0 {
		return fmt.Errorf("no labels found")
	}
	for _, label := range r.Labels {
		fmt.Println("-", label.Name)
	}
	return nil
}
