package viacep

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"
)

type mockLogger struct{}

func (m *mockLogger) Printf(format string, v ...interface{}) {}

func TestFindAddressByCep_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws/01001-000/json/" {
			t.Errorf("expected path '/ws/01001-000/json/', but got '%s'", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"cep": "01001-000", "localidade": "São Paulo"}`))
	}))
	defer server.Close()

	client := NewClient(&mockLogger{}, noop.NewTracerProvider().Tracer("test"))
	client.baseURL = server.URL

	address, err := client.FindAddressByCep(context.Background(), "01001-000")

	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}

	if address == nil {
		t.Fatal("expected an address, but got nil")
	}

	if address.City != "São Paulo" {
		t.Errorf("expected city 'São Paulo', but got '%s'", address.City)
	}
}

func TestFindAddressByCep_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"erro": true}`))
	}))
	defer server.Close()

	client := NewClient(&mockLogger{}, noop.NewTracerProvider().Tracer("test"))
	client.baseURL = server.URL

	_, err := client.FindAddressByCep(context.Background(), "99999-999")

	if err != ErrCepNotFound {
		t.Errorf("expected error '%v', but got '%v'", ErrCepNotFound, err)
	}
}
