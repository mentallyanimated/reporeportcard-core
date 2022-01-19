package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mentallyanimated/reporeportcard-core/graph"
)

type Server struct {
	httpRouter *chi.Mux
	httpServer *http.Server
}

func NewServer() *Server {
	router := chi.NewRouter()
	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}
	s := &Server{
		httpServer: httpServer,
		httpRouter: router,
	}

	s.registerRoutes()
	return s
}

func (s *Server) Start() {
	done := make(chan struct{})
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("Gracefully shutting down server")
		s.httpServer.Shutdown(context.Background())
		close(done)
	}()

	log.Println("Starting server...")
	if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Printf("server closed with error: %v", err)
	}
	<-done
}

func (s *Server) registerRoutes() {
	s.httpRouter.Get("/graph", s.graph())
}

func (s *Server) graph() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owner := r.URL.Query().Get("owner")
		repo := r.URL.Query().Get("repo")
		startParam := r.URL.Query().Get("start")
		endParam := r.URL.Query().Get("end")

		if owner == "" || repo == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		start := time.Unix(0, 0)
		end := time.Now()

		if startParam != "" {
			start, _ = time.Parse(time.RFC3339, startParam)
		}
		if endParam != "" {
			end, _ = time.Parse(time.RFC3339, endParam)
		}

		startExec := time.Now()
		pullDetails := graph.ImportRawData(owner, repo)
		log.Printf("Pulling data took %s", time.Since(startExec))

		startExec = time.Now()
		filteredPullDetails := graph.FilterPullDetailsByTime(pullDetails, start, end)
		log.Printf("Filtered pull details in %s", time.Since(startExec))

		startExec = time.Now()
		graph.BuildForceGraph(owner, repo, filteredPullDetails, w)
		log.Printf("Built graph in %s", time.Since(startExec))
	}
}
