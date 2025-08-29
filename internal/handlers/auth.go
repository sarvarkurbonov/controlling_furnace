package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthCredentials is used for both sign-up and sign-in
type AuthCredentials struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Response DTOs for Swagger
type SignUpResponse struct {
	ID int `json:"id"`
}
type TokenResponse struct {
	Token string `json:"token"`
}
type ErrorResponse struct {
	Error string `json:"error"`
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

// @Summary      Sign up
// @Description  Register a new user
// @Tags         auth
// @ID           authSignUp
// @Accept       json
// @Produce      json
// @Param        input  body   AuthCredentials  true  "User credentials"
// @Success      200    {object}  SignUpResponse
// @Failure      400    {object}  ErrorResponse  "Invalid request"
// @Failure      500    {object}  ErrorResponse  "Internal server error"
// @Router       /auth/sign-up [post]
func (h *Handler) signUp(c *gin.Context) {
	var input AuthCredentials
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

// @Summary      Sign in
// @Description  Authenticate user and return a JWT token
// @Tags         auth
// @ID           authSignIn
// @Accept       json
// @Produce      json
// @Param        input  body   AuthCredentials  true  "User credentials"
// @Success      200    {object}  TokenResponse
// @Failure      400    {object}  ErrorResponse  "Invalid request"
// @Failure      401    {object}  ErrorResponse  "Unauthorized"
// @Router       /auth/sign-in [post]
func (h *Handler) signIn(c *gin.Context) {
	var input AuthCredentials
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
