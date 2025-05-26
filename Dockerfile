FROM golang:1.24.2-alpine AS build

WORKDIR /app
ENV CGO_ENABLED=0

RUN apk update && apk add --no-cache gcc make musl-dev

# Required for gotailwind override
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY Makefile ./

COPY . .

RUN make generate

RUN go build -o webserver .

FROM alpine:edge AS release-stage

WORKDIR /app

COPY --from=build /app/public ./public
COPY --from=build /app/webserver .

CMD ["/app/webserver"]
