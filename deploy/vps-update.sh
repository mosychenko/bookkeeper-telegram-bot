#!/usr/bin/env bash
# Обновление уже установленного сервиса: git pull и запуск загруженного образа.
# Образ приложения здесь не собирается. Новый код приезжает с Mac
# скриптом deploy/push-image.sh.
# sudo bash deploy/vps-update.sh
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
  if command -v sudo >/dev/null 2>&1; then
    exec sudo -- bash "$0" "$@"
  fi
  echo "Нужен root: sudo bash deploy/vps-update.sh" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
# shellcheck source=deploy/lib.sh
source "$ROOT/deploy/lib.sh"

if [[ ! -f .env ]]; then
  echo "Нет .env. Сначала: sudo bash deploy/vps-install.sh домен email" >&2
  exit 1
fi

git pull --ff-only
require_app_image
compose up -d --no-build
echo "Обновлено. Образ приложения не пересобирался."
echo "Если менялись Go или экран, на Mac сначала: bash deploy/push-image.sh user@этот-сервер"
