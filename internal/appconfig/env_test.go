package appconfig

import "testing"

func TestWebAppURLDefaultsToDomain(t *testing.T) {
	t.Setenv("WEBAPP_URL", "")
	t.Setenv("DOMAIN", "budget.example.com")
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("DATA_SOURCE", "fixture")
	t.Setenv("APP_ENV", "prod")
	t.Setenv("DEV_SKIP_AUTH", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WebAppURL != "https://budget.example.com" {
		t.Fatalf("WEBAPP_URL %q", cfg.WebAppURL)
	}

	t.Setenv("WEBAPP_URL", "https://other.example.com/app")
	t.Setenv("DOMAIN", "budget.example.com")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WebAppURL != "https://other.example.com/app" {
		t.Fatalf("явный WEBAPP_URL не должен затираться: %q", cfg.WebAppURL)
	}
}
