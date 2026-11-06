#!/usr/bin/env bash
# Первая установка на чистый Ubuntu или Debian.
# sudo bash deploy/vps-install.sh budget.example.com you@example.com
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
  if command -v sudo >/dev/null 2>&1; then
    exec sudo -- bash "$0" "$@"
  fi
  echo "Нужен root: sudo bash deploy/vps-install.sh домен email" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
# shellcheck source=deploy/lib.sh
source "$ROOT/deploy/lib.sh"

DOMAIN_ARG="${1:-}"
EMAIL_ARG="${2:-}"

if [[ ! -f .env ]]; then
  cp .env.example .env
  echo "Создан .env из .env.example."
fi

if [[ -n "$DOMAIN_ARG" ]]; then
  set_env DOMAIN "$DOMAIN_ARG"
fi
if [[ -n "$EMAIL_ARG" ]]; then
  set_env ACME_EMAIL "$EMAIL_ARG"
fi

domain="$(env_val DOMAIN)"
email="$(env_val ACME_EMAIL)"

missing=()
[[ -z "$domain" ]] && missing+=("DOMAIN")
[[ -z "$email" ]] && missing+=("ACME_EMAIL")
if ((${#missing[@]})); then
  echo "Пустые обязательные переменные: ${missing[*]}" >&2
  echo "Запуск: sudo bash deploy/vps-install.sh example.com you@example.com" >&2
  echo "Либо впишите их в .env и запустите скрипт снова." >&2
  exit 1
fi

set_env WEBAPP_URL "https://${domain}"
set_env HTTP_PUBLISH "127.0.0.1:8080"
echo "WEBAPP_URL=https://${domain}"

warn=()
[[ -z "$(env_val TELEGRAM_BOT_TOKEN)" ]] && warn+=("TELEGRAM_BOT_TOKEN")
[[ -z "$(env_val TELEGRAM_ALLOWED_USER_IDS)" ]] && warn+=("TELEGRAM_ALLOWED_USER_IDS")
if ((${#warn[@]})); then
  echo "Пока пусто: ${warn[*]}"
  echo "Без них бот не ответит и в мини-приложение не пустит. Сайт для первой проверки всё равно поднимется."
fi

source_kind="$(env_val DATA_SOURCE)"
if [[ -z "$source_kind" ]]; then
  source_kind="fixture"
  set_env DATA_SOURCE fixture
fi

if [[ "$source_kind" == "sheets" ]]; then
  sa="$(env_val GOOGLE_SA_FILE)"
  if [[ -z "$sa" || ! -f "$sa" ]]; then
    echo "DATA_SOURCE=sheets, но GOOGLE_SA_FILE пуст или файла нет." >&2
    echo "Положите JSON ключ вне репозитория, например GOOGLE_SA_FILE=/root/dengi-sa.json" >&2
    exit 1
  fi
  sa_real="$(realpath "$sa")"
  case "$sa_real" in
    "$ROOT"/*)
      echo "Ключ ${sa_real} лежит внутри репозитория. Перенесите его наружу." >&2
      exit 1
      ;;
  esac
  chmod a+r "$sa_real"
  echo "Ключ смонтируется в контейнер из ${sa_real}."
elif [[ "$source_kind" != "fixture" ]]; then
  echo "DATA_SOURCE должен быть fixture или sheets, сейчас ${source_kind}." >&2
  exit 1
else
  echo "Режим фикстуры: первая проверка без таблицы и без ключа Google."
fi

install_docker
check_ports
require_app_image
compose up -d --no-build

echo "Готово. Проверка: curl -fsS https://${domain}/api/health"
echo "В браузере без Telegram будет просьба открыть мини-приложение из бота. Это нормально для первой проверки."
