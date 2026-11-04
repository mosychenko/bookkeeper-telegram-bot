package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/api"
	"github.com/mosychenko/bookkeeper-telegram-bot/internal/appconfig"
	"github.com/mosychenko/bookkeeper-telegram-bot/internal/auth"
	"github.com/mosychenko/bookkeeper-telegram-bot/internal/bot"
	"github.com/mosychenko/bookkeeper-telegram-bot/internal/sheet"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg, err := appconfig.Load()
	if err != nil {
		log.Error("конфигурация", "err", err)
		os.Exit(1)
	}
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Error("часовой пояс", "err", err)
		os.Exit(1)
	}

	src, err := buildSource(context.Background(), cfg)
	if err != nil {
		log.Error("источник данных", "err", err)
		os.Exit(1)
	}
	src = sheet.Cache(src, cfg.CacheTTL)

	handler := (&api.Server{
		Source: src,
		Auth: auth.Checker{
			Token:   cfg.BotToken,
			MaxAge:  auth.MaxAge,
			Allow:   cfg.Allow,
			DevSkip: cfg.DevSkipAuth,
		},
		Static: cfg.StaticDir,
		Log:    log,
		Now:    func() time.Time { return time.Now().In(loc) },
	}).Handler()

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.BotToken != "" {
		go func() {
			log.Info("бот запущен", "webapp", cfg.WebAppURL)
			if err := bot.Run(ctx, cfg.BotToken, cfg.WebAppURL, cfg.Allow); err != nil && ctx.Err() == nil {
				log.Error("бот", "err", err)
			}
		}()
	} else {
		log.Info("TELEGRAM_BOT_TOKEN не задан, бот не запущен")
	}

	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shut)
	}()

	log.Info("слушаю", "addr", cfg.Addr, "source", cfg.DataSource, "env", cfg.Env)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("http", "err", err)
		os.Exit(1)
	}
}

func buildSource(ctx context.Context, cfg appconfig.Config) (sheet.Source, error) {
	switch cfg.DataSource {
	case "sheets":
		bf, err := sheet.LoadBudget(cfg.BudgetPath)
		if err != nil {
			return nil, err
		}
		mp, err := sheet.LoadMapping(cfg.SheetsPath)
		if err != nil {
			return nil, err
		}
		values, err := sheet.NewGoogleValues(ctx, cfg.GoogleJSON)
		if err != nil {
			return nil, err
		}
		return &sheet.GoogleSource{
			SpreadsheetID: cfg.Spreadsheet,
			Values:        values,
			Budget:        bf,
			Map:           mp,
		}, nil
	default:
		in, err := sheet.LoadFixture(cfg.BudgetPath, cfg.FixturePath)
		if err != nil {
			return nil, err
		}
		return sheet.FixtureSource{Input: in}, nil
	}
}
