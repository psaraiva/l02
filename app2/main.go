package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"l02-02/telemetry"
	"l02-02/viacep"
	"l02-02/weatherapi"

	"github.com/joho/godotenv"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

const (
	regextCepPattern     = `^[0-9]{5}-[0-9]{3}$`
	InternalErrorMessage = "ocorreu um erro ao processar sua requisição"
)

type application struct {
	viaCepClient     viacep.ViaCepClient
	weatherApiClient weatherapi.WeatherApiClient
	logger           *log.Logger
	tracer           trace.Tracer
}

type response struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

var cepRegex = regexp.MustCompile(regextCepPattern)

func main() {
	logger := log.New(os.Stderr, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	godotenv.Load()

	tracer, shutdown, err := telemetry.InitTelemetry("app2-service", "app2-tracer")
	if err != nil {
		log.Fatalf("INFO: failed to initialize telemetry: %v", err)
	}
	defer func() {
		logger.Println("INFO: shutting down telemetry...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			logger.Printf("ERROR: fail to shutdown telemetry gracefully: %v", err)
		}
		logger.Println("INFO: telemetry gone.")
	}()

	// API Weather (need env)
	weatherAPIKey := os.Getenv("WEATHER_API_KEY")
	if weatherAPIKey == "" {
		log.Fatal("ERROR: The env variable WEATHER_API_KEY is required.")
		return
	}

	app := &application{
		viaCepClient:     viacep.NewClient(logger, tracer),
		weatherApiClient: weatherapi.NewClient(weatherAPIKey, logger, tracer),
		logger:           logger,
		tracer:           tracer,
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	otelHandler := otelhttp.NewHandler(http.HandlerFunc(app.handler), "/app2-server")
	mux := http.NewServeMux()
	mux.Handle("/get-weather-by-cep", app.logRequest(otelHandler))

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// (Ctrl+C)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		app.logger.Printf("Server listernig port %s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Can not start server: %v", err)
		}
	}()

	<-stop

	logger.Println("INFO: shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Fail shutindown server gracefully: %v", err)
	}

	log.Println("INFO: server gone.")
}

func (app *application) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}

		// DEBUG: Imprime todos os cabeçalhos recebidos
		app.logger.Println("--- Headers Recebidos ---")
		for name, values := range r.Header {
			for _, value := range values {
				app.logger.Printf("Header: %s: %s", name, value)
			}
		}
		app.logger.Println("-------------------------")
		// Algo como log de acesso
		app.logger.Printf("Request: IP=%s Method=%s URL=%s User-Agent=\"%s\"", ip, r.Method, r.URL.RequestURI(), r.UserAgent())
		next.ServeHTTP(w, r)
	})
}

func (app *application) handler(w http.ResponseWriter, r *http.Request) {
	ctx, span := app.tracer.Start(r.Context(), "/get-weather-by-cep")
	defer span.End()

	cep := r.URL.Query().Get("cep")
	if cep == "" {
		http.Error(w, "parâmetro 'cep' é obrigatório", http.StatusBadRequest)
		return
	}

	if !cepRegex.MatchString(cep) {
		http.Error(w, "inválid zipcode", http.StatusUnprocessableEntity)
		return
	}

	// 1.
	address, err := app.viaCepClient.FindAddressByCep(ctx, cep)
	if err != nil {
		if err == viacep.ErrCepNotFound {
			http.Error(w, viacep.ErrCepNotFound.Error(), http.StatusNotFound)
		} else {
			app.logger.Printf("Error can not find CEP: %v", err)
			http.Error(w, InternalErrorMessage, http.StatusInternalServerError)
		}
		return
	}

	// 2.
	weather, err := app.weatherApiClient.FindTemperatureByCity(ctx, address.City)
	if err != nil {
		app.logger.Printf("Internal error while fetching temperature for the city %s: %v", address.City, err)
		http.Error(w, InternalErrorMessage, http.StatusInternalServerError)
		return
	}

	// 3.
	response := response{
		City:  address.City,
		TempC: weather.Current.TempC,
		TempF: weather.Current.TempF,
		TempK: weather.Current.TempC + 273.15, // Kelvin
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
