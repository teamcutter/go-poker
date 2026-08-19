package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/teamcutter/go-poker/internal/domain/session"
	"github.com/teamcutter/go-poker/internal/domain/store"
	"github.com/teamcutter/go-poker/internal/domain/telegram"
	"github.com/teamcutter/go-poker/internal/domain/user"
)

var ErrInvalidInitData = errors.New("invalid telegram init data")

type UserRepository interface {
	Create(ctx context.Context, u *user.User) error
	FindByID(ctx context.Context, id int64) (*user.User, error)
}

type Service struct {
	users     UserRepository
	validator telegram.Validator
	sessions  session.Manager
	tx        store.Manager
	now       func() time.Time
}

func NewService(
	users UserRepository,
	validator telegram.Validator,
	sessions session.Manager,
	tx store.Manager,
	now func() time.Time,
) *Service {
	return &Service{users: users, validator: validator, sessions: sessions, tx: tx, now: now}
}

type Result struct {
	User     *user.User
	Token    string
	PublicID string
}

func (s *Service) Authenticate(ctx context.Context, initData string, tokenTTL time.Duration) (*Result, error) {
	tgUser, err := s.validator.Validate(initData)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidInitData, err)
	}

	u, err := s.users.FindByID(ctx, tgUser.ID)
	if errors.Is(err, store.ErrNotFound) {
		u, err = s.createUser(ctx, tgUser)
	}
	if err != nil {
		return nil, err
	}

	token, err := s.sessions.Create(session.Claims{
		UserID:    u.ID,
		ExpiresAt: s.now().Add(tokenTTL),
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &Result{User: u, Token: token, PublicID: s.sessions.PublicID(u.ID)}, nil
}

func (s *Service) createUser(ctx context.Context, tgUser *telegram.User) (*user.User, error) {
	var created *user.User
	err := s.tx.Run(ctx, func(ctx context.Context) error {
		u := &user.User{
			ID:        tgUser.ID,
			Username:  tgUser.Username,
			FirstName: tgUser.FirstName,
			CreatedAt: s.now(),
			UpdatedAt: s.now(),
		}
		if err := s.users.Create(ctx, u); err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		created = u
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}
