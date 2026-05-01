package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/paxaf/itkFinal/gw-analytics/internal/config"
	"github.com/paxaf/itkFinal/gw-analytics/internal/logger"
	"github.com/paxaf/itkFinal/gw-analytics/internal/storages"
	kafkaTransport "github.com/paxaf/itkFinal/gw-analytics/internal/transport/kafka"
	"github.com/paxaf/itkFinal/gw-analytics/internal/usecase"
)

const shutdownTimeout = 10 * time.Second

var ErrStorageNotImplemented = errors.New("analytics storage is not implemented")

type consumer interface {
	Run(ctx context.Context) error
	Close() error
}

type App struct {
	cfg      *config.Config
	log      logger.Interface
	storage  storages.Storage
	consumer consumer
}

var configPathFlag = flag.String("c", config.DefaultConfigPath, "path to config env file")

func New() (*App, error) {
	return NewWithStorage(nil)
}

func NewWithStorage(storage storages.Storage) (*App, error) {
	configPath := configPathFromFlags()

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	log := logger.New(cfg.Logger.Level)
	if storage == nil {
		return nil, fmt.Errorf("%w: Elasticsearch storage will be added separately", ErrStorageNotImplemented)
	}

	analyticsUC := usecase.New(storage)
	consumer := kafkaTransport.New(cfg.Kafka, analyticsUC, log)

	log.Info("application initialized", map[string]interface{}{
		"config_path":      configPath,
		"kafka_brokers":    cfg.Kafka.BrokerList(),
		"kafka_topic":      cfg.Kafka.Topic,
		"kafka_group_id":   cfg.Kafka.GroupID,
		"kafka_batch_size": cfg.Kafka.BatchSize,
		"log_level":        cfg.Logger.Level,
	})

	return &App{
		cfg:      cfg,
		log:      log,
		storage:  storage,
		consumer: consumer,
	}, nil
}

func (a *App) Run() error {
	a.log.Info("starting application", map[string]interface{}{
		"kafka_topic":    a.cfg.Kafka.Topic,
		"kafka_group_id": a.cfg.Kafka.GroupID,
	})

	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := a.consumer.Run(runCtx); err != nil {
			errCh <- fmt.Errorf("run kafka consumer: %w", err)
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

	if a.consumer != nil {
		if err := a.consumer.Close(); err != nil {
			closeErr = fmt.Errorf("close kafka consumer: %w", err)
		}
	}

	if a.storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := a.storage.Close(ctx); err != nil {
			if closeErr != nil {
				return fmt.Errorf("%v; close storage: %w", closeErr, err)
			}
			closeErr = fmt.Errorf("close storage: %w", err)
		}
	}

	if closeErr == nil && a.log != nil {
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
