package http

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	authapp "github.com/teamcutter/go-poker/internal/application/auth"
	pokerapp "github.com/teamcutter/go-poker/internal/application/poker"
	domainsession "github.com/teamcutter/go-poker/internal/domain/session"
	"github.com/teamcutter/go-poker/internal/domain/store"
	"github.com/teamcutter/go-poker/internal/domain/user"
	sessioninfra "github.com/teamcutter/go-poker/internal/infrastructure/session"
	telegraminfra "github.com/teamcutter/go-poker/internal/infrastructure/telegram"
)

const scenarioBotToken = "123456:scenario-bot-token"

type scenarioUsers struct{}

func (scenarioUsers) FindByID(_ context.Context, id int64) (*user.User, error) {
	if id != 42 && id != 43 {
		return nil, store.ErrNotFound
	}
	return &user.User{ID: id, PublicID: fmt.Sprintf("public-%d", id), FirstName: "Player"}, nil
}

func (scenarioUsers) Create(context.Context, *user.User) error {
	return nil
}

type scenarioAPI struct {
	server   *httptest.Server
	service  *pokerapp.Service
	sessions *sessioninfra.Manager
}

func newScenarioAPI(t *testing.T) *scenarioAPI {
	t.Helper()
	sessions := sessioninfra.NewManager("scenario-session-secret", "gopoker", time.Hour)
	auth := authapp.NewService(scenarioUsers{}, telegraminfra.NewValidator(scenarioBotToken, time.Hour), sessions, nil, time.Now)
	service := pokerapp.NewService(nil)
	ws := NewWSHandler(service, sessions, zap.NewNop())
	router := NewRouter(zap.NewNop(), sessions, NewAuthHandler(auth, time.Hour), NewPokerHandler(service), ws, nil, nil, 0, 0)
	router.Register()
	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)
	return &scenarioAPI{server: server, service: service, sessions: sessions}
}

func scenarioInitData(id int64) string {
	fields := url.Values{}
	fields.Set("auth_date", strconv.FormatInt(time.Now().Unix(), 10))
	fields.Set("user", fmt.Sprintf(`{"id":%d,"first_name":"Player"}`, id))
	keys := []string{"auth_date", "user"}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, key+"="+fields.Get(key))
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(scenarioBotToken))
	signature := hmac.New(sha256.New, secret.Sum(nil))
	signature.Write([]byte(strings.Join(pairs, "\n")))
	fields.Set("hash", hex.EncodeToString(signature.Sum(nil)))
	return fields.Encode()
}

func (a *scenarioAPI) request(t *testing.T, method, path, token string, body any) (int, map[string]json.RawMessage) {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, a.server.URL+path, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var envelope map[string]json.RawMessage
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, envelope
}

func scenarioData[T any](t *testing.T, envelope map[string]json.RawMessage) T {
	t.Helper()
	var data T
	if err := json.Unmarshal(envelope["data"], &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	return data
}

func scenarioError(t *testing.T, status, wantStatus int, envelope map[string]json.RawMessage, wantCode string) {
	t.Helper()
	if status != wantStatus {
		t.Fatalf("status %d, want %d: %s", status, wantStatus, envelope)
	}
	var problem struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(envelope["error"], &problem); err != nil {
		t.Fatalf("error envelope: %v", err)
	}
	if problem.Code != wantCode || problem.Message == "" {
		t.Fatalf("error = %+v, want code %q and nonempty message", problem, wantCode)
	}
}

func (a *scenarioAPI) login(t *testing.T, id int64) string {
	t.Helper()
	status, envelope := a.request(t, http.MethodPost, "/api/auth/telegram", "", map[string]string{"init_data": scenarioInitData(id)})
	if status != http.StatusOK {
		t.Fatalf("login status %d: %s", status, envelope)
	}
	data := scenarioData[struct {
		Token string `json:"token"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}](t, envelope)
	if data.Token == "" || data.User.ID != fmt.Sprintf("public-%d", id) {
		t.Fatalf("unexpected login response: %+v", data)
	}
	return data.Token
}

func (a *scenarioAPI) create(t *testing.T, token string) string {
	t.Helper()
	status, envelope := a.request(t, http.MethodPost, "/api/tables", token, map[string]int{"max_seats": 3, "big_blind": 10})
	if status != http.StatusOK {
		t.Fatalf("create status %d: %s", status, envelope)
	}
	data := scenarioData[struct {
		Table struct {
			Code string `json:"code"`
		} `json:"table"`
	}](t, envelope)
	if data.Table.Code == "" {
		t.Fatal("missing table code")
	}
	return data.Table.Code
}

func (a *scenarioAPI) wallet(t *testing.T, token string) int64 {
	t.Helper()
	status, envelope := a.request(t, http.MethodGet, "/api/poker/wallet", token, nil)
	if status != http.StatusOK {
		t.Fatalf("wallet status %d: %s", status, envelope)
	}
	return scenarioData[struct {
		Bankroll int64 `json:"bankroll"`
	}](t, envelope).Bankroll
}

func (a *scenarioAPI) table(t *testing.T, token, code string) struct {
	Code    string `json:"code"`
	Phase   string `json:"phase"`
	Players []struct {
		ID    string `json:"id"`
		Stack int64  `json:"stack"`
	} `json:"players"`
} {
	t.Helper()
	status, envelope := a.request(t, http.MethodGet, "/api/tables/"+code, token, nil)
	if status != http.StatusOK {
		t.Fatalf("get table status %d: %s", status, envelope)
	}
	return scenarioData[struct {
		Code    string `json:"code"`
		Phase   string `json:"phase"`
		Players []struct {
			ID    string `json:"id"`
			Stack int64  `json:"stack"`
		} `json:"players"`
	}](t, envelope)
}

func TestScenario20001ValidTelegramLoginAndSession(t *testing.T) {
	a := newScenarioAPI(t)
	token := a.login(t, 42)
	if got := a.wallet(t, token); got != 1000 {
		t.Fatalf("bankroll = %d, want 1000", got)
	}
	parts := strings.Split(token, ".")
	parts[2] = strings.Repeat("A", len(parts[2]))
	tampered := strings.Join(parts, ".")
	expired, err := a.sessions.Create(domainsession.Claims{PublicID: "public-42", ExpiresAt: time.Now().Add(-time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{tampered, expired} {
		status, _ := a.request(t, http.MethodGet, "/api/poker/wallet", invalid, nil)
		if status != http.StatusUnauthorized {
			t.Fatalf("invalid session status = %d, want 401", status)
		}
	}
}

func TestScenario20002TamperedAndEmptyInitData(t *testing.T) {
	a := newScenarioAPI(t)
	valid := scenarioInitData(42)
	values, err := url.ParseQuery(valid)
	if err != nil {
		t.Fatal(err)
	}
	values.Set("user", `{"id":43,"first_name":"Player"}`)
	for _, initData := range []string{values.Encode(), ""} {
		status, envelope := a.request(t, http.MethodPost, "/api/auth/telegram", "", map[string]string{"init_data": initData})
		wantCode := "invalid_init_data"
		wantStatus := http.StatusUnauthorized
		if initData == "" {
			wantCode = "bad_request"
			wantStatus = http.StatusBadRequest
		}
		scenarioError(t, status, wantStatus, envelope, wantCode)
		if _, ok := envelope["data"]; ok {
			t.Fatal("failed login returned data")
		}
	}
}

func TestScenario20003CreateAndReadTable(t *testing.T) {
	a := newScenarioAPI(t)
	token := a.login(t, 42)
	code := a.create(t, token)
	if got := a.table(t, token, code).Code; got != code {
		t.Fatalf("table code = %q, want %q", got, code)
	}
	status, envelope := a.request(t, http.MethodGet, "/api/tables", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list status %d: %s", status, envelope)
	}
	tables := scenarioData[[]struct {
		Code string `json:"code"`
	}](t, envelope)
	if len(tables) != 1 || tables[0].Code != code {
		t.Fatalf("listed tables = %+v, want %q", tables, code)
	}
	status, envelope = a.request(t, http.MethodPost, "/api/tables", "", map[string]int{"max_seats": 2})
	if status != http.StatusUnauthorized {
		t.Fatalf("anonymous create status %d: %s", status, envelope)
	}
}

func TestScenario20004JoinMovesBuyInToStack(t *testing.T) {
	a := newScenarioAPI(t)
	owner := a.login(t, 42)
	guest := a.login(t, 43)
	code := a.create(t, owner)
	before := a.wallet(t, guest)
	status, envelope := a.request(t, http.MethodPost, "/api/tables/"+code+"/join", guest, map[string]int{"buy_in": 200})
	if status != http.StatusOK {
		t.Fatalf("join status %d: %s", status, envelope)
	}
	if got := a.wallet(t, guest); got != before-200 {
		t.Fatalf("bankroll = %d, want %d", got, before-200)
	}
	table := a.table(t, guest, code)
	if len(table.Players) != 1 || table.Players[0].ID != "public-43" || table.Players[0].Stack != 200 {
		t.Fatalf("unexpected seat: %+v", table.Players)
	}
}

func TestScenario20005InsufficientBuyInKeepsBalancesAndSeat(t *testing.T) {
	a := newScenarioAPI(t)
	token := a.login(t, 42)
	first := a.create(t, token)
	status, envelope := a.request(t, http.MethodPost, "/api/tables/"+first+"/join", token, map[string]int{"buy_in": 900})
	if status != http.StatusOK {
		t.Fatalf("initial join status %d: %s", status, envelope)
	}
	other := a.create(t, token)
	status, envelope = a.request(t, http.MethodPost, "/api/tables/"+other+"/join", token, map[string]int{"buy_in": 200})
	scenarioError(t, status, http.StatusConflict, envelope, "not_enough_chips")
	if got := a.wallet(t, token); got != 100 {
		t.Fatalf("bankroll changed to %d", got)
	}
	if stack := a.table(t, token, first).Players[0].Stack; stack != 900 {
		t.Fatalf("existing stack changed to %d", stack)
	}
	if seats := len(a.table(t, token, other).Players); seats != 0 {
		t.Fatalf("failed join occupied %d seats", seats)
	}
}

func TestScenario20006LeaveRefundsStack(t *testing.T) {
	a := newScenarioAPI(t)
	token := a.login(t, 42)
	code := a.create(t, token)
	a.request(t, http.MethodPost, "/api/tables/"+code+"/join", token, map[string]int{"buy_in": 400})
	before := a.wallet(t, token)
	stack := a.table(t, token, code).Players[0].Stack
	status, envelope := a.request(t, http.MethodPost, "/api/tables/"+code+"/leave", token, nil)
	if status != http.StatusOK {
		t.Fatalf("leave status %d: %s", status, envelope)
	}
	if got := a.wallet(t, token); got != before+stack {
		t.Fatalf("bankroll = %d, want %d", got, before+stack)
	}
	if seats := len(a.table(t, token, code).Players); seats != 0 {
		t.Fatalf("leave retained %d seats", seats)
	}
}

func TestScenario20007StartNeedsTwoFundedPlayers(t *testing.T) {
	a := newScenarioAPI(t)
	token := a.login(t, 42)
	code := a.create(t, token)
	a.request(t, http.MethodPost, "/api/tables/"+code+"/join", token, map[string]int{"buy_in": 200})
	status, envelope := a.request(t, http.MethodPost, "/api/tables/"+code+"/start", token, nil)
	scenarioError(t, status, http.StatusConflict, envelope, "not_your_deal")
	if phase := a.table(t, token, code).Phase; phase != "waiting" {
		t.Fatalf("phase changed to %q", phase)
	}
}

func TestScenario20008TopUpAndRepeatClaim(t *testing.T) {
	a := newScenarioAPI(t)
	token := a.login(t, 42)
	code := a.create(t, token)
	a.request(t, http.MethodPost, "/api/tables/"+code+"/join", token, map[string]int{"buy_in": 1000})
	tb, err := a.service.Get(code)
	if err != nil {
		t.Fatal(err)
	}
	tb.Players[0].Stack = 0
	a.request(t, http.MethodPost, "/api/tables/"+code+"/leave", token, nil)
	before := a.wallet(t, token)
	if before != 0 {
		t.Fatalf("bankroll = %d, want an empty wallet", before)
	}
	status, envelope := a.request(t, http.MethodPost, "/api/poker/wallet/topup", token, nil)
	if status != http.StatusOK {
		t.Fatalf("topup status %d: %s", status, envelope)
	}
	if got := a.wallet(t, token); got != before+pokerapp.BonusChips {
		t.Fatalf("bankroll = %d, want %d", got, before+pokerapp.BonusChips)
	}
	status, envelope = a.request(t, http.MethodPost, "/api/poker/wallet/topup", token, nil)
	scenarioError(t, status, http.StatusConflict, envelope, "bonus_not_ready")
	if got := a.wallet(t, token); got != before+pokerapp.BonusChips {
		t.Fatalf("rejected topup changed bankroll to %d", got)
	}
}

func TestScenario20009WebSocketReceivesTableUpdates(t *testing.T) {
	a := newScenarioAPI(t)
	owner := a.login(t, 42)
	guest := a.login(t, 43)
	code := a.create(t, owner)
	a.request(t, http.MethodPost, "/api/tables/"+code+"/join", owner, map[string]int{"buy_in": 200})
	a.request(t, http.MethodPost, "/api/tables/"+code+"/join", guest, map[string]int{"buy_in": 200})
	wsURL := "ws" + strings.TrimPrefix(a.server.URL, "http") + "/api/ws/tables/" + code + "?token=" + url.QueryEscape(guest)
	conn, response, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var initial wsMsg
	if err := conn.ReadJSON(&initial); err != nil || initial.Type != "table_state" {
		t.Fatalf("initial websocket state: %+v, %v", initial, err)
	}
	status, envelope := a.request(t, http.MethodPost, "/api/tables/"+code+"/start", owner, nil)
	if status != http.StatusOK {
		t.Fatalf("start status %d: %s", status, envelope)
	}
	var updated wsMsg
	if err := conn.ReadJSON(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Type != "table_state" || updated.Table["phase"] != "preflop" || updated.Code != code {
		t.Fatalf("unexpected websocket update: %+v", updated)
	}
}

func TestScenario20014UnknownTableUsesErrorEnvelope(t *testing.T) {
	a := newScenarioAPI(t)
	token := a.login(t, 42)
	status, envelope := a.request(t, http.MethodGet, "/api/tables/UNKNOWN", token, nil)
	scenarioError(t, status, http.StatusNotFound, envelope, "table_not_found")
}

func TestScenario20015HealthCheck(t *testing.T) {
	a := newScenarioAPI(t)
	status, envelope := a.request(t, http.MethodGet, "/healthz", "", nil)
	if status != http.StatusOK || string(envelope["status"]) != `"ok"` {
		t.Fatalf("health response %d: %s", status, envelope)
	}
}
