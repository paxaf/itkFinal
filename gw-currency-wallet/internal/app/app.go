package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/auth"
	exchangerClient "github.com/paxaf/itkFinal/gw-currency-wallet/internal/clients/exchanger"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/config"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/logger"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/storages"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/storages/postgres"
	walletHTTP "github.com/paxaf/itkFinal/gw-currency-wallet/internal/transport/http"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/usecase"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/worker"
)

const (
	shutdownTimeout   = 10 * time.Second
	readHeaderTimeout = 5 * time.Second
)

type App struct {
	cfg           *config.Config
	log           *logger.Logger
	apiStorage    storages.Storage
	workerStorage storages.Storage
	exchanger     *exchangerClient.Client
	server        *http.Server
	worker        *worker.Worker
	path          string
}

var configPathFlag = flag.String("c", config.DefaultConfigPath, "path to config env file")

func New() (*App, error) {
	configPath := configPathFromFlags()

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	log := logger.New(cfg.Logger.Level)
	if strings.ToLower(cfg.Logger.Level) != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	apiPostgresCfg := cfg.Postgres
	apiPostgresCfg.MaxOpenConns = cfg.Postgres.APIMaxOpenConns
	apiStorage, err := postgres.New(&apiPostgresCfg)
	if err != nil {
		return nil, fmt.Errorf("create api storage: %w", err)
	}

	workerPostgresCfg := cfg.Postgres
	workerPostgresCfg.MaxOpenConns = cfg.Postgres.WorkerMaxOpenConns
	workerStorage, err := postgres.New(&workerPostgresCfg)
	if err != nil {
		_ = apiStorage.Close()
		return nil, fmt.Errorf("create worker storage: %w", err)
	}

	tokenManager := auth.NewManager(cfg.Auth.JWTSecret, cfg.Auth.TokenTTL())
	exchanger, err := exchangerClient.New(cfg.Exchanger.Address(), cfg.Exchanger.RequestTimeout())
	if err != nil {
		_ = apiStorage.Close()
		_ = workerStorage.Close()
		return nil, fmt.Errorf("create exchanger client: %w", err)
	}

	walletUC := usecase.New(apiStorage, tokenManager, exchanger)
	processor := usecase.NewProcessor(workerStorage, usecase.ProcessorConfig{BatchSize: cfg.Worker.BatchSize})
	bgWorker := worker.New(processor, log, cfg.Worker.PollInterval(), cfg.Worker.Concurrency, cfg.Worker.WalletsLimit)

	handler := walletHTTP.NewHandler(walletUC, tokenManager, log)
	server := &http.Server{
		Addr:              cfg.HTTP.Address(),
		Handler:           walletHTTP.NewRouter(handler, cfg.HTTP.AccessLog),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	log.Info("application initialized", map[string]interface{}{
		"config_path":   configPath,
		"http_addr":     cfg.HTTP.Address(),
		"postgres_host": cfg.Postgres.Host,
		"postgres_db":   cfg.Postgres.Name,
		"exchanger":     cfg.Exchanger.Address(),
		"log_level":     cfg.Logger.Level,
	})

	return &App{
		cfg:           cfg,
		log:           log,
		apiStorage:    apiStorage,
		workerStorage: workerStorage,
		exchanger:     exchanger,
		server:        server,
		worker:        bgWorker,
		path:          configPath,
	}, nil
}

func (a *App) Run() error {
	a.log.Info("starting application", map[string]interface{}{
		"http_addr": a.cfg.HTTP.Address(),
	})

	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if a.worker != nil {
		go a.worker.Run(runCtx)
	}

	errCh := make(chan error, 1)
	go func() {
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen and serve: %w", err)
		}
	}()

	select {
	case <-runCtx.Done():
		a.log.Info("shutdown signal received")
		return nil
	case err := <-errCh:
		return err
	}
}

func (a *App) Close() error {
	var closeErr error

	if a.worker != nil {
		a.worker.Close()
	}

	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := a.server.Shutdown(ctx); err != nil {
			closeErr = fmt.Errorf("shutdown server: %w", err)
		}
	}

	if a.apiStorage != nil {
		if err := a.apiStorage.Close(); err != nil {
			if closeErr != nil {
				return fmt.Errorf("%v; close api storage: %w", closeErr, err)
			}
			closeErr = fmt.Errorf("close api storage: %w", err)
		}
	}

	if a.workerStorage != nil {
		if err := a.workerStorage.Close(); err != nil {
			if closeErr != nil {
				return fmt.Errorf("%v; close worker storage: %w", closeErr, err)
			}
			closeErr = fmt.Errorf("close worker storage: %w", err)
		}
	}

	if a.exchanger != nil {
		if err := a.exchanger.Close(); err != nil {
			if closeErr != nil {
				return fmt.Errorf("%v; close exchanger client: %w", closeErr, err)
			}
			closeErr = fmt.Errorf("close exchanger client: %w", err)
		}
	}

	if closeErr == nil {
		a.log.Info("application shutdown completed")
	}
	return closeErr
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
