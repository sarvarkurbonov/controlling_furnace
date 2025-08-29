package handlers

import (
	"controlling_furnace/internal/logger"
	"controlling_furnace/internal/service"

	"github.com/gin-gonic/gin"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Handler wires HTTP layer to services and logging.
type Handler struct {
	services *service.Service
	log      *logger.Logger
}

// NewHandler constructs a new HTTP handler with dependencies.
func NewHandler(services *service.Service, log *logger.Logger) *Handler {
	return &Handler{services: services, log: log}
}

// InitRoutes builds and returns the Gin router with all routes registered.
func (h *Handler) InitRoutes() *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health endpoint
	router.GET("/health", h.health)

	// Auth endpoints
	h.registerAuthRoutes(router)

	// Versioned API endpoints (protected)
	h.registerAPIRoutes(router)

	// Minimal WebSocket connection (HTTP upgrade) — same port
	router.GET("/ws", h.wsConnect)

	return router
}

func (h *Handler) registerAuthRoutes(r *gin.Engine) {
	auth := r.Group("/auth")
	{
		auth.POST("/sign-up", h.signUp)
		auth.POST("/sign-in", h.signIn)
	}
}

func (h *Handler) registerAPIRoutes(r *gin.Engine) {
	api := r.Group("/api/v1", h.userIdMiddleware)
	{
		h.registerFurnaceRoutes(api)
		h.registerLogRoutes(api)
	}
}

func (h *Handler) registerFurnaceRoutes(api *gin.RouterGroup) {
	furnace := api.Group("/furnace")
	{
		furnace.POST("/start", h.startFurnace)
		furnace.POST("/stop", h.stopFurnace)
		// Body example: {"mode":"HEAT","target_c":850,"duration_s":600}
		furnace.POST("/mode", h.setMode)
		furnace.GET("/state", h.getState)
	}
}

func (h *Handler) registerLogRoutes(api *gin.RouterGroup) {
	logs := api.Group("/logs")
	{
		logs.GET("/", h.getLogs)
	}
}
