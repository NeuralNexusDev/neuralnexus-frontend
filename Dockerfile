FROM alpine:3.14

COPY ./webserver .
COPY ./dist /static

CMD ["./webserver"]
