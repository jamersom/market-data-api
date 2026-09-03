package handlers

import (
	"net/http"
	"strconv"

	"github.com/jamersom/market-data-api/internal/domain"
)

func optionalIntParameter(r *http.Request, name string, defaultValue int) (int, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, domain.ValidationError{Field: name, Value: value, Message: name + " must be an integer", Err: validationSentinel(name)}
	}
	return parsed, nil
}

func validationSentinel(name string) error {
	switch name {
	case "marketType":
		return domain.ErrInvalidMarketType
	case "offset":
		return domain.ErrInvalidOffset
	default:
		return domain.ErrInvalidLimit
	}
}
