package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/advbet/flower"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

type emailWorker struct {
	db    *sql.DB
	redis *redis.Client
	log   *slog.Logger
}

func (ew *emailWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ew.log.Info("context done")
			return
		case <-ticker.C:
			ew.log.Info("sending email")
		}
	}
}

type server struct {
	port int
	log  *slog.Logger
}

func (s *server) Run(ctx context.Context) {
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", s.port),
	}

	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)

		err := srv.ListenAndServe()

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.With("err", err).Error("server closed unexpectedly")
		}
	}()

	select {
	case <-ctx.Done():
	case <-doneCh:
		return
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := srv.Shutdown(closeCtx); err != nil {
		s.log.With("err", err).Error("cannot shutdown server")
	}

	<-doneCh
}

func main() {
	db, err := sql.Open("mysql", "user:pass@tcp(host:3306)/db_name?tls=skip-verify")
	if err != nil {
		panic(err)
	}

	defer db.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer rdb.Close()

	ew := &emailWorker{
		db:    db,
		redis: rdb,
		log:   slog.With("service", "emailWorker"),
	}

	s := &server{
		port: 4212,
		log:  slog.With("service", "server"),
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	opts := flower.Options{
		BeforeServiceStart: func(s string) {
			slog.With("service", s).Info("starting")
		},
		AfterServiceStop: func(s string) {
			slog.With("service", s).Info("stopped")
		},
	}

	slog.Info("app starting")

	flower.Run(ctx, opts,
		flower.ServiceGroup{
			"emailWorker": ew,
		},
		flower.ServiceGroup{
			"server": s,
		},
	)

	slog.Info("app gracefully closed")
}
