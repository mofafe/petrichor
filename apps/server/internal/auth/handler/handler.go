package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mofafe/petrichor/apps/server/internal/auth/middleware"
	"github.com/mofafe/petrichor/apps/server/internal/auth/model"
	"github.com/mofafe/petrichor/apps/server/internal/auth/service"
)

type Handler struct {
	auth *service.Service
}

func New(auth *service.Service) *Handler {
	return &Handler{auth: auth}
}

func (h *Handler) Register(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	user, err := h.auth.Register(c.Request.Context(), req.Username, req.Password)
	switch {
	case errors.Is(err, service.ErrInvalidUsername), errors.Is(err, service.ErrInvalidPassword):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid username or password"})
	case errors.Is(err, service.ErrUsernameTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	default:
		c.JSON(http.StatusCreated, userResponseFrom(user))
	}
}

func (h *Handler) Login(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	token, user, err := h.auth.Login(c.Request.Context(), req.Username, req.Password)
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	default:
		c.JSON(http.StatusOK, loginResponse{Token: token, User: userResponseFrom(user)})
	}
}

func (h *Handler) Me(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.auth.CurrentUser(c.Request.Context(), userID)
	switch {
	case errors.Is(err, service.ErrInvalidToken):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	default:
		c.JSON(http.StatusOK, userResponseFrom(user))
	}
}

type credentialsRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	Token string       `json:"token"`
	User  userResponse `json:"user"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

func userResponseFrom(user model.User) userResponse {
	return userResponse{
		ID:        user.ID,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
	}
}
