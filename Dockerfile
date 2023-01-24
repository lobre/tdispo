FROM node:alpine as node
FROM golang:alpine as builder

# gather npm and node
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm
COPY --from=node /usr/local/bin/node /usr/local/bin/
RUN apk upgrade --no-cache -U && \
  apk add --no-cache binutils libstdc++ && \
  strip /usr/local/bin/node && \
  apk del binutils

# install build deps
RUN apk add build-base
RUN npm --global install tailwindcss

# build application
COPY . /go/src/app
WORKDIR /go/src/app
RUN --mount=type=cache,target=/root/.cache/go-build go generate && go install

# final image
FROM alpine
RUN apk add --no-cache sqlite
COPY --from=builder /go/bin/tdispo /usr/local/bin/
EXPOSE 8080
VOLUME /root
ENTRYPOINT [ "tdispo" ]
CMD [ "-dsn", "/root/tdispo.db" ]
