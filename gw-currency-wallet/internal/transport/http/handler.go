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

type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

type RegisterRequest struct {
	Username string `json:"username" example:"paxaf"`
	Password string `json:"password" example:"secret1"`
	Email    string `json:"email" example:"paxaf@example.com"`
}

type LoginRequest struct {
	Username string `json:"username" example:"paxaf"`
	Password string `json:"password" example:"secret1"`
}

type MoneyOperationRequest struct {
	Amount   float64 `json:"amount" example:"100.50"`
	Currency string  `json:"currency" enums:"USD,EUR,RUB" example:"USD"`
}

type ExchangeRequest struct {
	FromCurrency string  `json:"from_currency" enums:"USD,EUR,RUB" example:"USD"`
	ToCurrency   string  `json:"to_currency" enums:"USD,EUR,RUB" example:"EUR"`
	Amount       float64 `json:"amount" example:"100"`
}

type MessageResponse struct {
	Message string `json:"message" example:"User registered successfully"`
}

type LoginResponse struct {
	Token string `json:"token" example:"jwt-token"`
}

type BalanceResponse struct {
	Balance map[string]float64 `json:"balance"`
}

type NewBalanceResponse struct {
	Message    string             `json:"message" example:"Account topped up successfully"`
	NewBalance map[string]float64 `json:"new_balance"`
}

type RatesResponse struct {
	Rates map[string]float64 `json:"rates"`
}

type ExchangeResponse struct {
	Message         string             `json:"message" example:"Exchange successful"`
	ExchangedAmount float64            `json:"exchanged_amount" example:"92"`
	NewBalance      map[string]float64 `json:"new_balance"`
}

type ErrorResponse struct {
	Error string `json:"error" example:"bad request"`
}

func NewHandler(uc usecase.UseCase, tokenParser TokenParser, log logger.Interface) *Handler {
	return &Handler{uc: uc, tokenParser: tokenParser, log: log}
}

// Health godoc
// @Summary Check service health
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (h *Handler) Health(c *gin.Context) {
	h.logInfo("health checked", nil)
	c.JSON(stdhttp.StatusOK, gin.H{"status": "ok"})
}

// Register godoc
// @Summary Register a user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Register request"
// @Success 201 {object} MessageResponse
// @Failure 400 {object} ErrorResponse
// @Router /register [post]
func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if !decodeJSON(c, &req) {
		h.logWarn("register user failed", map[string]interface{}{"error": "bad request body"})
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

	h.logInfo("user registered", map[string]interface{}{"username": req.Username})
	c.JSON(stdhttp.StatusCreated, gin.H{"message": "User registered successfully"})
}

// Login godoc
// @Summary Login and receive JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login request"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /login [post]
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if !decodeJSON(c, &req) {
		h.logWarn("login user failed", map[string]interface{}{"error": "bad request body"})
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

	h.logInfo("user logged in", map[string]interface{}{"username": req.Username})
	c.JSON(stdhttp.StatusOK, gin.H{"token": token})
}

// GetBalance godoc
// @Summary Get user balances
// @Tags wallet
// @Produce json
// @Security BearerAuth
// @Success 200 {object} BalanceResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /balance [get]
func (h *Handler) GetBalance(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		h.logWarn("get balance failed", map[string]interface{}{"error": "missing user id"})
		c.JSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	balance, err := h.uc.GetBalance(c.Request.Context(), userID)
	if err != nil {
		h.writeError(c, "get balance", err)
		return
	}

	h.logInfo("balance received", map[string]interface{}{"user_id": userID})
	c.JSON(stdhttp.StatusOK, gin.H{"balance": balancesToMajor(balance)})
}

// Deposit godoc
// @Summary Deposit money to wallet
// @Tags wallet
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body MoneyOperationRequest true "Deposit request"
// @Success 200 {object} NewBalanceResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /wallet/deposit [post]
func (h *Handler) Deposit(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		h.logWarn("deposit failed", map[string]interface{}{"error": "missing user id"})
		c.JSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req moneyOperationRequest
	if !decodeJSON(c, &req) {
		h.logWarn("deposit failed", map[string]interface{}{"user_id": userID, "error": "bad request body"})
		return
	}

	amountMinor, err := amountToMinor(req.Amount)
	if err != nil {
		h.writeError(c, "deposit", err, map[string]interface{}{"user_id": userID, "currency": req.Currency})
		return
	}

	balance, err := h.uc.Deposit(c.Request.Context(), userID, req.Currency, amountMinor)
	if err != nil {
		h.writeError(c, "deposit", err, map[string]interface{}{
			"user_id":      userID,
			"currency":     req.Currency,
			"amount_minor": amountMinor,
		})
		return
	}

	h.logInfo("deposit completed", map[string]interface{}{
		"user_id":      userID,
		"currency":     req.Currency,
		"amount_minor": amountMinor,
	})
	c.JSON(stdhttp.StatusOK, gin.H{
		"message":     "Account topped up successfully",
		"new_balance": balancesToMajor(balance),
	})
}

// Withdraw godoc
// @Summary Withdraw money from wallet
// @Tags wallet
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body MoneyOperationRequest true "Withdraw request"
// @Success 200 {object} NewBalanceResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /wallet/withdraw [post]
func (h *Handler) Withdraw(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		h.logWarn("withdraw failed", map[string]interface{}{"error": "missing user id"})
		c.JSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req moneyOperationRequest
	if !decodeJSON(c, &req) {
		h.logWarn("withdraw failed", map[string]interface{}{"user_id": userID, "error": "bad request body"})
		return
	}

	amountMinor, err := amountToMinor(req.Amount)
	if err != nil {
		h.writeError(c, "withdraw", err, map[string]interface{}{"user_id": userID, "currency": req.Currency})
		return
	}

	balance, err := h.uc.Withdraw(c.Request.Context(), userID, req.Currency, amountMinor)
	if err != nil {
		h.writeError(c, "withdraw", err, map[string]interface{}{
			"user_id":      userID,
			"currency":     req.Currency,
			"amount_minor": amountMinor,
		})
		return
	}

	h.logInfo("withdraw completed", map[string]interface{}{
		"user_id":      userID,
		"currency":     req.Currency,
		"amount_minor": amountMinor,
	})
	c.JSON(stdhttp.StatusOK, gin.H{
		"message":     "Withdrawal successful",
		"new_balance": balancesToMajor(balance),
	})
}

// GetExchangeRates godoc
// @Summary Get exchange rates
// @Tags exchange
// @Produce json
// @Security BearerAuth
// @Success 200 {object} RatesResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /exchange/rates [get]
func (h *Handler) GetExchangeRates(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		h.logWarn("get exchange rates failed", map[string]interface{}{"error": "missing user id"})
		c.JSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rates, err := h.uc.GetExchangeRates(c.Request.Context())
	if err != nil {
		h.writeError(c, "get exchange rates", err, map[string]interface{}{"user_id": userID})
		return
	}

	h.logInfo("exchange rates received", map[string]interface{}{"user_id": userID})
	c.JSON(stdhttp.StatusOK, gin.H{"rates": rates})
}

// Exchange godoc
// @Summary Exchange money between currencies
// @Tags exchange
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ExchangeRequest true "Exchange request"
// @Success 200 {object} ExchangeResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /exchange [post]
func (h *Handler) Exchange(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		h.logWarn("exchange failed", map[string]interface{}{"error": "missing user id"})
		c.JSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req exchangeRequest
	if !decodeJSON(c, &req) {
		h.logWarn("exchange failed", map[string]interface{}{"user_id": userID, "error": "bad request body"})
		return
	}

	amountMinor, err := amountToMinor(req.Amount)
	if err != nil {
		h.writeError(c, "exchange", err, map[string]interface{}{"user_id": userID})
		return
	}

	fromCurrency, err := domain.NormalizeCurrency(req.FromCurrency)
	if err != nil {
		h.writeError(c, "exchange", err, map[string]interface{}{"user_id": userID, "from_currency": req.FromCurrency})
		return
	}
	toCurrency, err := domain.NormalizeCurrency(req.ToCurrency)
	if err != nil {
		h.writeError(c, "exchange", err, map[string]interface{}{"user_id": userID, "to_currency": req.ToCurrency})
		return
	}

	result, err := h.uc.Exchange(c.Request.Context(), domain.ExchangeOperation{
		UserID:       userID,
		FromCurrency: fromCurrency,
		ToCurrency:   toCurrency,
		AmountMinor:  amountMinor,
	})
	if err != nil {
		h.writeError(c, "exchange", err, map[string]interface{}{
			"user_id":       userID,
			"from_currency": fromCurrency,
			"to_currency":   toCurrency,
			"amount_minor":  amountMinor,
		})
		return
	}

	h.logInfo("exchange completed", map[string]interface{}{
		"user_id":                userID,
		"from_currency":          fromCurrency,
		"to_currency":            toCurrency,
		"amount_minor":           amountMinor,
		"exchanged_amount_minor": result.ExchangedAmountMinor,
	})
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
			h.logWarn("authorization failed", map[string]interface{}{
				"method": c.Request.Method,
				"path":   c.Request.URL.Path,
				"error":  "missing bearer token",
			})
			c.AbortWithStatusJSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		userID, err := h.tokenParser.Parse(tokenValue)
		if err != nil {
			h.logWarn("authorization failed", map[string]interface{}{
				"method": c.Request.Method,
				"path":   c.Request.URL.Path,
				"error":  "invalid bearer token",
			})
			c.AbortWithStatusJSON(stdhttp.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set(userIDContextKey, userID)
		c.Next()
	}
}

func (h *Handler) writeError(c *gin.Context, action string, err error, fields ...map[string]interface{}) {
	status := stdhttp.StatusInternalServerError
	message := "internal error"

	switch {
	case errors.Is(err, domain.ErrInvalidUsername),
		errors.Is(err, domain.ErrInvalidEmail),
		errors.Is(err, domain.ErrInvalidPassword),
		errors.Is(err, domain.ErrInvalidUserID),
		errors.Is(err, domain.ErrInvalidCurrency),
		errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, domain.ErrSameCurrency),
		errors.Is(err, domain.ErrConvertedAmountTooSmall):
		status = stdhttp.StatusBadRequest
		message = "bad request"
	case errors.Is(err, storages.ErrDuplicateUser):
		status = stdhttp.StatusBadRequest
		message = "Username or email already exists"
	case errors.Is(err, domain.ErrInvalidCredentials):
		status = stdhttp.StatusUnauthorized
		message = "Invalid username or password"
	case errors.Is(err, storages.ErrUserNotFound):
		status = stdhttp.StatusNotFound
		message = "user not found"
	case errors.Is(err, domain.ErrInsufficientFunds):
		status = stdhttp.StatusBadRequest
		message = "Insufficient funds or invalid amount"
	case errors.Is(err, domain.ErrExchangeRateUnavailable):
		status = stdhttp.StatusInternalServerError
		message = "Failed to retrieve exchange rates"
	case errors.Is(err, auth.ErrInvalidToken):
		status = stdhttp.StatusUnauthorized
		message = "unauthorized"
	}

	logFields := mergeLogFields(fields...)
	logFields["status"] = status
	logFields["error"] = err.Error()
	if status >= stdhttp.StatusInternalServerError {
		h.logError(action+" failed", logFields)
	} else {
		h.logWarn(action+" failed", logFields)
	}

	c.JSON(status, gin.H{"error": message})
}

func (h *Handler) logInfo(message string, fields map[string]interface{}) {
	if h.log == nil {
		return
	}
	if len(fields) == 0 {
		h.log.Info(message)
		return
	}
	h.log.Info(message, fields)
}

func (h *Handler) logWarn(message string, fields map[string]interface{}) {
	if h.log == nil {
		return
	}
	if len(fields) == 0 {
		h.log.Warn(message)
		return
	}
	h.log.Warn(message, fields)
}

func (h *Handler) logError(message string, fields map[string]interface{}) {
	if h.log == nil {
		return
	}
	if len(fields) == 0 {
		h.log.Error(message)
		return
	}
	h.log.Error(message, fields)
}

func mergeLogFields(fields ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, fieldSet := range fields {
		for key, value := range fieldSet {
			result[key] = value
		}
	}
	return result
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
