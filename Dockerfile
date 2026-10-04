FROM golang:1.24.3-alpine AS build

WORKDIR /app
ENV CGO_ENABLED=0

RUN apk update && apk add --no-cache gcc make

COPY go.mod go.sum ./
RUN go mod download

COPY Makefile ./

COPY . .

RUN make build

FROM alpine:edge AS release-stage

WORKDIR /app

COPY --from=build /app/public ./public
COPY --from=build /app/build/webserver .

CMD ["/app/webserver"]
