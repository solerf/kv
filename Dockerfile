FROM node:22-alpine AS assets
WORKDIR /src
COPY package.json pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY scripts/ scripts/
RUN node scripts/copy-assets.js

FROM golang:1.26-alpine AS build
RUN go install github.com/a-h/templ/cmd/templ@latest
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=assets /src/static/css/bootstrap.min.css static/css/bootstrap.min.css
COPY --from=assets /src/static/css/neobrutalismcss.css static/css/neobrutalismcss.css
COPY --from=assets /src/static/js/ static/js/
RUN templ generate
RUN CGO_ENABLED=0 go build -o /kv .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /kv .
COPY --from=build /src/static/ static/
EXPOSE 8888
ENTRYPOINT ["./kv"]
