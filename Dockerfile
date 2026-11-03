FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/anitoon4bot ./cmd/anitoon4bot

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/anitoon4bot /app/anitoon4bot
USER nonroot:nonroot
ENTRYPOINT ["/app/anitoon4bot"]
