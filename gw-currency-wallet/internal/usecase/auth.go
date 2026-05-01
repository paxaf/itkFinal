package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/domain"
	"github.com/paxaf/itkFinal/gw-currency-wallet/internal/storages"
	"golang.org/x/crypto/bcrypt"
)

func (s *Service) Register(ctx context.Context, user domain.RegisterUser) (domain.User, error) {
	user.Username = strings.TrimSpace(user.Username)
	user.Email = strings.TrimSpace(user.Email)

	if err := user.Validate(); err != nil {
		return domain.User{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}

	created, err := s.storage.CreateUser(ctx, user.Username, user.Email, string(passwordHash))
	if err != nil {
		return domain.User{}, err
	}

	return created, nil
}

func (s *Service) Login(ctx context.Context, user domain.LoginUser) (string, error) {
	user.Username = strings.TrimSpace(user.Username)

	if err := user.Validate(); err != nil {
		return "", err
	}

	credentials, err := s.storage.GetUserCredentialsByUsername(ctx, user.Username)
	if err != nil {
		if errors.Is(err, storages.ErrUserNotFound) {
			return "", domain.ErrInvalidCredentials
		}
		return "", err
	}

	if err = bcrypt.CompareHashAndPassword([]byte(credentials.PasswordHash), []byte(user.Password)); err != nil {
		return "", domain.ErrInvalidCredentials
	}

	token, err := s.tokenManager.Generate(credentials.ID)
	if err != nil {
		return "", err
	}

	return token, nil
}
