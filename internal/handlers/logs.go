package handlers

import (
	"controlling_furnace/internal/service"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	errFromInvalid = "invalid 'from' time; use RFC3339 or YYYY-MM-DD"
	errToInvalid   = "invalid 'to' time; use RFC3339 or YYYY-MM-DD"

	layoutDateTime = "2006-01-02 15:04:05"
	layoutDate     = "2006-01-02"
)

// isDateOnly reports whether the query string represents a date without time component.
func isDateOnly(s string) bool {
	return !strings.ContainsAny(s, "T ")
}

// @Summary      List logs
// @Description  Filter logs by date (RFC3339, 'YYYY-MM-DD HH:MM:SS', or 'YYYY-MM-DD'). If 'to' is date-only, it is treated as end-of-day inclusive (23:59:59.999999999Z).
// @Tags         logs
// @Produce      json
// @Param        from  query   string  false  "Start of range (RFC3339, 'YYYY-MM-DD HH:MM:SS', or 'YYYY-MM-DD')"  example(2025-08-01)
// @Param        to    query   string  false  "End of range (RFC3339, 'YYYY-MM-DD HH:MM:SS', or 'YYYY-MM-DD'). Date-only treated as end of day."  example(2025-08-31)
// @Param        type  query   string  false  "Event type"  Enums(START,MODE_CHANGE,STOP,ERROR)
// @Success      200   {object}  map[string]interface{}  "count, events"
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /api/v1/logs [get]
// @Security     BearerAuth
func (h *Handler) getLogs(c *gin.Context) {
	ctx := c.Request.Context()
	var (
		from time.Time
		to   time.Time
		// Normalize event type: trim spaces and uppercase to match expected values.
		eventType = strings.ToUpper(strings.TrimSpace(c.Query("type")))
		err       error
	)
	// Parse 'from' (optional)
	if qs := c.Query("from"); qs != "" {
		from, err = parseQueryTime(qs)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": errFromInvalid})
			return
		}
	}
	// Parse 'to' (optional). If only a date is provided, make it end-of-day inclusive.
	if qs := c.Query("to"); qs != "" {
		to, err = parseQueryTime(qs)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": errToInvalid})
			return
		}
		// If the user didn't include a time component, treat "to" as the end of that day.
		if isDateOnly(qs) {
			to = to.Add(24*time.Hour - time.Nanosecond).UTC()
		}
	}
	// Validate range if both provided
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'from' must be <= 'to'"})
		return
	}
	events, err := h.services.EventLog.List(ctx, service.LogFilter{
		From: from,
		To:   to,
		Type: eventType,
	})
	if err != nil {
		if h.log != nil {
			h.log.Errorw("logs_list_failed", "err", err, "from", from, "to", to, "type", eventType)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load logs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"count":  len(events),
		"events": events,
	})
}

// ... existing code ...
func parseQueryTime(s string) (time.Time, error) {
	// Try multiple accepted formats, normalizing to UTC.
	for _, layout := range []string{time.RFC3339, layoutDateTime, layoutDate} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf(
		"invalid time format %q, expected one of: "+
			"RFC3339 (e.g. 2025-08-27T15:04:05Z), "+
			"'YYYY-MM-DD HH:MM:SS', "+
			"'YYYY-MM-DD'",
		s,
	)
}

// ... existing code ...
