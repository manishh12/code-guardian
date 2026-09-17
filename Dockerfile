# syntax=docker/dockerfile:1
FROM node:20-alpine AS web-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -o /out/server ./cmd/server && CGO_ENABLED=0 go build -o /out/seed ./cmd/seed

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/seed /app/seed
COPY guidelines /app/guidelines
COPY --from=web-build /src/web/dist /app/web/dist
ENV PORT=8787
ENV STATIC_DIR=/app/web/dist
EXPOSE 8787
HEALTHCHECK --interval=15s --timeout=5s --retries=5 CMD wget -qO- http://127.0.0.1:8787/health || exit 1
CMD ["/app/server"]
