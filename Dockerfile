# syntax=docker/dockerfile:1

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 app
WORKDIR /app
COPY --from=build /out/server /app/server
COPY config /app/config
COPY --from=web /src/web/dist /app/web/dist
USER app
ENV STATIC_DIR=/app/web/dist \
    BUDGET_CONFIG=/app/config/budget.yaml \
    SHEETS_CONFIG=/app/config/sheets.yaml \
    FIXTURE_CONFIG=/app/config/fixture.yaml \
    ADDR=:8080 \
    DATA_SOURCE=fixture \
    APP_ENV=prod \
    CACHE_TTL=3m \
    TZ=Europe/Moscow
EXPOSE 8080
ENTRYPOINT ["/app/server"]
