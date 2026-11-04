# Общие функции скриптов VPS. Подключается через source, сама ничего не запускает.

env_val() {
  local line
  line="$(grep -E "^${1}=" .env | tail -n 1 || true)"
  line="${line#*=}"
  line="${line%$'\r'}"
  line="${line#\"}"
  line="${line%\"}"
  line="${line#\'}"
  line="${line%\'}"
  printf '%s' "$line"
}

set_env() {
  local key="$1" value="$2" esc
  if grep -q "^${key}=" .env; then
    esc="$(printf '%s' "$value" | sed 's/[&|]/\\&/g')"
    sed -i "s|^${key}=.*|${key}=${esc}|" .env
  else
    printf '%s=%s\n' "$key" "$value" >> .env
  fi
}

compose() {
  local -a args=(-f docker-compose.yml -f docker-compose.prod.yml)
  if [[ "$(env_val DATA_SOURCE)" == "sheets" ]]; then
    args+=(-f docker-compose.sheets.yml)
  fi
  docker compose "${args[@]}" "$@"
}

install_docker() {
  if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
    apt-get update
    apt-get install -y ca-certificates curl git
  fi
  if ! command -v git >/dev/null 2>&1; then
    apt-get update
    apt-get install -y git
  fi
  if ! command -v docker >/dev/null 2>&1; then
    curl -fsSL https://get.docker.com | sh
  fi
  if ! docker compose version >/dev/null 2>&1; then
    apt-get update
    apt-get install -y docker-compose-plugin
  fi
  if ! docker compose version >/dev/null 2>&1; then
    echo "Не удалось поставить Docker Compose." >&2
    exit 1
  fi
}

port_busy() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    if ss -ltn | awk '{print $4}' | grep -Eq ":${port}$"; then
      return 0
    fi
    return 1
  fi
  if command -v lsof >/dev/null 2>&1; then
    if lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1; then
      return 0
    fi
    return 1
  fi
  echo "Нет ss и lsof, не могу проверить порт ${port}." >&2
  exit 1
}

check_ports() {
  if compose ps --status running --services 2>/dev/null | grep -qx caddy; then
    echo "Caddy этого проекта уже слушает 80/443, проверку портов пропускаю."
    return 0
  fi
  local port
  for port in 80 443; do
    if port_busy "$port"; then
      echo "Порт ${port} занят. Для сертификата нужны свободные 80 и 443." >&2
      if command -v ss >/dev/null 2>&1; then
        ss -ltnp | grep -E ":${port}\\b" || true
      fi
      exit 1
    fi
  done
}
