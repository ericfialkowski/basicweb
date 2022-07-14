package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ericfialkowski/env"
	"github.com/ericfialkowski/status"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/joho/godotenv/autoload"
)

var (
	port = env.GetIntOrDefault("port", 8900)
	ip   = env.GetStringOrDefault("ip", "")
)

func main() {
	// set up http router
	router := mux.NewRouter()

	// Can remove the top level router if no catch-all handling is wanted/needed
	apiRouter := router.PathPrefix("/api/").Subrouter()
	v1router := apiRouter.PathPrefix("/v1/").Subrouter()
	v2router := apiRouter.PathPrefix("/v2/").Subrouter()

	// add handlers
	simpleStatus := status.NewStatus()
	v1router.HandleFunc("/health/full", simpleStatus.Handler)
	v1router.HandleFunc("/", helloHandler).Methods(http.MethodGet)

	v2router.HandleFunc("/", panicHandler)

	router.Use(handlers.RecoveryHandler())
	router.PathPrefix("/").HandlerFunc(catchAllHandler)

	bindAddr := fmt.Sprintf("%s:%d", ip, port)
	log.Printf("Listening to %s", bindAddr)

	// good to go
	simpleStatus.Ok("All good")

	srv := &http.Server{
		Addr: bindAddr,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: env.GetDurationOrDefault("http_write_timeout", time.Second*10),
		ReadTimeout:  env.GetDurationOrDefault("http_read_timeout", time.Second*15),
		IdleTimeout:  env.GetDurationOrDefault("http_idle_timeout", time.Second*60),
		// wrap the router with a request logging handler
		Handler: handlers.LoggingHandler(os.Stdout, router),
	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	// we're ready to accept requests
	simpleStatus.Ok("All good")

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// change status to indicate shutting down
	simpleStatus.Ok("Shutting down")

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), env.GetDurationOrDefault("shutdown_wait_timeout", time.Second*15))
	defer cancel()

	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	_ = srv.Shutdown(ctx)

	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services to finalize
	// based on context cancellation.

	log.Println("shutting down")
	os.Exit(0)
}

func panicHandler(_ http.ResponseWriter, _ *http.Request) {
	panic("Panic at the disco")
}

func helloHandler(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(writer).Encode("Hello World!"); err != nil {
		log.Printf("Couldn't encode/write json: %v", err)
	}
}

func catchAllHandler(writer http.ResponseWriter, _ *http.Request) {
	//
	// This is where you would do any special handling for random requests
	//
	writer.WriteHeader(http.StatusNotFound)
}
