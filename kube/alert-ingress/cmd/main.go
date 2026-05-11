package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/argues/argus/alert-ingress/internal/pubsub"
	"github.com/argues/argus/alert-ingress/internal/webhook"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	publisher := initPublisher(ctx)
	defer publisher.Close()

	mux := http.NewServeMux()
	mux.Handle("/webhooks/anomstack", webhook.New(publisher))

	port := os.Getenv("ALERT_INGRESS_PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      withLogging(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		log.Printf("alert-ingress listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
}

func initPublisher(ctx context.Context) pubsub.Publisher {
	mode := os.Getenv("ALERT_INGRESS_MODE")
	switch mode {
	case "gcp":
		p, err := pubsub.NewGCP(ctx, "", "")
		if err != nil {
			log.Fatalf("gcp pubsub: %v", err)
		}
		return p
	default:
		log.Println("alert-ingress running in stdout mode (set ALERT_INGRESS_MODE=gcp for PubSub)")
		return pubsub.NewStdout()
	}
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
