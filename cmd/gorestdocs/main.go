package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gorestdocs "github.com/alek-sys/go-rest-docs"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: gorestdocs <command> [flags]\n\ncommands:\n  replay  start a replay mock server\n")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "replay":
		if err := runReplay(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runReplay(args []string) error {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	session := fs.String("session", "", "session directory to replay (required)")
	port := fs.Int("port", 8080, "port to listen on")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *session == "" {
		return fmt.Errorf("--session is required")
	}

	srv, err := gorestdocs.NewReplayServer(*session)
	if err != nil {
		return err
	}

	endpoints := srv.Endpoints()
	fmt.Printf("gorestdocs replay server\n")
	fmt.Printf("session: %s\n", *session)
	fmt.Printf("loaded %d endpoint(s):\n", len(endpoints))
	for _, ep := range endpoints {
		fmt.Printf("  %s\n", ep)
	}
	fmt.Printf("listening on :%d\n", *port)

	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: srv.Handler(),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		fmt.Println("\nshutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
}
