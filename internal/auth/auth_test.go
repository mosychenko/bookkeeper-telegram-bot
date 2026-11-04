package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const testToken = "123456:TEST_TOKEN"

func TestAllowlist(t *testing.T) {
	a, err := ParseAllowlist(" 111, 222 ,")
	if err != nil {
		t.Fatal(err)
	}
	if !a.Allows(111) || !a.Allows(222) {
		t.Fatal("свои id должны проходить")
	}
	if a.Allows(333) || a.Allows(0) {
		t.Fatal("чужой id прошёл")
	}
	empty, err := ParseAllowlist("  ")
	if err != nil {
		t.Fatal(err)
	}
	if empty.Allows(111) {
		t.Fatal("пустой список никого не пускает")
	}
	if _, err := ParseAllowlist("12,abc"); err == nil {
		t.Fatal("мусор в списке должен быть ошибкой")
	}
}

func TestInitDataValid(t *testing.T) {
	raw := SignedInitData(testToken, 111, "Maxim", time.Now().Add(-time.Hour))
	c := Checker{Token: testToken, MaxAge: MaxAge, Allow: Allowlist{111: {}}}
	id, err := c.Authorize("tma " + raw)
	if err != nil || id != 111 {
		t.Fatalf("валидная подпись: id=%d err=%v", id, err)
	}
	// Scheme case and extra space are accepted.
	if _, err := c.Authorize("TMA  " + raw); err != nil {
		t.Fatal(err)
	}
}

func TestInitDataForged(t *testing.T) {
	raw := SignedInitData(testToken, 111, "Maxim", time.Now())
	c := Checker{Token: testToken, MaxAge: MaxAge, Allow: Allowlist{111: {}}}

	// Flip the signature without touching auth_date, so this is a bad
	// signature and not an expiry.
	forged := strings.Replace(raw, "hash=", "hash=ff", 1)
	if _, err := c.Authorize("tma " + forged); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("поддельная подпись: %v", err)
	}

	other := SignedInitData("999:OTHER", 111, "Maxim", time.Now())
	if _, err := c.Authorize("tma " + other); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("чужой токен: %v", err)
	}

	// A valid signature whose user id was edited must fail too.
	const needle = "%3A111%2C"
	if !strings.Contains(raw, needle) {
		t.Fatalf("не нашли id в initData: %s", raw)
	}
	tampered := strings.Replace(raw, needle, "%3A222%2C", 1)
	if _, err := c.Authorize("tma " + tampered); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("подменённый user: %v", err)
	}
}

func TestInitDataExpired(t *testing.T) {
	raw := SignedInitData(testToken, 111, "Maxim", time.Now().Add(-25*time.Hour))
	c := Checker{Token: testToken, MaxAge: MaxAge, Allow: Allowlist{111: {}}}
	if _, err := c.Authorize("tma " + raw); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("просроченная сессия: %v", err)
	}
	// Just inside the window still works.
	fresh := SignedInitData(testToken, 111, "Maxim", time.Now().Add(-23*time.Hour))
	if _, err := c.Authorize("tma " + fresh); err != nil {
		t.Fatal(err)
	}
}

func TestAllowlistRejectsValidSignature(t *testing.T) {
	raw := SignedInitData(testToken, 999, "Stranger", time.Now())
	c := Checker{Token: testToken, MaxAge: MaxAge, Allow: Allowlist{111: {}, 222: {}}}
	_, err := c.Authorize("Authorization: tma " + raw)
	// The header must be exactly "tma <data>", not a longer scheme.
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("лишний префикс: %v", err)
	}
	_, err = c.Authorize("tma " + raw)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("чужой пользователь: %v", err)
	}
}

func TestDevSkipOnlyExactToken(t *testing.T) {
	c := Checker{DevSkip: true, Allow: Allowlist{}}
	if _, err := c.Authorize("tma dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Authorize("tma development"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("dev-режим не должен принимать произвольную строку: %v", err)
	}
	off := Checker{DevSkip: false}
	if _, err := off.Authorize("tma dev"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("без DEV_SKIP_AUTH строка dev запрещена")
	}
}

func TestMissingHeader(t *testing.T) {
	c := Checker{Token: testToken, Allow: Allowlist{1: {}}}
	for _, h := range []string{"", "Bearer abc", "tma", "tma "} {
		if _, err := c.Authorize(h); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("%q: %v", h, err)
		}
	}
}
