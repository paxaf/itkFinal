package http

import "github.com/gin-gonic/gin"

func NewRouter(h *Handler, accessLog bool) *gin.Engine {
	r := gin.New()
	if accessLog {
		r.Use(gin.Logger())
	}
	r.Use(gin.Recovery())

	v1 := r.Group("/api/v1")
	v1.GET("/health", h.Health)
	v1.POST("/register", h.Register)
	v1.POST("/login", h.Login)

	protected := v1.Group("")
	protected.Use(h.AuthMiddleware())
	protected.GET("/balance", h.GetBalance)
	protected.POST("/wallet/deposit", h.Deposit)
	protected.POST("/wallet/withdraw", h.Withdraw)
	protected.GET("/exchange/rates", h.GetExchangeRates)
	protected.POST("/exchange", h.Exchange)

	return r
}
