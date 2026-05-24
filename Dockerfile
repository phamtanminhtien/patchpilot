FROM node:26-alpine AS web-build

WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:1.26-alpine AS api-build

RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/patchpilot ./cmd/patchpilot

FROM alpine:3.22

RUN apk add --no-cache ca-certificates git
WORKDIR /app
COPY --from=api-build /out/patchpilot /usr/local/bin/patchpilot
COPY --from=web-build /src/web/dist /app/web/dist
RUN mkdir -p /data /workspace

ENV PATCHPILOT_ADDR=0.0.0.0:8080
ENV PATCHPILOT_ALLOWED_ROOTS=/workspace
ENV PATCHPILOT_DATA_DIR=/data
ENV PATCHPILOT_STATIC_DIR=/app/web/dist
ENV PATCHPILOT_LOG_FORMAT=json

EXPOSE 8080
CMD ["patchpilot"]
