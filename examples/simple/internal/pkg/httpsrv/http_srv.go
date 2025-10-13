package httpsrv

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/cardinalby/depo"
)

const shutdownTimeout = 5 * time.Second

type server struct {
	addr    string
	handler http.Handler
}

func NewServer(addr string, handler http.Handler) depo.ReadinessRunnable {
	return &server{
		addr:    addr,
		handler: handler,
	}
}

func (s *server) Run(ctx context.Context, onReady func()) error {
	log.Println("starting HTTP server tcp listener")
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}
	log.Println("HTTP server tcp listener started")
	if onReady != nil {
		onReady()
	}
	httpSrv := http.Server{
		Handler: s.handler,
	}
	serveRes := make(chan error, 1)
	go func() {
		serveRes <- httpSrv.Serve(listener)
		log.Println("HTTP server Serve completed")
		close(serveRes)
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		log.Printf("shutting down HTTP server (cause: %v)\n", context.Cause(ctx).Error())
		return httpSrv.Shutdown(shutdownCtx)

	case err := <-serveRes:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server error: %w", err)
		}
		return nil
	}
}
