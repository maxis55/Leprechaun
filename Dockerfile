# syntax=docker/dockerfile:1

# --- build stage ---
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache module downloads in a layer of their own.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Static binary so the runtime image can be minimal.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/leprechaun .

# --- runtime stage ---
FROM alpine:3
# HTTPS to discord/silpo/varus/google needs the cert bundle.
RUN apk add --no-cache ca-certificates && adduser -D -H app
USER app
COPY --from=build /out/leprechaun /usr/local/bin/leprechaun
ENTRYPOINT ["/usr/local/bin/leprechaun"]
