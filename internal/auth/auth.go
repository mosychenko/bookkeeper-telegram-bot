package auth

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

const MaxAge = 24 * time.Hour

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

// Allowlist is the set of Telegram user ids that may open the screen.
// An empty list allows nobody.
type Allowlist map[int64]struct{}

func ParseAllowlist(raw string) (Allowlist, error) {
	out := Allowlist{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out, nil
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, errors.New("TELEGRAM_ALLOWED_USER_IDS: «" + part + "» не похоже на id")
		}
		out[id] = struct{}{}
	}
	return out, nil
}

func (a Allowlist) Allows(id int64) bool {
	if len(a) == 0 || id <= 0 {
		return false
	}
	_, ok := a[id]
	return ok
}

// Checker validates Mini App initData and the family allowlist.
type Checker struct {
	Token   string
	MaxAge  time.Duration
	Allow   Allowlist
	DevSkip bool
}

// Authorize reads an "Authorization: tma <initData>" header.
// Invalid or expired initData is ErrUnauthorized. A valid user outside
// the allowlist is ErrForbidden.
func (c Checker) Authorize(header string) (int64, error) {
	raw, err := initDataFromHeader(header)
	if err != nil {
		return 0, ErrUnauthorized
	}
	if c.DevSkip && raw == "dev" {
		return 0, nil
	}
	age := c.MaxAge
	if age <= 0 {
		age = MaxAge
	}
	if c.Token == "" {
		return 0, ErrUnauthorized
	}
	if err := initdata.Validate(raw, c.Token, age); err != nil {
		return 0, ErrUnauthorized
	}
	data, err := initdata.Parse(raw)
	if err != nil || data.User.ID <= 0 {
		return 0, ErrUnauthorized
	}
	if !c.Allow.Allows(data.User.ID) {
		return 0, ErrForbidden
	}
	return data.User.ID, nil
}

func initDataFromHeader(header string) (string, error) {
	header = strings.TrimSpace(header)
	if len(header) < 4 || !strings.EqualFold(header[:3], "tma") {
		return "", ErrUnauthorized
	}
	rest := header[3:]
	if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return "", ErrUnauthorized
	}
	raw := strings.TrimSpace(rest)
	if raw == "" {
		return "", ErrUnauthorized
	}
	return raw, nil
}

// SignedInitData builds a WebApp initData query string. Tests and local
// tooling use it; production initData comes from Telegram.
func SignedInitData(token string, userID int64, firstName string, at time.Time) string {
	user := `{"id":` + strconv.FormatInt(userID, 10) + `,"first_name":` + strconv.Quote(firstName) + `}`
	hash := initdata.Sign(map[string]string{"user": user}, token, at)
	q := url.Values{}
	q.Set("user", user)
	q.Set("auth_date", strconv.FormatInt(at.Unix(), 10))
	q.Set("hash", hash)
	return q.Encode()
}
