# The API, cmd/migrate (Fly's release command) and cmd/payouts (the operator's payout tool) in one
# image. Build from apps/api; go.work only uses this module.
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/api ./cmd/migrate ./cmd/payouts

# distroless/static carries CA certificates and time zone data, which properties' local dates need.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /
EXPOSE 8080
# CMD rather than ENTRYPOINT, so Fly's release_command can run /migrate in its place.
CMD ["/api"]
