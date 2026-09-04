package http

import (
	"net/http"

	marketdataapi "github.com/jamersom/market-data-api"
	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/handlers"
)

const swaggerUI = `<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Market Data API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.32.14/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.32.14/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      SwaggerUIBundle({
        url: "/openapi.yaml",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>`

func RegisterRoutes(
	mux *http.ServeMux,
	quoteHandler *handlers.QuoteHandler,
	quoteHistoryHandler *handlers.QuoteHistoryHandler,
) {
	mux.HandleFunc("GET /quotes/{ticker}", quoteHandler.Get)
	mux.HandleFunc("GET /quotes/{ticker}/history", quoteHistoryHandler.Get)
	mux.HandleFunc("GET /openapi.yaml", serveOpenAPI)
	mux.HandleFunc("GET /docs", redirectToDocs)
	mux.HandleFunc("GET /docs/", serveSwaggerUI)
}

func serveOpenAPI(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(marketdataapi.OpenAPISpec())
}

func redirectToDocs(writer http.ResponseWriter, request *http.Request) {
	http.Redirect(writer, request, "/docs/", http.StatusPermanentRedirect)
}

func serveSwaggerUI(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/docs/" {
		http.NotFound(writer, request)
		return
	}

	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte(swaggerUI))
}
