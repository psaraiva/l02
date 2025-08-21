package viacep

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrCepNotFound = fmt.Errorf("CEP não encontrado")
	ErrInternal    = fmt.Errorf("ocorreu um erro interno ao buscar o CEP")
)

type ViaCepClient interface {
	FindAddressByCep(ctx context.Context, cep string) (*ViaCepResponse, error)
}

type Logger interface {
	Printf(format string, v ...interface{})
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	logger     Logger
	tracer     trace.Tracer
}

type ViaCepResponse struct {
	Cep    string `json:"cep"`
	Street string `json:"logradouro"`
	City   string `json:"localidade"`
	State  string `json:"uf"`
	Erro   bool   `json:"erro"`
}

func NewClient(logger Logger, tracer trace.Tracer) *Client {
	return &Client{
		httpClient: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			Timeout:   5 * time.Second,
		},
		baseURL: "https://viacep.com.br",
		logger:  logger,
		tracer:  tracer,
	}
}

func (c *Client) FindAddressByCep(ctx context.Context, cep string) (*ViaCepResponse, error) {
	ctx, span := c.tracer.Start(ctx, "FindAddressByCep")
	span.SetAttributes(attribute.String("cep.value", cep))
	defer span.End()

	url := fmt.Sprintf("%s/ws/%s/json/", c.baseURL, cep)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		c.logger.Printf("Error requesting from ViaCEP API: %v", err)
		return nil, ErrInternal
	}
	defer resp.Body.Close()

	span.SetAttributes(semconv.HTTPStatusCodeKey.Int(resp.StatusCode))
	if resp.StatusCode != http.StatusOK {
		span.AddEvent("ViaCEP API returned non-OK status")
		return nil, ErrCepNotFound
	}

	var data ViaCepResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		span.RecordError(err)
		c.logger.Printf("Error decoding ViaCEP API response: %v", err)
		return nil, ErrInternal
	}

	if data.Erro {
		span.AddEvent("ViaCEP API response indicates CEP not found (erro=true)")
		return nil, ErrCepNotFound
	}

	return &data, nil
}
