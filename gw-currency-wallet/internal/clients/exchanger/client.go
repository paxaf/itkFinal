package exchanger

import (
	"context"
	"fmt"
	"time"

	exchangegrpc "github.com/paxaf/itkFinal/proto-exchange/exchange"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn    *grpc.ClientConn
	client  exchangegrpc.ExchangeServiceClient
	timeout time.Duration
}

func New(address string, timeout time.Duration) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create exchanger grpc client: %w", err)
	}

	return &Client{
		conn:    conn,
		client:  exchangegrpc.NewExchangeServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *Client) GetRates(ctx context.Context) (map[string]float64, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.client.GetExchangeRates(ctx, &exchangegrpc.Empty{})
	if err != nil {
		return nil, fmt.Errorf("get exchange rates: %w", err)
	}

	rates := make(map[string]float64, len(resp.GetRates()))
	for currency, rate := range resp.GetRates() {
		rates[currency] = float64(rate)
	}

	return rates, nil
}

func (c *Client) GetRate(ctx context.Context, fromCurrency string, toCurrency string) (float64, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.client.GetExchangeRateForCurrency(ctx, &exchangegrpc.CurrencyRequest{
		FromCurrency: fromCurrency,
		ToCurrency:   toCurrency,
	})
	if err != nil {
		return 0, fmt.Errorf("get exchange rate for currency: %w", err)
	}

	return float64(resp.GetExchangeRate()), nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.timeout)
}
