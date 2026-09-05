package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/response"
	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

const comparisonBodyLimit = 64 * 1024
const comparisonTimeout = 5 * time.Second

type ComparisonsHandler struct{ compare inbound.CompareQuotesUseCase }

func NewComparisonsHandler(compare inbound.CompareQuotesUseCase) *ComparisonsHandler {
	return &ComparisonsHandler{compare: compare}
}

// Os dois métodos convertem os parâmetros para o mesmo contrato de entrada.
type comparisonRequest struct {
	Tickers       []string                  `json:"tickers"`
	From          string                    `json:"from"`
	To            string                    `json:"to"`
	MarketType    *int                      `json:"marketType"`
	Metrics       []domain.ComparisonMetric `json:"metrics"`
	Benchmark     string                    `json:"benchmark"`
	IncludeSeries bool                      `json:"includeSeries"`
}

func (h *ComparisonsHandler) Get(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	for name, values := range query {
		switch name {
		case "tickers", "from", "to", "marketType", "metrics", "benchmark", "includeSeries":
		default:
			writeError(w, domain.ValidationError{Field: name, Message: "unknown query parameter"})
			return
		}
		if len(values) != 1 {
			writeError(w, domain.ValidationError{Field: name, Message: "query parameter must not be repeated"})
			return
		}
	}
	request := comparisonRequest{Tickers: strings.Split(query.Get("tickers"), ","), From: query.Get("from"), To: query.Get("to"), Benchmark: query.Get("benchmark")}
	if query.Has("metrics") {
		for _, metric := range strings.Split(query.Get("metrics"), ",") {
			request.Metrics = append(request.Metrics, domain.ComparisonMetric(metric))
		}
	}
	if query.Has("marketType") {
		market, err := strconv.Atoi(query.Get("marketType"))
		if err != nil {
			writeError(w, domain.ValidationError{Field: "marketType", Message: "marketType must be an integer", Err: domain.ErrInvalidMarketType})
			return
		}
		request.MarketType = &market
	}
	if query.Has("includeSeries") {
		switch query.Get("includeSeries") {
		case "true":
			request.IncludeSeries = true
		case "false":
		default:
			writeError(w, domain.ValidationError{Field: "includeSeries", Message: "includeSeries must be true or false"})
			return
		}
	}
	h.execute(w, r, request)
}

func (h *ComparisonsHandler) Post(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeComparisonRequestError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, comparisonBodyLimit)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request comparisonRequest
	err = decoder.Decode(&request)
	if err == nil {
		var extra any
		if next := decoder.Decode(&extra); next != io.EOF {
			if next == nil {
				err = errors.New("multiple JSON values")
			} else {
				err = next
			}
		}
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeComparisonRequestError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds 64 KiB")
			return
		}
		writeComparisonRequestError(w, http.StatusBadRequest, "invalid_request", "body must be a single JSON object with supported fields and types")
		return
	}
	h.execute(w, r, request)
}

func (h *ComparisonsHandler) execute(w http.ResponseWriter, r *http.Request, request comparisonRequest) {
	input, err := request.input()
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), comparisonTimeout)
	defer cancel()
	output, err := h.compare.Execute(ctx, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ComparisonResponse(output))
}

func (request comparisonRequest) input() (inbound.CompareQuotesInput, error) {
	input := inbound.CompareQuotesInput{Tickers: request.Tickers, Metrics: request.Metrics, Benchmark: request.Benchmark, IncludeSeries: request.IncludeSeries}
	for _, field := range []struct {
		name, value string
		target      *time.Time
	}{{"from", request.From, &input.From}, {"to", request.To, &input.To}} {
		date, err := time.Parse(time.DateOnly, field.value)
		if err != nil {
			return input, domain.ValidationError{Field: field.name, Value: field.value, Message: field.name + " is required and must use YYYY-MM-DD", Err: domain.ErrInvalidDateRange}
		}
		*field.target = date
	}
	if request.MarketType != nil {
		if *request.MarketType <= 0 {
			return input, domain.ValidationError{Field: "marketType", Message: "marketType must be positive", Err: domain.ErrInvalidMarketType}
		}
		input.MarketType = *request.MarketType
	}
	return input, nil
}

func writeComparisonRequestError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, response.Error{Error: response.ErrorDetail{Code: code, Message: message, Retryable: false}})
}
