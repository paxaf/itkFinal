package http

import (
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/auth"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/logger"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/storages"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/usecase"
)

const userIDContextKey = "user_id"

type TokenParser interface {
	Parse(tokenValue string) (int64, error)
}

type Handler struct {
	uc          usecase.UseCase
	tokenParser TokenParser
	log         logger.Interface
}

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type moneyOperationRequest struct {
	Amount   json.Number `json:"amount"`
	Currency string      `json:"currency"`
}

type exchangeRequest struct {
	FromCurrency string      `json:"from_currency"`
	ToCurrency   string      `json:"to_currency"`
	Amount       json.Number `json:"amount"`
}

func NewHandler(uc usecase.UseCase, tokenParser TokenParser, log logger.Interface) *Handler {
	return &Handler{uc: uc, tokenParser: tokenParser, log: log}
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(stdhttp.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if !decodeJSON(c, &req) {
		return
	}

	_, err := h.uc.Register(c.Request.Context(), domain.RegisterUser{
		Username: req.Username,
		Password: req.Password,
		Email:    req.Email,
	})
	if err != nil {
		h.writeError(c, "register user", err)
		return
	}

	c.JSON(stdhttp.StatusCreated, gin.H{"message": "User registered successfully"})
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if !decodeJSON(c, &req) {
		return
	}

	token, err := h.uc.Login(c.Request.Context(), domain.LoginUser{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		h.writeError(c, "login user", err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{"token": token})
}

func (h *Handler) GetBalance(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	balance, err := h.uc.GetBalance(c.Request.Context(), userID)
	if err != nil {
		h.writeError(c, "get balance", err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{"balance": balancesToMajor(balance)})
}

func (h *Handler) Deposit(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req moneyOperationRequest
	if !decodeJSON(c, &req) {
		return
	}

	amountMinor, err := amountToMinor(req.Amount)
	if err != nil {
		h.writeError(c, "deposit", err)
		return
	}

	balance, err := h.uc.Deposit(c.Request.Context(), userID, req.Currency, amountMinor)
	if err != nil {
		h.writeError(c, "deposit", err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"message":     "Account topped up successfully",
		"new_balance": balancesToMajor(balance),
	})
}

func (h *Handler) Withdraw(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req moneyOperationRequest
	if !decodeJSON(c, &req) {
		return
	}

	amountMinor, err := amountToMinor(req.Amount)
	if err != nil {
		h.writeError(c, "withdraw", err)
		return
	}

	balance, err := h.uc.Withdraw(c.Request.Context(), userID, req.Currency, amountMinor)
	if err != nil {
		h.writeError(c, "withdraw", err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"message":     "Withdrawal successful",
		"new_balance": balancesToMajor(balance),
	})
}

func (h *Handler) GetExchangeRates(c *gin.Context) {
	rates, err := h.uc.GetExchangeRates(c.Request.Context())
	if err != nil {
		h.writeError(c, "get exchange rates", err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{"rates": rates})
}

func (h *Handler) Exchange(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req exchangeRequest
	if !decodeJSON(c, &req) {
		return
	}

	amountMinor, err := amountToMinor(req.Amount)
	if err != nil {
		h.writeError(c, "exchange", err)
		return
	}

	fromCurrency, err := domain.NormalizeCurrency(req.FromCurrency)
	if err != nil {
		h.writeError(c, "exchange", err)
		return
	}
	toCurrency, err := domain.NormalizeCurrency(req.ToCurrency)
	if err != nil {
		h.writeError(c, "exchange", err)
		return
	}

	result, err := h.uc.Exchange(c.Request.Context(), domain.ExchangeOperation{
		UserID:       userID,
		FromCurrency: fromCurrency,
		ToCurrency:   toCurrency,
		AmountMinor:  amountMinor,
	})
	if err != nil {
		h.writeError(c, "exchange", err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"message":          "Exchange successful",
		"exchanged_amount": minorToMajor(result.ExchangedAmountMinor),
		"new_balance":      balancesToMajor(result.NewBalance),
	})
}

func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenValue, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			c.AbortWithStatusJSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		userID, err := h.tokenParser.Parse(tokenValue)
		if err != nil {
			c.AbortWithStatusJSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set(userIDContextKey, userID)
		c.Next()
	}
}

func (h *Handler) writeError(c *gin.Context, action string, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidUsername),
		errors.Is(err, domain.ErrInvalidEmail),
		errors.Is(err, domain.ErrInvalidPassword),
		errors.Is(err, domain.ErrInvalidUserID),
		errors.Is(err, domain.ErrInvalidCurrency),
		errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, domain.ErrSameCurrency),
		errors.Is(err, domain.ErrConvertedAmountTooSmall):
		c.JSON(stdhttp.StatusBadRequest, gin.H{"error": "bad request"})
	case errors.Is(err, storages.ErrDuplicateUser):
		c.JSON(stdhttp.StatusBadRequest, gin.H{"error": "Username or email already exists"})
	case errors.Is(err, domain.ErrInvalidCredentials):
		c.JSON(stdhttp.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
	case errors.Is(err, storages.ErrUserNotFound):
		c.JSON(stdhttp.StatusNotFound, gin.H{"error": "user not found"})
	case errors.Is(err, domain.ErrInsufficientFunds):
		c.JSON(stdhttp.StatusBadRequest, gin.H{"error": "Insufficient funds or invalid amount"})
	case errors.Is(err, domain.ErrExchangeRateUnavailable):
		c.JSON(stdhttp.StatusInternalServerError, gin.H{"error": "Failed to retrieve exchange rates"})
	case errors.Is(err, auth.ErrInvalidToken):
		c.JSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
	default:
		if h.log != nil {
			h.log.Error("%s: %v", action, err)
		}
		c.JSON(stdhttp.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

func decodeJSON(c *gin.Context, dst interface{}) bool {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"error": "bad request"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"error": "bad request"})
		return false
	}

	return true
}

func amountToMinor(amount json.Number) (int64, error) {
	value := strings.TrimSpace(amount.String())
	if value == "" {
		return 0, domain.ErrInvalidAmount
	}
	if strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+") {
		return 0, domain.ErrInvalidAmount
	}

	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, domain.ErrInvalidAmount
	}

	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, domain.ErrInvalidAmount
	}

	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) > 2 {
			return 0, domain.ErrInvalidAmount
		}
	}
	for len(fraction) < 2 {
		fraction += "0"
	}

	cents, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, domain.ErrInvalidAmount
	}

	minor := whole*100 + cents
	if minor <= 0 {
		return 0, domain.ErrInvalidAmount
	}

	return minor, nil
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}
	return parts[1], true
}

func userIDFromContext(c *gin.Context) (int64, bool) {
	value, ok := c.Get(userIDContextKey)
	if !ok {
		return 0, false
	}

	userID, ok := value.(int64)
	return userID, ok && userID > 0
}

func balancesToMajor(balances map[string]int64) map[string]float64 {
	result := make(map[string]float64, len(balances))
	for currency, amountMinor := range balances {
		result[currency] = minorToMajor(amountMinor)
	}
	return result
}

func minorToMajor(amountMinor int64) float64 {
	return float64(amountMinor) / 100
}
