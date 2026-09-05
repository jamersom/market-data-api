package postgres

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/handlers"
	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/application/services"
	"github.com/jamersom/market-data-api/internal/domain"
	"github.com/jamersom/market-data-api/internal/infra/database"
	"github.com/joho/godotenv"
)

func TestComparisonQuoteRepositoryIntegration(t *testing.T) {
	if os.Getenv("DATABASE_INTEGRATION_TEST") != "1" {
		t.Skip("defina DATABASE_INTEGRATION_TEST=1 para executar")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && os.Getenv("DATABASE_URL") == "" {
		t.Fatal("configure DATABASE_URL ou o arquivo .env")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.NewPostgresPool(ctx)
	if err != nil {
		t.Fatalf("conectar: %v", err)
	}
	defer pool.Close()
	repository := NewQuoteRepository(pool)
	latest, err := repository.FindLatestByTicker(ctx, "PETR4", 10)
	if err != nil {
		t.Fatal(err)
	}
	to := latest.Quote.TradingDate
	from := to.AddDate(-1, 0, 0)
	tickers := []string{"PETR4", "VALE3", "ZZZZZZZZZZZZ"}
	records, err := repository.FindByTickersAndPeriod(ctx, tickers, from, to, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records[tickers[2]]) != 0 {
		t.Fatal("ticker inexistente retornou dados")
	}
	for _, ticker := range tickers[:2] {
		page, err := repository.FindByTickerAndPeriod(ctx, ticker, from, to, 10, 1000, 0, domain.SortAscending)
		if err != nil {
			t.Fatal(err)
		}
		if len(records[ticker]) < 2 || int64(len(records[ticker])) != page.Total {
			t.Fatalf("histórico incompleto para %s: lote=%d, total=%d", ticker, len(records[ticker]), page.Total)
		}
		for i, record := range records[ticker] {
			quote := record.Quote
			if quote.Ticker != ticker || quote.MarketType != 10 || quote.TradingDate.Before(from) || quote.TradingDate.After(to) || quote.ClosePriceCents != page.Records[i].Quote.ClosePriceCents || record.PublishedAt.IsZero() {
				t.Fatalf("registro inválido: %s, índice %d", ticker, i)
			}
			if i > 0 && !quote.TradingDate.After(records[ticker][i-1].Quote.TradingDate) {
				t.Fatalf("datas duplicadas ou fora de ordem: %s", ticker)
			}
		}
	}
	// Confere a inclusão da data final e o filtro de mercado com dados publicados reais.
	single, err := repository.FindByTickersAndPeriod(ctx, []string{"PETR4"}, to, to, 10)
	if err != nil || len(single["PETR4"]) != 1 {
		t.Fatalf("limites inclusivos: %v, %d registros", err, len(single["PETR4"]))
	}
	empty, err := repository.FindByTickersAndPeriod(ctx, tickers, from, to, 99999)
	if err != nil || len(empty["PETR4"]) != 0 {
		t.Fatalf("filtro de mercado: %v", err)
	}
	output, err := services.NewCompareQuotesService(repository, nil).Execute(ctx, inbound.CompareQuotesInput{Tickers: tickers, From: from, To: to})
	if err != nil || len(output.Assets) != 3 || output.Assets[2].Status != domain.ComparisonAssetInsufficientData {
		t.Fatalf("integração com aplicação: %v", err)
	}
	// Exercita os dois transportes com o serviço e o repositório reais, sem iniciar outro servidor.
	handler := handlers.NewComparisonsHandler(services.NewCompareQuotesService(repository, nil))
	get := httptest.NewRecorder()
	handler.Get(get, httptest.NewRequest("GET", "/comparisons?tickers=PETR4,VALE3&from="+from.Format(time.DateOnly)+"&to="+to.Format(time.DateOnly), nil))
	post := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/comparisons", strings.NewReader(`{"tickers":["PETR4","VALE3"],"from":"`+from.Format(time.DateOnly)+`","to":"`+to.Format(time.DateOnly)+`"}`))
	request.Header.Set("Content-Type", "application/json")
	handler.Post(post, request)
	if get.Code != 200 || post.Code != 200 || get.Body.String() != post.Body.String() {
		t.Fatalf("integração HTTP: GET=%d, POST=%d; respostas equivalentes=%v", get.Code, post.Code, get.Body.String() == post.Body.String())
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	_, err = repository.FindByTickersAndPeriod(canceled, tickers, from, to, 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelamento: %v", err)
	}
}
