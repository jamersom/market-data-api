package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeOpenAPI(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	response := httptest.NewRecorder()

	serveOpenAPI(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/yaml; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", contentType)
	}
	if !strings.HasPrefix(response.Body.String(), "openapi: 3.1.0") {
		t.Fatal("response does not contain the embedded OpenAPI document")
	}
}

func TestServeSwaggerUI(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/docs/", nil)
	response := httptest.NewRecorder()

	serveSwaggerUI(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if !strings.Contains(response.Body.String(), `url: "/openapi.yaml"`) {
		t.Fatal("Swagger UI does not reference the embedded OpenAPI document")
	}
}

func TestRedirectToDocs(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/docs", nil)
	response := httptest.NewRecorder()

	redirectToDocs(response, request)

	if response.Code != http.StatusPermanentRedirect {
		t.Fatalf("expected status %d, got %d", http.StatusPermanentRedirect, response.Code)
	}
	if location := response.Header().Get("Location"); location != "/docs/" {
		t.Fatalf("unexpected redirect location: %s", location)
	}
}
