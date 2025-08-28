package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Single, shared credentials payload for both sign-up and sign-in.
type authCredentials struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// bindJSONOrBadRequest tries to bind the request body into dst and writes a 400 JSON on failure.
// Returns false if the request was already handled (aborted), true otherwise.
func (h *Handler) bindJSONOrBadRequest(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		// optional structured logging
		if h.log != nil {
			h.log.Infow("auth_bad_request_body", "err", err)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}
	return true
}

// ... existing code ...
func (h *Handler) signUp(c *gin.Context) {
	var input authCredentials
	if ok := h.bindJSONOrBadRequest(c, &input); !ok {
		return
	}

	id, err := h.services.SignUp(input.Username, input.Password)
	if err != nil {
		if h.log != nil {
			h.log.Infow("auth_sign_up_failed", "username", input.Username, "err", err)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id})
}

// ... existing code ...
func (h *Handler) signIn(c *gin.Context) {
	var input authCredentials
	if ok := h.bindJSONOrBadRequest(c, &input); !ok {
		return
	}

	token, err := h.services.GenerateToken(input.Username, input.Password)
	if err != nil {
		if h.log != nil {
			h.log.Infow("auth_sign_in_failed", "username", input.Username, "err", err)
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// ... existing code ...
