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
	"google.golang.org/grpc"
)

type App struct {
	cfg        *config.Config
	log        *logger.Logger
	grpcServer *grpc.Server
	path       string
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

	grpcServer := grpc.NewServer()

	return &App{
		cfg:        cfg,
		log:        log,
		grpcServer: grpcServer,
		path:       configPath,
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
	a.grpcServer.GracefulStop()
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
