package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/httpui"
	"wayfinding-release-gate/internal/store"
)

type runtime struct {
	store    *store.FileStore
	service  *application.Service
	server   *http.Server
	listener net.Listener
}

func newRuntime(cfg config) (*runtime, error) {
	repository, err := store.Open(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	service := application.NewService(repository)
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		_ = repository.Close()
		return nil, err
	}
	server := &http.Server{Handler: httpui.New(service), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	return &runtime{repository, service, server, listener}, nil
}
func (r *runtime) serve() error {
	err := r.server.Serve(r.listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func (r *runtime) close(ctx context.Context) error {
	serverErr := r.server.Shutdown(ctx)
	storeErr := r.store.Close()
	return errors.Join(serverErr, storeErr)
}
func runServer(cfg config) error {
	rt, err := newRuntime(cfg)
	if err != nil {
		return err
	}
	fmt.Printf("导向放样审批台监听 %s\n", rt.listener.Addr())
	serveErr := make(chan error, 1)
	go func() { serveErr <- rt.serve() }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case err := <-serveErr:
		return err
	case <-signals:
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return rt.close(ctx)
	}
}
