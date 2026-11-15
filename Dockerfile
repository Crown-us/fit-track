# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO is off: modernc.org/sqlite is pure Go, so the binary is fully static.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && mkdir /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
# Copied with nonroot ownership so a named volume mounted here is writable.
COPY --from=build --chown=nonroot:nonroot /out/data /data
ENV DATABASE_PATH=/data/fittrack.db \
    HTTP_ADDR=:8080
EXPOSE 8080
VOLUME ["/data"]
USER nonroot:nonroot
ENTRYPOINT ["/api"]
