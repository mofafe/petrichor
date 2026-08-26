package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mofafe/petrichor/apps/server/internal/auth/model"
	"github.com/mofafe/petrichor/apps/server/internal/auth/repository"
	"golang.org/x/crypto/bcrypt"
)

const tokenTTL = 24 * time.Hour

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalidUsername    = errors.New("invalid username")
	ErrUsernameTaken      = errors.New("username already exists")

	usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{2,31}$`)
)

type UserRepository interface {
	Create(ctx context.Context, user model.User) error
	FindByID(ctx context.Context, id string) (model.User, error)
	FindByUsername(ctx context.Context, username string) (model.User, error)
	UsernameExists(ctx context.Context, username string) (bool, error)
}

type Service struct {
	users     UserRepository
	jwtSecret []byte
	now       func() time.Time
}

func New(users UserRepository, jwtSecret string) (*Service, error) {
	if jwtSecret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}

	return &Service{
		users:     users,
		jwtSecret: []byte(jwtSecret),
		now:       time.Now,
	}, nil
}

func (s *Service) Register(ctx context.Context, username string, password string) (model.User, error) {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return model.User{}, ErrInvalidUsername
	}
	if err := validatePassword(password); err != nil {
		return model.User{}, err
	}

	exists, err := s.users.UsernameExists(ctx, username)
	if err != nil {
		return model.User{}, err
	}
	if exists {
		return model.User{}, ErrUsernameTaken
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, fmt.Errorf("hash password: %w", err)
	}

	now := s.now().UTC()
	user := model.User{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: string(passwordHash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return model.User{}, err
	}

	return user, nil
}

func (s *Service) Login(ctx context.Context, username string, password string) (string, model.User, error) {
	user, err := s.users.FindByUsername(ctx, strings.TrimSpace(username))
	if errors.Is(err, repository.ErrUserNotFound) {
		return "", model.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", model.User{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", model.User{}, ErrInvalidCredentials
	}

	token, err := s.issueToken(user.ID)
	if err != nil {
		return "", model.User{}, err
	}

	return token, user, nil
}

func (s *Service) CurrentUser(ctx context.Context, userID string) (model.User, error) {
	user, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, repository.ErrUserNotFound) {
		return model.User{}, ErrInvalidToken
	}
	if err != nil {
		return model.User{}, err
	}

	return user, nil
}

func (s *Service) UserIDFromToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}

		return s.jwtSecret, nil
	}, jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return "", ErrInvalidToken
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return "", ErrInvalidToken
	}

	return claims.Subject, nil
}

func (s *Service) issueToken(userID string) (string, error) {
	now := s.now().UTC()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		IssuedAt:  jwt.NewNumericDate(now),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return signed, nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return ErrInvalidPassword
	}
	if len(password) > 72 {
		return ErrInvalidPassword
	}

	return nil
}
