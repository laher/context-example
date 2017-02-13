/*
Example code for a lightning talk on context.Context

Key points:

 * Handling deadlines
 * Adding/retrieving Values
 * Wrapping a handler/mux with context-handling
 * Accessing a request via http.Request.GetContext() (since go1.7)
 * Client disconnection triggers 'Done' (since go1.8)

References:

https://golang.org/pkg/context/
https://blog.golang.org/context
https://tip.golang.org/doc/go1.8
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
	ClientIPKey = "client-ip"
	flush       = true
)

func main() {
	log.Println("Context example ...")
	log.Printf("Flush: %v\n", flush)
	mux := http.NewServeMux()
	mux.HandleFunc("/", defaultHandler)

	log.Fatal(http.ListenAndServe(":8085", wrapContext(mux)))
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
	log.Printf("Request complete")
}

func doWork(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// 'middleware' or wrapper
func wrapContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, "-", r.RequestURI)
		ci := r.RemoteAddr
		fwd := r.Header.Get("X-Forwarded-For")
		if fwd != "" {
			ci = fwd
		}
		ctx := context.WithValue(r.Context(), ClientIPKey, ci)
		deadline := time.Now().Add(time.Duration(3) * time.Second)
		dctx, cancel := context.WithDeadline(ctx, deadline)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(dctx))
	})
}

func getClientIP(ctx context.Context) string {
	return ctx.Value(ClientIPKey).(string)
}
