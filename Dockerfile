FROM golang:1.22.1-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
RUN go install github.com/a-h/templ/cmd/templ@latest

COPY . .

RUN go generate

RUN CGO_ENABLED=0 GOOS=linux go build -o webserver .

FROM alpine:edge AS release-stage

WORKDIR /app

COPY ./public ./public
COPY --from=build /app/webserver .

CMD ["/app/webserver"]
