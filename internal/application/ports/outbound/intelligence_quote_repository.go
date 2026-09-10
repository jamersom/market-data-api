package outbound

import (
	"context"
	"time"

	"github.com/jamersom/market-data-api/internal/domain"
)

// IntelligenceHistory contains the complete published history up to the query
// date (not just the last 252 rows). This fixes the RSI seed at the first
// available quote. Corrections to that history must change DataVersion.
type IntelligenceHistory struct {
	Records     []domain.QuoteRecord
	DataVersion string
	// Sessions must contain every exchange session from the first quote through
	// the query date, inclusive. CalendarVerified may only be true when coverage
	// is complete and obtained from an authoritative calendar, not quote dates.
	Sessions         []time.Time
	CalendarVerified bool
	Calendar         domain.IntelligenceCalendarMetadata
}

// IntelligenceQuoteRepository loads history and calendar in one application
// call. Calendar coverage may be unavailable and must then remain unverified.
type IntelligenceQuoteRepository interface {
	FindIntelligenceHistory(ctx context.Context, ticker string, marketType int, asOf time.Time) (IntelligenceHistory, error)
}
