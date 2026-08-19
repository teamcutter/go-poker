package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	authapp "github.com/teamcutter/go-poker/internal/application/auth"

	"github.com/teamcutter/go-poker/internal/infrastructure/config"
	"github.com/teamcutter/go-poker/internal/infrastructure/postgres"
	"github.com/teamcutter/go-poker/internal/infrastructure/redis"
	sess "github.com/teamcutter/go-poker/internal/infrastructure/session"
	tginfra "github.com/teamcutter/go-poker/internal/infrastructure/telegram"

	pokerapp "github.com/teamcutter/go-poker/internal/application/poker"

	httptransport "github.com/teamcutter/go-poker/internal/transport/http"
	"github.com/teamcutter/go-poker/migrations"
)

func main() {
	log, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()

	if err := godotenv.Load(); err != nil {
		log.Warn("no .env file found, using environment")
	}

	if err := run(log); err != nil {
		log.Fatal("server exited with error", zap.Error(err))
	}
}

func run(log *zap.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()

	db, err := postgres.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := postgres.Migrate(ctx, db.Raw(), migrations.Files); err != nil {
		return err
	}

	redisClient, err := redis.New(ctx, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		return err
	}
	defer func() { _ = redisClient.Close() }()

	users := postgres.NewUserRepository(db)

	telegramValidator := tginfra.NewValidator(cfg.TelegramBotToken, cfg.TelegramAuthTTL)
	sessionManager := sess.NewManager(cfg.SessionSecret, cfg.PublicIDSecret, cfg.JWTIssuer, cfg.SessionTTL)
	limiter := redis.NewRateLimiter(redisClient)

	now := time.Now
	tx := db

	authSvc := authapp.NewService(users, telegramValidator, sessionManager, tx, now)
	authHandler := httptransport.NewAuthHandler(authSvc, cfg.SessionTTL)

	pokerSvc := pokerapp.NewService(nil)
	reaperCtx, stopReaper := context.WithCancel(ctx)
	defer stopReaper()
	go pokerSvc.RunReaper(reaperCtx)
	pokerHandler := httptransport.NewPokerHandler(pokerSvc)
	wsHandler := httptransport.NewWSHandler(pokerSvc, sessionManager, log)

	router := httptransport.NewRouter(
		log,
		sessionManager,
		authHandler,
		pokerHandler,
		wsHandler,
		limiter,
		cfg.CORSOrigins,
		30,
		60*time.Second,
	)
	router.Register()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-stop:
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
