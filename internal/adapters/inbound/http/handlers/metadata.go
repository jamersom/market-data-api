package handlers

import (
	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/response"
	"github.com/jamersom/market-data-api/internal/domain"
)

func metadataFromRecord(record domain.QuoteRecord) response.Metadata {
	meta := response.Metadata{Source: "B3 COTAHIST", PriceAdjustment: "unadjusted"}
	enrichMetadata(&meta, record)
	return meta
}

func enrichMetadata(meta *response.Metadata, record domain.QuoteRecord) {
	meta.SourceURL, meta.ParserVersion, meta.LayoutVersion = record.SourceURL, record.ParserVersion, record.LayoutVersion
	if !record.PublishedAt.IsZero() {
		meta.PublishedAt = record.PublishedAt.Format("2006-01-02T15:04:05Z07:00")
	}
}
