# build stage
FROM golang:alpine AS build-env

RUN apk add --no-cache git 
RUN go get -d -v github.com/mholt/caddy/caddy github.com/pedronasser/caddy-search github.com/simia-tech/caddy-locale
WORKDIR /go/src/github.com/mholt/caddy/caddy

RUN sed -i '/This is where other plugins get plugged in (imported)/a _ "github.com/pedronasser/caddy-search"' caddymain/run.go \
 && sed -i '/This is where other plugins get plugged in (imported)/a _ "github.com/simia-tech/caddy-locale"' caddymain/run.go \
 && go install -v . \
 && /go/bin/caddy -version

FROM alpine:edge
EXPOSE 80

RUN apk add --no-cache wget mailcap ca-certificates
COPY --from=build-env /go/bin/caddy /usr/sbin/caddy

COPY docker/caddy.conf /etc/caddy.conf
COPY public /srv/www

CMD ["/usr/sbin/caddy", "-conf", "/etc/caddy.conf"]
