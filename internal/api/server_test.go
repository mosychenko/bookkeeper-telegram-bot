package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/auth"
	"github.com/mosychenko/bookkeeper-telegram-bot/internal/sheet"
)

func TestBudgetAuthAndNumbers(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	in, err := sheet.LoadFixture(filepath.Join(root, "config", "budget.yaml"), filepath.Join(root, "config", "fixture.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	const token = "123456:TEST_TOKEN"
	allow := auth.Allowlist{111: {}, 222: {}}
	h := (&Server{
		Source: sheet.FixtureSource{Input: in},
		Auth:   auth.Checker{Token: token, MaxAge: auth.MaxAge, Allow: allow},
		Log:    DiscardLog(),
		Now:    func() time.Time { return time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC) },
	}).Handler()

	t.Run("no header", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/budget", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("code %d", rec.Code)
		}
	})

	t.Run("forged", func(t *testing.T) {
		raw := auth.SignedInitData(token, 111, "Maxim", time.Now())
		raw = raw[:len(raw)-4] + "ffff"
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/budget", nil)
		req.Header.Set("Authorization", "tma "+raw)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("expired", func(t *testing.T) {
		raw := auth.SignedInitData(token, 111, "Maxim", time.Now().Add(-25*time.Hour))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/budget", nil)
		req.Header.Set("Authorization", "tma "+raw)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("code %d", rec.Code)
		}
	})

	t.Run("stranger", func(t *testing.T) {
		raw := auth.SignedInitData(token, 999, "Stranger", time.Now())
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/budget", nil)
		req.Header.Set("Authorization", "tma "+raw)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("family", func(t *testing.T) {
		raw := auth.SignedInitData(token, 222, "Zhenya", time.Now())
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/budget", nil)
		req.Header.Set("Authorization", "tma "+raw)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
		}
		var view struct {
			Free       int64  `json:"free"`
			PerDay     int64  `json:"perDay"`
			StatusText string `json:"statusText"`
			LastPace   int64  `json:"lastPace"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
			t.Fatal(err)
		}
		if view.Free != 46234 || view.PerDay != 3082 || view.StatusText != "Впритык" || view.LastPace != 3891 {
			t.Fatalf("%+v", view)
		}
	})
}

func TestHealthOpen(t *testing.T) {
	h := (&Server{Log: DiscardLog()}).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}
