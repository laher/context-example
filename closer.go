package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

func WrapContextWithHijack(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, "-", r.RequestURI)
		ctx := putClientIPIntoContext(r)
		deadline := time.Now().Add(time.Duration(3) * time.Second)
		dctx, cancel := context.WithDeadline(ctx, deadline)
		defer cancel()
		nextDone := &isDone{}
		go func() {
			<-dctx.Done()
			if !nextDone.done {
				w.WriteHeader(http.StatusGatewayTimeout)
				fmt.Fprintf(w, "Please back off. Cancelling operation\n")
				w.(http.Flusher).Flush()
				log.Printf("Gateway timeout")
				hj, ok := w.(http.Hijacker)
				if !ok {
					http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
					return
				}
				conn, _, err := hj.Hijack()
				if err != nil {
					return
				}
				err = conn.Close()
				if err != nil {
					return
				}
			}

			cancel()
		}()
		next.ServeHTTP(w, r.WithContext(dctx))
		nextDone.done = true
		log.Printf("Request complete")
	})
}
