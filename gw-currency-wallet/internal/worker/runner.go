package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/logger"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/usecase"
)

const (
	defaultPollInterval = time.Second
	defaultConcurrency  = 1
	defaultWalletsLimit = 128
)

type Worker struct {
	uc           usecase.QueueProcessor
	log          logger.Interface
	pollInterval time.Duration
	concurrency  int
	walletsLimit int

	inFlightMu sync.Mutex
	inFlight   map[string]struct{}

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func New(uc usecase.QueueProcessor, log logger.Interface, pollInterval time.Duration, concurrency int, walletsLimit int) *Worker {
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	if walletsLimit <= 0 {
		walletsLimit = defaultWalletsLimit
	}
	if log == nil {
		log = logger.New("info")
	}

	return &Worker{
		uc:           uc,
		log:          log,
		pollInterval: pollInterval,
		concurrency:  concurrency,
		walletsLimit: walletsLimit,
		inFlight:     make(map[string]struct{}),
	}
}

func (w *Worker) Run(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)

	done := make(chan struct{})
	w.mu.Lock()
	w.cancel = cancel
	w.done = done
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.cancel = nil
		close(done)
		w.done = nil
		w.mu.Unlock()
	}()

	jobs := make(chan string, w.walletsLimit)
	var wg sync.WaitGroup
	wg.Add(w.concurrency)
	for i := 0; i < w.concurrency; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case walletKey, ok := <-jobs:
					if !ok {
						return
					}
					w.processWallet(runCtx, walletKey)
				case <-runCtx.Done():
					return
				}
			}
		}()
	}

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	defer func() {
		close(jobs)
		wg.Wait()
	}()

	for {
		select {
		case <-ticker.C:
			walletKeys, err := w.uc.ListPendingWallets(runCtx, w.walletsLimit)
			if err != nil &&
				!errors.Is(err, context.Canceled) &&
				!errors.Is(err, context.DeadlineExceeded) {
				w.log.Error("list pending wallets: %v", err)
				continue
			}

			for _, walletKey := range walletKeys {
				if !w.markInFlight(walletKey) {
					continue
				}

				select {
				case jobs <- walletKey:
				case <-runCtx.Done():
					w.unmarkInFlight(walletKey)
					return
				}
			}
		case <-runCtx.Done():
			return
		}
	}
}

func (w *Worker) Close() {
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (w *Worker) processWallet(ctx context.Context, walletKey string) {
	defer w.unmarkInFlight(walletKey)
	defer func() {
		if p := recover(); p != nil {
			w.log.Error("worker panic recovered: %v", p)
		}
	}()

	err := w.uc.ProcessWallet(ctx, walletKey)
	if err != nil &&
		!errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) {
		w.log.Error("process wallet %s: %v", walletKey, err)
	}
}

func (w *Worker) markInFlight(walletKey string) bool {
	w.inFlightMu.Lock()
	defer w.inFlightMu.Unlock()

	if _, ok := w.inFlight[walletKey]; ok {
		return false
	}
	w.inFlight[walletKey] = struct{}{}
	return true
}

func (w *Worker) unmarkInFlight(walletKey string) {
	w.inFlightMu.Lock()
	defer w.inFlightMu.Unlock()
	delete(w.inFlight, walletKey)
}
