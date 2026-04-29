package main

import (
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/app"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/logger"
)

// @title Currency Wallet API
// @version 0.1.0
// @description HTTP API for registration, JWT authorization, balances, deposits, withdrawals and currency exchange.
// @BasePath /api/v1
// @schemes http
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
func main() {
	application, err := app.New()
	if err != nil {
		logger.Fatal("init app: %v", err)
	}

	if err := application.Run(); err != nil {
		_ = application.Close()
		logger.Fatal("run app: %v", err)
	}

	if err := application.Close(); err != nil {
		logger.Fatal("close app: %v", err)
	}
}
