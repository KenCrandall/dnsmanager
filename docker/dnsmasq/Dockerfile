FROM alpine:3.21

RUN apk add --no-cache dnsmasq

COPY docker/dnsmasq/dnsmasq.conf /etc/dnsmasq.conf

EXPOSE 53/tcp 53/udp 67/udp 69/udp

CMD ["dnsmasq", "--keep-in-foreground", "--conf-file=/etc/dnsmasq.conf"]

