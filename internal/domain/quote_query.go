package domain

import "time"

const (
	DefaultMarketType = 10
	DefaultLimit      = 100
	MaxLimit          = 1000
)

type SortOrder string

const (
	SortAscending  SortOrder = "asc"
	SortDescending SortOrder = "desc"
)

type QuoteRecord struct {
	Quote              Quote
	PreviousCloseCents *int64
	PublishedAt        time.Time
	SourceURL          string
	ParserVersion      string
	LayoutVersion      string
}

type QuotePage struct {
	Records []QuoteRecord
	Total   int64
}
