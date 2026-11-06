package deploy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(wd)
}

func TestVPSScriptsDoNotBuild(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{"deploy/vps-install.sh", "deploy/vps-update.sh"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if !strings.Contains(text, "compose up -d --no-build") {
			t.Fatalf("%s должен запускать уже загруженный образ", name)
		}
		rest := strings.ReplaceAll(text, "--no-build", "")
		if strings.Contains(rest, "--build") || strings.Contains(rest, "compose build") {
			t.Fatalf("%s всё ещё собирает образ на сервере", name)
		}
		cmd := exec.Command("bash", "-n", filepath.Join(root, name))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", name, err, out)
		}
	}
}

func TestPushImageUsage(t *testing.T) {
	script := filepath.Join(repoRoot(t), "deploy/push-image.sh")
	help := exec.Command("bash", script, "--help")
	out, err := help.CombinedOutput()
	if err != nil {
		t.Fatalf("help: %v\n%s", err, out)
	}
	text := string(out)
	for _, want := range []string{"user@host", "linux/amd64", "dengi:latest", "docker save", "gunzip | docker load", "vps-install.sh", "vps-update.sh"} {
		if !strings.Contains(text, want) {
			t.Fatalf("справка без %q:\n%s", want, text)
		}
	}

	missing := exec.Command("bash", script)
	out, err = missing.CombinedOutput()
	if err == nil {
		t.Fatal("без адреса SSH скрипт должен остановиться")
	}
	if !strings.Contains(string(out), "user@host") {
		t.Fatalf("сообщение: %s", out)
	}

	bad := exec.Command("bash", script, "user@vps", "--platform")
	out, err = bad.CombinedOutput()
	if err == nil {
		t.Fatal("пустой --platform должен остановиться")
	}
	if !strings.Contains(string(out), "linux/amd64") {
		t.Fatalf("сообщение: %s", out)
	}
}

func TestMissingImageMessage(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DATA_SOURCE=fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	stub := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", `set -euo pipefail; source "$ROOT/deploy/lib.sh"; require_app_image`)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin", "ROOT="+root, "APP_IMAGE=dengi:latest")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("без образа скрипт должен остановиться, вывод: %s", out)
	}
	text := string(out)
	for _, want := range []string{"На сервере нет образа dengi:latest", "deploy/push-image.sh", "Сборка npm и Go на VPS не выполняется"} {
		if !strings.Contains(text, want) {
			t.Fatalf("ждали %q в:\n%s", want, text)
		}
	}

	custom := exec.Command("bash", "-c", `set -euo pipefail; source "$ROOT/deploy/lib.sh"; require_app_image`)
	custom.Dir = dir
	custom.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin", "ROOT="+root, "APP_IMAGE=family:book")
	out, err = custom.CombinedOutput()
	if err == nil {
		t.Fatal("свой тег тоже должен отсутствовать")
	}
	if !strings.Contains(string(out), "family:book") {
		t.Fatalf("тег не попал в сообщение: %s", out)
	}
}

func TestImagePresent(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_IMAGE=from-env:1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	stub := "#!/bin/sh\nif [ \"$1\" = image ] && [ \"$2\" = inspect ] && [ \"$3\" = from-env:1 ]; then exit 0; fi\necho unexpected: \"$@\" >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	env := make([]string, 0, len(os.Environ())+2)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "APP_IMAGE=") {
			continue
		}
		env = append(env, e)
	}
	env = append(env, "PATH="+bin+":/usr/bin:/bin", "ROOT="+root)
	cmd := exec.Command("bash", "-c", `set -euo pipefail; source "$ROOT/deploy/lib.sh"; require_app_image`)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("образ из .env не принят: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "from-env:1") {
		t.Fatalf("вывод: %s", out)
	}
}

func TestComposeKeepsLocalBuild(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker не установлен")
	}
	root := repoRoot(t)
	cmd := exec.Command("docker", "compose", "-f", "docker-compose.yml", "config")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "APP_IMAGE=dengi:latest")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compose config: %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "image: dengi:latest") {
		t.Fatalf("нет тега образа:\n%s", text)
	}
	if !strings.Contains(text, "dockerfile: Dockerfile") && !strings.Contains(text, "build:") {
		t.Fatalf("локальная сборка пропала из compose:\n%s", text)
	}

	prod := exec.Command("docker", "compose", "-f", "docker-compose.yml", "-f", "docker-compose.prod.yml", "config")
	prod.Dir = root
	prod.Env = append(os.Environ(), "DOMAIN=dozp.ru", "ACME_EMAIL=you@example.com", "APP_IMAGE=dengi:latest")
	out, err = prod.CombinedOutput()
	if err != nil {
		t.Fatalf("prod config: %v\n%s", err, out)
	}
	text = string(out)
	if !strings.Contains(text, "pull_policy: never") {
		t.Fatalf("боевой слой может потянуть чужой образ:\n%s", text)
	}
	if !strings.Contains(text, "/app/config") {
		t.Fatalf("config с диска сервера не смонтирован:\n%s", text)
	}
}
