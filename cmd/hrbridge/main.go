package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/astronaut808/hrbridge/internal/agent"
)

var (
	configPath = flag.String("config", agent.DefaultConfigPath, "path to hrbridge.conf")
	showVer    = flag.Bool("version", false, "print version and exit")
)

func main() {
	flag.Parse()

	if *showVer {
		if _, err := os.Stdout.WriteString("hrbridge " + agent.Version + "\n"); err != nil {
			log.Fatal(err)
		}
		return
	}

	cfg, createdToken, err := agent.LoadOrCreateConfig(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if createdToken {
		log.Printf("generated auth token in %s", *configPath)
	}

	srv := agent.NewServer(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		if cfg.EnableTLS {
			if cfg.CertFile == "" || cfg.KeyFile == "" {
				log.Fatal("TLS is enabled, but certFile/keyFile are not configured")
			}
			log.Printf("HydraBridge listening on https://%s", cfg.Listen)
			errc <- srv.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
			return
		}
		log.Printf("HydraBridge listening on http://%s", cfg.Listen)
		errc <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	case err := <-errc:
		if err != nil {
			log.Fatalf("server: %v", err)
		}
	}
}
