FROM golang:alpine as builder
RUN apk add build-base
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN GOOS=linux go build -o tdispo

FROM alpine
COPY --from=builder /build/tdispo /usr/local/bin/
EXPOSE 8080
VOLUME /root
ENTRYPOINT [ "tdispo" ]
CMD [ "-dsn", "/root/tdispo.db", "-lang", "fr" ]
