package weatherapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"
)

type mockLogger struct{}

func (m *mockLogger) Printf(format string, v ...interface{}) {}

func TestFindTemperatureByCity_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/current.json" {
			t.Errorf("expected path '/current.json', but got '%s'", r.URL.Path)
		}

		if r.URL.Query().Get("q") != "São Paulo" {
			t.Errorf("expected query param 'q' to be 'São Paulo', but got '%s'", r.URL.Query().Get("q"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"current":{"temp_c": 25.5, "temp_f": 77.9}}`))
	}))
	defer server.Close()

	client := NewClient("fake-api-key", &mockLogger{}, noop.NewTracerProvider().Tracer("test"))
	client.baseURL = server.URL
	weather, err := client.FindTemperatureByCity(context.Background(), "São Paulo")

	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}

	if weather.Current.TempC != 25.5 {
		t.Errorf("expected TempC 25.5, but got '%f'", weather.Current.TempC)
	}

	if weather.Current.TempF != 77.9 {
		t.Errorf("expected TempF 77.9, but got '%f'", weather.Current.TempF)
	}
}

func TestFindTemperatureByCity_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient("fake-api-key", &mockLogger{}, noop.NewTracerProvider().Tracer("test"))
	client.baseURL = server.URL

	_, err := client.FindTemperatureByCity(context.Background(), "CidadeInexistente")
	if err != ErrCityNotFound {
		t.Errorf("expected error '%v', but got '%v'", ErrCityNotFound, err)
	}
}
