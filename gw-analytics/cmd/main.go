package main

import (
	"github.com/paxaf/itkFinal/gw-analytics/internal/app"
	"github.com/paxaf/itkFinal/gw-analytics/internal/logger"
)

func main() {
	application, err := app.New()
	if err != nil {
		logger.Fatal("init app: %v", err)
	}

	if err = application.Run(); err != nil {
		_ = application.Close()
		logger.Fatal("run app: %v", err)
	}

	if err = application.Close(); err != nil {
		logger.Fatal("close app: %v", err)
	}
}
