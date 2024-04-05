package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	otel_metrics "github.com/VictorH97/devfullcycle/goexpert/Lab-Temp-CEP-OTEL/internal/infra/otel-metrics"
	"github.com/VictorH97/devfullcycle/goexpert/Lab-Temp-CEP-OTEL/internal/infra/web"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
)

func init() {
	viper.AutomaticEnv()
}

func main() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	shutdown, err := otel_metrics.InitProvider(viper.GetString("OTEL_SERVICE_NAME"), viper.GetString("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal("failed to shutdown TracerProvider: %w", err)
		}
	}()

	tracer := otel.Tracer("microservice-tracer")
	cepServer := web.NewServerCep("microservice-cep-request", tracer)
	router := cepServer.CreateServer()

	go func() {
		log.Println("Starting server on port", "8080")
		if err := http.ListenAndServe(":8080", router); err != nil {
			log.Fatal(err)
		}
	}()

	select {
	case <-sigCh:
		log.Println("Shutting down gracefully, CTRL+C pressed...")
	case <-ctx.Done():
		log.Println("Shutting down due to other reason...")
	}

	_, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
}
