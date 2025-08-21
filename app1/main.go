package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"l02-01/telemetry"

	"github.com/joho/godotenv"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

const regextCepPattern = `^[0-9]{5}-[0-9]{3}$`

type application struct {
	logger     *log.Logger
	tracer     trace.Tracer
	httpClient *http.Client
}

type Request struct {
	Cep string `json:"cep"`
}

type Response struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

type loggingRoundTripper struct {
	logger *log.Logger
	next   http.RoundTripper
}

var cepRegex = regexp.MustCompile(regextCepPattern)

func main() {
	godotenv.Load()

	logger := log.New(os.Stderr, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)

	tracer, shutdown, err := telemetry.InitTelemetry("app1-service", "app1-tracer")
	if err != nil {
		log.Fatalf("ERROR: Failed to initialize telemetry: %v", err)
	}
	defer func() {
		logger.Println("INFO: Shutting down telemetry...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			logger.Printf("ERROR: Failed to shutdown telemetry gracefully: %v", err)
		}
		logger.Println("INFO: Telemetry shut down.")
	}()

	baseTransport := http.DefaultTransport
	loggingTransport := &loggingRoundTripper{
		logger: logger,
		next:   baseTransport,
	}

	otelTransport := otelhttp.NewTransport(loggingTransport)
	httpClient := &http.Client{Transport: otelTransport, Timeout: 10 * time.Second}

	app := &application{
		logger:     logger,
		tracer:     tracer,
		httpClient: httpClient,
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.Handle("/weather-by-cep", app.logRequest(http.HandlerFunc(app.handler)))

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// (Ctrl+C)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Inicia o servidor em uma goroutine para não bloquear a execução
	go func() {
		app.logger.Printf("Server listening on port %s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("ERROR: Could not start server: %v", err)
		}
	}()

	// Bloqueia a execução até que um sinal de interrupção seja recebido
	<-stop

	app.logger.Println("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("ERROR: Failed to shut down server gracefully: %v", err)
	}

	app.logger.Println("Server shut down.")
}

// Algo parecido como log de acesso
func (app *application) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}
		app.logger.Printf("Request: IP=%s Method=%s URL=%s User-Agent=\"%s\"", ip, r.Method, r.URL.RequestURI(), r.UserAgent())
		next.ServeHTTP(w, r)
	})
}

func (app *application) handler(w http.ResponseWriter, r *http.Request) {
	ctx, span := app.tracer.Start(r.Context(), "/weather-by-cep")
	defer span.End()

	// 1. Validação
	req := Request{}
	err := json.NewDecoder(r.Body).Decode(&req)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.Cep == "" {
		http.Error(w, "param 'cep' is required", http.StatusBadRequest)
		return
	}

	if !cepRegex.MatchString(req.Cep) {
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	// 2. Processamento
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	app2Endpoint := fmt.Sprintf("%s/get-weather-by-cep?cep=%s", os.Getenv("APP2_BASE_URL"), req.Cep)
	reqApp2, err := http.NewRequestWithContext(ctxWithTimeout, "GET", app2Endpoint, nil)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "fail create request to orchestrator service", http.StatusInternalServerError)
		return
	}

	response, err := app.httpClient.Do(reqApp2)
	if err != nil {
		span.RecordError(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		http.Error(w, "can not find zipcode", http.StatusNotFound)
		return
	}

	if response.StatusCode >= http.StatusBadRequest {
		http.Error(w, "error on find weather in orchestrator service", response.StatusCode)
		return
	}

	// 3. Resultado
	resp := Response{}
	err = json.NewDecoder(response.Body).Decode(&resp)
	if err != nil {
		span.RecordError(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// Para fins didáticos, é necessário uma camanda extra para capturar os dados de cabeçalhos
func (l *loggingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	l.logger.Println("------- Headers send -------")
	for name, values := range r.Header {
		for _, value := range values {
			l.logger.Printf("Header: %s: %s", name, value)
		}
	}
	l.logger.Println("-----------------------------")
	return l.next.RoundTrip(r)
}
