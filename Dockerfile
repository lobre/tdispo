FROM node:alpine as tailwindcss
RUN apk add make
RUN npm --global config set user root && npm --global install tailwindcss
RUN mkdir /app /app/static
COPY Makefile tailwind.css tailwind.config.js /app/
COPY views /app/views
WORKDIR /app
RUN make tailwindcss

FROM golang:alpine as go
RUN apk add make build-base
COPY . /go/src/app
COPY --from=tailwindcss /app/static/style.css /go/src/app/static/
WORKDIR /go/src/app
RUN --mount=type=cache,target=/root/.cache/go-build make install

FROM alpine
COPY --from=go /go/bin/tdispo /usr/local/bin/
EXPOSE 8080
VOLUME /root
ENTRYPOINT [ "tdispo" ]
CMD [ "-dsn", "/root/tdispo.db", "-lang", "fr" ]
