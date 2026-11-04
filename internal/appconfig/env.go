package appconfig

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/auth"
)

// Config is process configuration. Secrets come from the environment,
// never from the repo.
type Config struct {
	Addr        string
	Env         string
	DevSkipAuth bool
	BotToken    string
	WebAppURL   string
	Allow       auth.Allowlist
	DataSource  string
	Spreadsheet string
	GoogleJSON  []byte
	BudgetPath  string
	SheetsPath  string
	FixturePath string
	CacheTTL    time.Duration
	StaticDir   string
}

func Load() (Config, error) {
	env := getenv("APP_ENV", "prod")
	devSkip := truthy(os.Getenv("DEV_SKIP_AUTH"))
	if devSkip && env != "dev" {
		return Config{}, fmt.Errorf("DEV_SKIP_AUTH можно включать только при APP_ENV=dev")
	}
	allow, err := auth.ParseAllowlist(os.Getenv("TELEGRAM_ALLOWED_USER_IDS"))
	if err != nil {
		return Config{}, err
	}
	source := getenv("DATA_SOURCE", "fixture")
	if source != "fixture" && source != "sheets" {
		return Config{}, fmt.Errorf("DATA_SOURCE должен быть fixture или sheets, сейчас %q", source)
	}
	ttl, err := time.ParseDuration(getenv("CACHE_TTL", "3m"))
	if err != nil {
		return Config{}, fmt.Errorf("CACHE_TTL: %w", err)
	}
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	webApp := strings.TrimSpace(os.Getenv("WEBAPP_URL"))
	if token != "" {
		u, err := url.Parse(webApp)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return Config{}, fmt.Errorf("WEBAPP_URL должен быть https-адресом мини-приложения")
		}
		if len(allow) == 0 {
			return Config{}, fmt.Errorf("с токеном бота нужен TELEGRAM_ALLOWED_USER_IDS")
		}
	}
	var creds []byte
	if source == "sheets" {
		creds, err = googleCreds()
		if err != nil {
			return Config{}, err
		}
	}
	sheetID := getenv("BUDGET_SPREADSHEET_ID", "1n_Yp3gHkX-jNZGD5Mn4IYxPtAixZhF6xyXNVurUZrBI")
	if source == "sheets" && strings.TrimSpace(sheetID) == "" {
		return Config{}, fmt.Errorf("нужен BUDGET_SPREADSHEET_ID")
	}
	return Config{
		Addr:        getenv("ADDR", ":8080"),
		Env:         env,
		DevSkipAuth: devSkip,
		BotToken:    token,
		WebAppURL:   webApp,
		Allow:       allow,
		DataSource:  source,
		Spreadsheet: sheetID,
		GoogleJSON:  creds,
		BudgetPath:  getenv("BUDGET_CONFIG", "config/budget.yaml"),
		SheetsPath:  getenv("SHEETS_CONFIG", "config/sheets.yaml"),
		FixturePath: getenv("FIXTURE_CONFIG", "config/fixture.yaml"),
		CacheTTL:    ttl,
		StaticDir:   getenv("STATIC_DIR", "web/dist"),
	}, nil
}

func googleCreds() ([]byte, error) {
	if raw := strings.TrimSpace(os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON")); raw != "" {
		if strings.HasPrefix(raw, "{") {
			return []byte(raw), nil
		}
		b, err := os.ReadFile(raw)
		if err != nil {
			return nil, fmt.Errorf("GOOGLE_SERVICE_ACCOUNT_JSON: %w", err)
		}
		return b, nil
	}
	if path := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("GOOGLE_APPLICATION_CREDENTIALS: %w", err)
		}
		return b, nil
	}
	return nil, fmt.Errorf("для DATA_SOURCE=sheets нужен GOOGLE_SERVICE_ACCOUNT_JSON или GOOGLE_APPLICATION_CREDENTIALS")
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
