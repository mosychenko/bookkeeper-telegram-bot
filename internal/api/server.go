package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/auth"
	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
	"github.com/mosychenko/bookkeeper-telegram-bot/internal/sheet"
)

type Server struct {
	Source sheet.Source
	Auth   auth.Checker
	Static string
	Log    *slog.Logger
	Now    func() time.Time
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.health)
	mux.HandleFunc("/api/budget", s.budget)
	mux.HandleFunc("/", s.static)
	return mux
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Server) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) budget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	id, err := s.Auth.Authorize(r.Header.Get("Authorization"))
	if errors.Is(err, auth.ErrForbidden) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Нет доступа."})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Сессия Telegram недействительна. Откройте мини-приложение заново.",
		})
		return
	}
	in, err := s.Source.Load(r.Context(), s.now())
	if err != nil {
		s.log().Error("бюджет", "err", err, "user", id)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось прочитать бюджет."})
		return
	}
	s.log().Info("бюджет", "user", id)
	writeJSON(w, http.StatusOK, budget.Build(in))
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	if s.Static == "" {
		http.NotFound(w, r)
		return
	}
	root, err := filepath.Abs(s.Static)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")
	full := filepath.Join(root, rel)
	abs, err := filepath.Abs(full)
	if err != nil || (abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator))) {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
		return
	}
	http.ServeFile(w, r, abs)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// DiscardLog is a quiet logger for tests.
func DiscardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
