package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: %s <config>", os.Args[0])
	}

	cfg, err := loadConfig(os.Args[1])
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	authenticator, err := newGoogleAuthenticator(ctx, cfg)
	if err != nil {
		return err
	}

	bot, err := newDiscordBot(cfg, authenticator)
	if err != nil {
		return err
	}
	defer bot.Close()

	if err := bot.Open(); err != nil {
		return err
	}

	server := &http.Server{
		Addr:    cfg.HTTP.Addr,
		Handler: newHTTPServer(authenticator, bot),
	}

	listener, err := net.Listen(cfg.HTTP.Net, cfg.HTTP.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	serverErr := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErr:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
