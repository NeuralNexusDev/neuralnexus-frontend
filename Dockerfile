FROM golang:1.24.2-bookworm AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

RUN go install github.com/a-h/templ/cmd/templ@latest

COPY . .

RUN make downloadtw
RUN make generate

RUN CGO_ENABLED=0 GOOS=linux go build -o webserver .

FROM alpine:edge AS release-stage

WORKDIR /app

COPY --from=build /app/public ./public
COPY --from=build /app/webserver .

CMD ["/app/webserver"]
