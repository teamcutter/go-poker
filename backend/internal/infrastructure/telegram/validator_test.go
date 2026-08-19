package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	domaintelegram "github.com/teamcutter/go-poker/internal/domain/telegram"
)

func wantErr(t *testing.T, want error, err error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

const testBotToken = "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

func buildInitData(t *testing.T, botToken string, fields map[string]string) string {
	t.Helper()
	pairs := make([]string, 0, len(fields))
	for k, v := range fields {
		if k == "hash" {
			continue
		}
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	secret := deriveSecretKey(botToken)
	hash := hmacSHA256Hex(secret, dataCheckString)

	q := url.Values{}
	for k, v := range fields {
		q.Set(k, v)
	}
	q.Set("hash", hash)
	return q.Encode()
}

func TestValidateAcceptsValidInitData(t *testing.T) {
	now := time.Now()
	userJSON, _ := json.Marshal(map[string]any{"id": 42, "username": "tester", "first_name": "T"})

	initData := buildInitData(t, testBotToken, map[string]string{
		"auth_date": strconv.FormatInt(now.Add(-time.Minute).Unix(), 10),
		"query_id":  "AAHdF6IQAAAAAN0XohDhrOrc",
		"user":      string(userJSON),
	})

	v := NewValidator(testBotToken, time.Hour)
	u, err := v.Validate(initData)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if u.ID != 42 || u.Username != "tester" || u.FirstName != "T" {
		t.Fatalf("unexpected user: %+v", u)
	}
}

func TestValidateRejectsExpiredInitData(t *testing.T) {
	now := time.Now()
	userJSON, _ := json.Marshal(map[string]any{"id": 42})

	initData := buildInitData(t, testBotToken, map[string]string{
		"auth_date": strconv.FormatInt(now.Add(-2*time.Hour).Unix(), 10),
		"user":      string(userJSON),
	})

	v := NewValidator(testBotToken, time.Hour)
	_, err := v.Validate(initData)
	wantErr(t, domaintelegram.ErrExpiredInitData, err)
}

func TestValidateRejectsTamperedUser(t *testing.T) {
	now := time.Now()
	userJSON, _ := json.Marshal(map[string]any{"id": 42})

	initData := buildInitData(t, testBotToken, map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
		"user":      string(userJSON),
	})

	values, _ := url.ParseQuery(initData)
	values.Set("user", `{"id":"999"}`)

	v := NewValidator(testBotToken, time.Hour)
	_, err := v.Validate(values.Encode())
	wantErr(t, domaintelegram.ErrInvalidInitData, err)
}

func TestValidateRejectsWrongBotToken(t *testing.T) {
	now := time.Now()
	userJSON, _ := json.Marshal(map[string]any{"id": 42})

	initData := buildInitData(t, "wrong-token", map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
		"user":      string(userJSON),
	})

	v := NewValidator(testBotToken, time.Hour)
	_, err := v.Validate(initData)
	wantErr(t, domaintelegram.ErrInvalidInitData, err)
}

func TestValidateRejectsMissingHash(t *testing.T) {
	now := time.Now()
	initData := fmt.Sprintf("auth_date=%d", now.Unix())

	v := NewValidator(testBotToken, time.Hour)
	_, err := v.Validate(initData)
	wantErr(t, domaintelegram.ErrInvalidInitData, err)
}

func TestValidateIgnoresMaxAgeWhenZero(t *testing.T) {
	now := time.Now()
	userJSON, _ := json.Marshal(map[string]any{"id": 42})

	initData := buildInitData(t, testBotToken, map[string]string{
		"auth_date": strconv.FormatInt(now.Add(-48*time.Hour).Unix(), 10),
		"user":      string(userJSON),
	})

	v := NewValidator(testBotToken, 0)
	if _, err := v.Validate(initData); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
