/*
Example code for a lightning talk on context.Context

See article here: https://medium.com/@ambot/exploring-the-context-package-db30a818d563#.2q34adjd7

*/
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	ClientIPKey           = "client-ip"
	flush                 = true
	DefaultTimeoutSeconds = 3
)

func main() {
	log.Println("Context example ...")
	log.Printf("Flush: %v\n", flush)
	mux := http.NewServeMux()
	mux.HandleFunc("/", defaultHandler)
	mux.HandleFunc("/cancel", cancelHandler)
	mux.HandleFunc("/buggy", buggyHandler)

	log.Fatal(http.ListenAndServe(":8765", WrapContext(mux)))
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {

	d := r.URL.Query().Get("d")
	di, err := time.ParseDuration(d)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Bad request: please specify duration with e.g. '?d=1s' (%s)\n", err.Error())
		log.Printf("Bad request")
		return
	}
	log.Printf("Query for %v", di.String())
	if flush {
		fmt.Fprintf(w, "Sloow query here\n")
		w.(http.Flusher).Flush()
	}
	select {
	case <-doWork(di):
		//slow query here
		fmt.Fprintf(w, "Slow query complete... \n")
	case <-r.Context().Done():
		// timeout
		if !flush {
			w.WriteHeader(http.StatusGatewayTimeout)
		}
		fmt.Fprintf(w, "Please back off. Cancelling operation\n")
		log.Printf("Gateway timeout")
		return
	}

	fmt.Fprintf(w, "Hello GoAKL (IP: %s)\n", getClientIP(r.Context()))
}

func cancelHandler(w http.ResponseWriter, r *http.Request) {
	d := r.URL.Query().Get("d")
	di, err := time.ParseDuration(d)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Bad request: please specify duration with e.g. '?d=1s' (%s)\n", err.Error())
		log.Printf("Bad request")
		return
	}
	cctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-time.After(di)
		cancel()
		log.Printf("Context cancelled")
	}()
	go func() {
		<-cctx.Done()
		log.Printf("Done received multiple times woop")
	}()
	log.Printf("Query for %v", di.String())

	select {
	case <-cctx.Done():
		//slow query here
		fmt.Fprintf(w, "Slow query complete... \n")
	case <-r.Context().Done():
		w.WriteHeader(http.StatusGatewayTimeout)
		fmt.Fprintf(w, "Please back off. Cancelling operation\n")
		log.Printf("Gateway timeout")
		return
	}
	fmt.Fprintf(w, "Context cancelled yay\n")
}

func buggyHandler(w http.ResponseWriter, r *http.Request) {
	ch := make(chan int)
	ch <- 1
	fmt.Fprintf(w, "Hello GoAKL (IP: %s)\n", getClientIP(r.Context()))
}

func doWork(d time.Duration) <-chan time.Time {
	return time.After(d)
}

type isDone struct {
	done bool
}

// 'middleware' or wrapper
// could be used for Auth, logging, requestid generation
func WrapContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, "-", r.RequestURI)
		ctx := putClientIPIntoContext(r)
		deadline := time.Now().Add(time.Duration(DefaultTimeoutSeconds) * time.Second)
		dctx, cancel := context.WithDeadline(ctx, deadline)
		defer cancel()
		nextDone := &isDone{}
		next.ServeHTTP(w, r.WithContext(dctx))
		nextDone.done = true
		log.Printf("Request complete")
	})
}

func putClientIPIntoContext(r *http.Request) context.Context {
	ci := r.RemoteAddr
	fwd := r.Header.Get("X-Forwarded-For")
	if fwd != "" {
		ci = fwd
	}
	ctx := context.WithValue(r.Context(), ClientIPKey, ci)
	return ctx
}

func getClientIP(ctx context.Context) string {
	return ctx.Value(ClientIPKey).(string)
}
