#!/usr/bin/env bash
# Сборка образа на Mac и передача на VPS. На сервере npm и Go не запускаются.
# bash deploy/push-image.sh user@vps
# bash deploy/push-image.sh user@vps --platform linux/amd64 --image dengi:latest
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

usage() {
  cat <<'EOF'
Использование: bash deploy/push-image.sh user@host [--platform linux/amd64] [--image dengi:latest]

Собирает образ под архитектуру VPS и передаёт его:
  docker save ОБРАЗ | gzip | ssh user@host 'gunzip | docker load'

По умолчанию платформа linux/amd64: образ Apple Silicon на обычном Debian не запустится.
Имя образа по умолчанию dengi:latest. Те же значения задают переменные PLATFORM и APP_IMAGE.
На сервере уже должен быть Docker, приём выполняет docker load.
После загрузки на VPS: sudo bash deploy/vps-install.sh dozp.ru you@example.com
или sudo bash deploy/vps-update.sh
EOF
}

TARGET=""
PLATFORM="${PLATFORM:-linux/amd64}"
IMAGE="${APP_IMAGE:-dengi:latest}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --platform)
      if [[ $# -lt 2 ]]; then
        echo "После --platform нужна архитектура, например linux/amd64" >&2
        exit 1
      fi
      PLATFORM="$2"
      shift 2
      ;;
    --image)
      if [[ $# -lt 2 ]]; then
        echo "После --image нужно имя, например dengi:latest" >&2
        exit 1
      fi
      IMAGE="$2"
      shift 2
      ;;
    --platform=*)
      PLATFORM="${1#*=}"
      shift
      ;;
    --image=*)
      IMAGE="${1#*=}"
      shift
      ;;
    -*)
      echo "Неизвестный флаг: $1" >&2
      usage >&2
      exit 1
      ;;
    *)
      if [[ -n "$TARGET" ]]; then
        echo "Лишний аргумент: $1" >&2
        usage >&2
        exit 1
      fi
      TARGET="$1"
      shift
      ;;
  esac
done

if [[ -z "$TARGET" ]]; then
  echo "Нужен адрес SSH: user@host" >&2
  usage >&2
  exit 1
fi
if [[ "$TARGET" != *@* ]]; then
  echo "Адрес SSH выглядит как user@host, сейчас: ${TARGET}" >&2
  exit 1
fi
if [[ -z "$PLATFORM" || -z "$IMAGE" ]]; then
  echo "Платформа и имя образа не должны быть пустыми." >&2
  exit 1
fi
if ! command -v docker >/dev/null 2>&1; then
  echo "На этой машине нет docker." >&2
  exit 1
fi
if ! command -v gzip >/dev/null 2>&1 || ! command -v ssh >/dev/null 2>&1; then
  echo "Нужны gzip и ssh." >&2
  exit 1
fi

echo "Сборка ${IMAGE} для ${PLATFORM}."
docker build --platform "$PLATFORM" -t "$IMAGE" .

echo "Передача ${IMAGE} на ${TARGET}."
docker save "$IMAGE" | gzip | ssh "$TARGET" 'gunzip | docker load'

echo "Образ ${IMAGE} загружен на ${TARGET}."
echo "Дальше на VPS, из каталога репозитория:"
echo "  первая установка: sudo bash deploy/vps-install.sh dozp.ru you@example.com"
echo "  обновление:       sudo bash deploy/vps-update.sh"
