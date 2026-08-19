package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	domaintelegram "github.com/teamcutter/go-poker/internal/domain/telegram"
)

type Validator struct {
	botToken string
	maxAge   time.Duration
	now      func() time.Time
}

func NewValidator(botToken string, maxAge time.Duration) *Validator {
	return &Validator{botToken: botToken, maxAge: maxAge, now: time.Now}
}

func (v *Validator) Validate(initData string) (*domaintelegram.User, error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, fmt.Errorf("%w: malformed query string", domaintelegram.ErrInvalidInitData)
	}

	receivedHash := values.Get("hash")
	if receivedHash == "" {
		return nil, fmt.Errorf("%w: missing hash", domaintelegram.ErrInvalidInitData)
	}

	authDateRaw := values.Get("auth_date")
	if authDateRaw == "" {
		return nil, fmt.Errorf("%w: missing auth_date", domaintelegram.ErrInvalidInitData)
	}
	authDateUnix, err := strconv.ParseInt(authDateRaw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: bad auth_date", domaintelegram.ErrInvalidInitData)
	}
	authDate := time.Unix(authDateUnix, 0)

	if v.maxAge > 0 && v.now().After(authDate.Add(v.maxAge)) {
		return nil, domaintelegram.ErrExpiredInitData
	}

	pairs := make([]string, 0, len(values)-1)
	for key, vals := range values {
		if key == "hash" {
			continue
		}
		pairs = append(pairs, key+"="+vals[0])
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	secretKey := deriveSecretKey(v.botToken)
	expectedHash := hmacSHA256Hex(secretKey, dataCheckString)
	if !hmac.Equal([]byte(expectedHash), []byte(receivedHash)) {
		return nil, fmt.Errorf("%w: signature mismatch", domaintelegram.ErrInvalidInitData)
	}

	userRaw := values.Get("user")
	if userRaw == "" {
		return nil, fmt.Errorf("%w: missing user field", domaintelegram.ErrInvalidInitData)
	}

	var payload struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
	}
	if err := json.Unmarshal([]byte(userRaw), &payload); err != nil {
		return nil, fmt.Errorf("%w: bad user payload", domaintelegram.ErrInvalidInitData)
	}
	if payload.ID == 0 {
		return nil, fmt.Errorf("%w: user id missing", domaintelegram.ErrInvalidInitData)
	}

	return &domaintelegram.User{
		ID:        payload.ID,
		Username:  payload.Username,
		FirstName: payload.FirstName,
		AuthDate:  authDate,
	}, nil
}

func deriveSecretKey(botToken string) []byte {
	return hmacSHA256([]byte("WebAppData"), []byte(botToken))
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func hmacSHA256Hex(key []byte, data string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
