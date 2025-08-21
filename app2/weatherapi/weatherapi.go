package weatherapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrCityNotFound = fmt.Errorf("cidade não encontrada")
	ErrInternal     = fmt.Errorf("ocorreu um erro interno ao buscar o clima")
)

type WeatherApiClient interface {
	FindTemperatureByCity(ctx context.Context, city string) (*WeatherApiResponse, error)
}

type Logger interface {
	Printf(format string, v ...interface{})
}

type Client struct {
	apiKey     string
	httpClient *http.Client
	logger     Logger
	baseURL    string
	tracer     trace.Tracer
}

type CurrentWeather struct {
	TempC float64 `json:"temp_c"`
	TempF float64 `json:"temp_f"`
}

type WeatherApiResponse struct {
	Current CurrentWeather `json:"current"`
	Erro    bool           `json:"erro"`
}

// Necessário para sobrescrever dados da URL
type redactingTransport struct {
	base http.RoundTripper
}

// Não expor chave de API na URL
func (t *redactingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	span := trace.SpanFromContext(req.Context())

	if req.URL != nil {
		q := req.URL.Query()
		if q.Get("key") != "" {
			sanitizedURL := *req.URL
			newQuery := req.URL.Query()
			newQuery.Set("key", "***")
			sanitizedURL.RawQuery = newQuery.Encode()
			sanitizedURLString := sanitizedURL.String()
			span.SetAttributes(
				semconv.HTTPURLKey.String(sanitizedURLString),
				semconv.URLFullKey.String(sanitizedURLString),
			)
		}
	}

	return t.base.RoundTrip(req)
}

func NewClient(apiKey string, logger Logger, tracer trace.Tracer) *Client {
	otelTransport := otelhttp.NewTransport(&redactingTransport{base: http.DefaultTransport})
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Transport: otelTransport, Timeout: 5 * time.Second},
		baseURL:    "https://api.weatherapi.com/v1",
		logger:     logger,
		tracer:     tracer,
	}
}

func (c *Client) FindTemperatureByCity(ctx context.Context, city string) (*WeatherApiResponse, error) {
	ctx, span := c.tracer.Start(ctx, "FindTemperatureByCity")
	span.SetAttributes(attribute.String("city.name", city))
	defer span.End()

	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		span.RecordError(err)
		c.logger.Printf("Invalid base URL: %v", err)
		return nil, ErrInternal
	}

	baseURL.Path += "/current.json"
	params := url.Values{}
	params.Add("key", c.apiKey)
	params.Add("q", city)

	baseURL.RawQuery = params.Encode()
	fullURL := baseURL.String()
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		c.logger.Printf("Error requesting from WeatherAPI: %v", err)
		return nil, ErrInternal
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		span.AddEvent("WeatherAPI returned non-OK status")
		return nil, ErrCityNotFound
	}

	var data WeatherApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		span.RecordError(err)
		c.logger.Printf("Error decoding WeatherAPI response: %v", err)
		return nil, ErrInternal
	}

	return &data, nil
}
