package telegram

import (
	"errors"
	"time"
)

var (
	ErrInvalidInitData = errors.New("invalid init data")
	ErrExpiredInitData = errors.New("init data expired")
)

type User struct {
	ID        int64
	Username  string
	FirstName string
	AuthDate  time.Time
}

type Validator interface {
	Validate(initData string) (*User, error)
}
