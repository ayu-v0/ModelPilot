package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ayu-v0/ModelPilot/internal/api"
)

func main() {
	maxJobs := 4
	if value := os.Getenv("MODELPILOT_MAX_JOBS"); value != "" {
		var err error
		maxJobs, err = strconv.Atoi(value)
		if err != nil || maxJobs < 1 || maxJobs > 32 {
			log.Fatal("MODELPILOT_MAX_JOBS must be 1..32")
		}
	}
	timeout := 30 * time.Minute
	if value := os.Getenv("MODELPILOT_JOB_TIMEOUT"); value != "" {
		var err error
		timeout, err = time.ParseDuration(value)
		if err != nil || timeout <= 0 {
			log.Fatal("MODELPILOT_JOB_TIMEOUT must be a positive duration")
		}
	}
	service, err := api.New(api.Options{
		Dir: os.Getenv("MODELPILOT_DATA_DIR"), APIKey: os.Getenv("MODELPILOT_API_KEY"),
		AllowedHosts: strings.Split(os.Getenv("MODELPILOT_ALLOWED_HOSTS"), ","),
		MaxJobs:      maxJobs, JobTimeout: timeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	addr := os.Getenv("MODELPILOT_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: service, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErrors := make(chan error, 1)
	go func() { log.Printf("ModelPilot API listening on %s", addr); serveErrors <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
	case err := <-serveErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server failed: %v", err)
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		log.Printf("HTTP shutdown: %v", err)
		_ = server.Close()
	}
	service.Close()
}
