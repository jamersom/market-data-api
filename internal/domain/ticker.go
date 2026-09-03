package domain

import (
	"regexp"
	"strings"
)

var tickerPattern = regexp.MustCompile(`^[A-Z0-9]{4,12}$`)

func NormalizeTicker(value string) (string, error) {
	ticker := strings.ToUpper(strings.TrimSpace(value))

	if !tickerPattern.MatchString(ticker) {
		return "", ValidationError{
			Field:   "ticker",
			Value:   value,
			Message: "ticker must contain between 4 and 12 letters or numbers",
			Err:     ErrInvalidTicker,
		}
	}

	return ticker, nil
}
