package handlers

import (
	"controlling_furnace/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Common response/status constants to avoid magic strings and typos.
const (
	statusOK      = "ok"
	statusStarted = "started"
	statusStopped = "stopped"
	statusModeSet = "mode_set"

	errStartFurnace    = "failed to start furnace"
	errStopFurnace     = "failed to stop furnace"
	errGetState        = "failed to load state"
	errInvalidBodyPref = "invalid body: "
)

// Centralized error logging and response.
func (h *Handler) logAndJSONError(c *gin.Context, httpCode int, userMsg, logKey string, err error, kv ...interface{}) {
	if h.log != nil && err != nil {
		fields := append([]interface{}{"err", err}, kv...)
		h.log.Errorw(logKey, fields...)
	}
	c.JSON(httpCode, gin.H{"error": userMsg})
}

// Respond with a status and include current state if available (best-effort).
func (h *Handler) respondWithStatusAndState(c *gin.Context, status string, extra gin.H) {
	ctx := c.Request.Context()
	resp := gin.H{"status": status}
	for k, v := range extra {
		resp[k] = v
	}
	st, err := h.services.Monitoring.GetState(ctx)
	if err == nil {
		resp["state"] = st
	}
	c.JSON(http.StatusOK, resp)
}

// Request DTO for setting mode.
type modeRequest struct {
	Mode        string  `json:"mode" binding:"required"` // HEAT | COOL | STANDBY
	TargetTempC float64 `json:"target_temp_c,omitempty"` // required if mode=HEAT
	DurationSec int     `json:"duration_sec,omitempty"`  // required if mode=HEAT
}

// SetModeRequest is an exported model for Swagger docs of the setMode payload.
type SetModeRequest struct {
	// Mode to set. Allowed: HEAT, COOL, STANDBY
	Mode string `json:"mode" example:"HEAT"`
	// Target temperature in Celsius (required when mode=HEAT)
	TargetTempC float64 `json:"target_temp_c,omitempty" example:"850"`
	// Heating duration in seconds (required when mode=HEAT)
	DurationSec int `json:"duration_sec,omitempty" example:"600"`
}

// @Summary      Health check
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /health [get]
func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": statusOK,
	})
}

// @Summary      Start furnace
// @Tags         furnace
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "status, state"
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/furnace/start [post]
// @Security     BearerAuth
func (h *Handler) startFurnace(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.services.Furnace.Start(ctx); err != nil {
		h.logAndJSONError(c, http.StatusInternalServerError, errStartFurnace, "furnace_start_failed", err)
		return
	}
	h.respondWithStatusAndState(c, statusStarted, gin.H{})
}

// @Summary      Stop furnace
// @Tags         furnace
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/furnace/stop [post]
// @Security     BearerAuth
func (h *Handler) stopFurnace(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.services.Furnace.Stop(ctx); err != nil {
		h.logAndJSONError(c, http.StatusInternalServerError, errStopFurnace, "furnace_stop_failed", err)
		return
	}
	h.respondWithStatusAndState(c, statusStopped, gin.H{})
}

// @Summary      Set mode
// @Description  HEAT requires target_temp_c and duration_sec
// @Tags         furnace
// @Accept       json
// @Produce      json
// @Param        body  body   SetModeRequest  true  "Mode payload"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /api/v1/furnace/mode [post]
// @Security     BearerAuth
func (h *Handler) setMode(c *gin.Context) {
	var req modeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errInvalidBodyPref + err.Error()})
		return
	}
	ctx := c.Request.Context()
	params := service.ModeParams{
		Mode:        req.Mode,
		TargetTempC: req.TargetTempC,
		DurationSec: req.DurationSec,
	}
	if err := h.services.Furnace.SetMode(ctx, params); err != nil {
		// Treat as bad request if validation failed in service; otherwise internal error.
		// (You can refine this by returning typed errors from service.)
		if h.log != nil {
			h.log.Errorw("furnace_set_mode_failed", "err", err, "mode", req.Mode)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.respondWithStatusAndState(c, statusModeSet, gin.H{"mode": req.Mode})
}

// @Summary      Get furnace state
// @Tags         furnace
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/furnace/state [get]
// @Security     BearerAuth
func (h *Handler) getState(c *gin.Context) {
	ctx := c.Request.Context()
	st, err := h.services.Monitoring.GetState(ctx)
	if err != nil {
		h.logAndJSONError(c, http.StatusInternalServerError, errGetState, "furnace_get_state_failed", err)
		return
	}
	c.JSON(http.StatusOK, st)
}
