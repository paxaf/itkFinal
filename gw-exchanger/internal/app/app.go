package app

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/paxaf/itkFinal/gw-exchanger/internal/config"
	"github.com/paxaf/itkFinal/gw-exchanger/internal/logger"
	"github.com/paxaf/itkFinal/gw-exchanger/internal/storages"
	"github.com/paxaf/itkFinal/gw-exchanger/internal/storages/postgres"
	"github.com/paxaf/itkFinal/gw-exchanger/internal/usecase"
	exchangegrpc "github.com/paxaf/itkFinal/proto-exchange/exchange"
	"google.golang.org/grpc"

	grpcHandler "github.com/paxaf/itkFinal/gw-exchanger/internal/transport/grpc"
)

type App struct {
	cfg        *config.Config
	log        *logger.Logger
	storage    storages.Storage
	grpcServer *grpc.Server
}

var configPathFlag = flag.String("c", config.DefaultConfigPath, "path to config env file")

func New() (*App, error) {
	configPath := configPathFromFlags()

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	log := logger.New(cfg.Logger.Level)
	log.Info("application initialized", map[string]interface{}{
		"config_path":   configPath,
		"grpc_addr":     cfg.GRPC.Address(),
		"postgres_host": cfg.Postgres.Host,
		"postgres_db":   cfg.Postgres.Name,
		"log_level":     cfg.Logger.Level,
	})
	pool, err := postgres.New(&cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	exchangerUC := usecase.New(pool)
	grpcServer := grpc.NewServer()
	handler := grpcHandler.NewHandler(exchangerUC, log)
	exchangegrpc.RegisterExchangeServiceServer(grpcServer, handler)

	return &App{
		cfg:        cfg,
		log:        log,
		storage:    pool,
		grpcServer: grpcServer,
	}, nil
}

func (a *App) Run() error {
	a.log.Info("starting application", map[string]interface{}{
		"grpc_addr": a.cfg.GRPC.Address(),
	})
	lis, err := net.Listen("tcp", a.cfg.GRPC.Address())
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC address %q: %w", a.cfg.GRPC.Address(), err)
	}

	errCh := make(chan error, 1)
	go func() {
		if serveErr := a.grpcServer.Serve(lis); serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
			errCh <- fmt.Errorf("failed to serve gRPC: %w", serveErr)
		}
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signalCh)

	select {
	case sig := <-signalCh:
		a.log.Info("shutdown signal received", map[string]interface{}{
			"signal": sig.String(),
		})
		return nil
	case serveErr := <-errCh:
		return serveErr
	}
}

func (a *App) Close() error {
	if a.grpcServer != nil {
		a.grpcServer.GracefulStop()
	}
	if a.storage != nil {
		if err := a.storage.Close(); err != nil {
			return fmt.Errorf("close storage: %w", err)
		}
	}
	a.log.Info("application shutdown completed")
	return nil
}

func configPathFromFlags() string {
	if !flag.Parsed() {
		flag.Parse()
	}

	path := strings.TrimSpace(*configPathFlag)
	if path == "" {
		return config.DefaultConfigPath
	}

	return path
}
