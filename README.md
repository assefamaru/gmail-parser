# gmail-parser

This is a simple gmail parser built using the following instructions:

- https://developers.google.com/workspace/gmail/api/quickstart/go

The motivation behind this parser is to extract e-transfer information for personal book-keeping.

## Run

```bash
go run cmd/parser/main.go -from="<from-date>" -to="<to-date>"
```
