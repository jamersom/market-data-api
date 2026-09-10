package domain

import "time"

type CalendarCoverage struct {
	Year               int
	From, To           time.Time
	IntegrityValidated bool
	Version            string
}

// Observed coverage is distinct from independent official verification.
type IntelligenceCalendarMetadata struct {
	Source, Version, Policy string
	OfficialVerified        bool
	Coverage                []CalendarCoverage
}
