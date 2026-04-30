package grpc

import (
	"context"
	"errors"
	"strings"

	"github.com/paxaf/itkFinal/gw-exchanger/internal/logger"
	"github.com/paxaf/itkFinal/gw-exchanger/internal/usecase"
	exchangegrpc "github.com/paxaf/itkFinal/proto-exchange/exchange"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	usecase usecase.Exchanger
	log     *logger.Logger
	exchangegrpc.UnimplementedExchangeServiceServer
}

func NewHandler(usecase usecase.Exchanger, log *logger.Logger) *Handler {
	return &Handler{
		usecase: usecase,
		log:     log,
	}
}

func (h *Handler) GetExchangeRates(ctx context.Context, _ *exchangegrpc.Empty) (*exchangegrpc.ExchangeRatesResponse, error) {
	res, err := h.usecase.GetRates(ctx)
	if err != nil {
		h.log.Error("failed to get exchange rates", map[string]interface{}{"error": err.Error()})
		return nil, status.Error(codes.Internal, "failed to get exchange rates")
	}

	responseRates := make(map[string]float32, len(res))
	for currency, rate := range res {
		responseRates[currency] = float32(rate)
	}
	return &exchangegrpc.ExchangeRatesResponse{
		Rates: responseRates,
	}, nil
}

func (h *Handler) GetExchangeRateForCurrency(ctx context.Context, req *exchangegrpc.CurrencyRequest) (*exchangegrpc.ExchangeRateResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	from := req.GetFromCurrency()
	to := req.GetToCurrency()
	res, err := h.usecase.GetRate(ctx, from, to)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidCurrency):
			return nil, status.Error(codes.InvalidArgument, "invalid currency code")

		default:
			h.log.Error("failed to get exchange rate", map[string]interface{}{
				"error": err.Error(),
				"from":  from,
				"to":    to,
			})

			return nil, status.Error(codes.Internal, "failed to get exchange rate")
		}
	}
	return &exchangegrpc.ExchangeRateResponse{
		FromCurrency: strings.ToUpper(strings.TrimSpace(from)),
		ToCurrency:   strings.ToUpper(strings.TrimSpace(to)),
		ExchangeRate: float32(res)}, nil
}
