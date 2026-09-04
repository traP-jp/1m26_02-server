package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	traqbot "github.com/traPtitech/traq-bot"
)

func main() {
	logger := log.New(os.Stdout, "BOT_MAI: ", log.LstdFlags|log.LUTC)
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		logger.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var provisioner *devProvisioner
	var provisioned provisionedBot
	if cfg.AutoRegister {
		provisioner, provisioned, err = provisionDevBot(ctx, cfg, logger)
		if err != nil {
			logger.Fatal(err)
		}
		cfg.AccessToken = provisioned.AccessToken
		cfg.VerificationToken = provisioned.VerificationToken
		logger.Printf("development bot is configured (bot ID: %s)", provisioned.ID)
	}

	client := newTraQClient(cfg.APIBaseURL, cfg.AccessToken)
	handlers := newEventHandlers(&bot{poster: client, executor: newCommandExecutor(client), state: client, logger: logger})
	botServer := traqbot.NewBotServer(cfg.VerificationToken, handlers)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", botServer)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		logger.Fatal(err)
	}
	serveErr := make(chan error, 1)
	go func() {
		logger.Printf("listening on :%s", cfg.Port)
		serveErr <- server.Serve(listener)
	}()

	if provisioner != nil {
		activateCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err = provisioner.activate(activateCtx, provisioned.ID)
		cancel()
		if err != nil {
			_ = server.Close()
			logger.Fatal(err)
		}
		logger.Printf("development bot is active")
	}

	select {
	case err = <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal(err)
		}
		return
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Printf("failed to shut down gracefully: %v", err)
	}
	<-serveErr
}
