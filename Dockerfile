#指定基础镜像
FROM golang:1.13 as builder

#工作目录
WORKDIR $GOPATH/src/v2ray.com/core
COPY . $GOPATH/src/v2ray.com/core

ENV CGO_ENABLED=0 
ENV GOOS=linux

RUN go get && \
    go build -a -o /v2ray -ldflags '-s -w -extldflags "-static"' ./main && \
    go build -a -o /v2ctl -tags confonly -ldflags '-s -w -extldflags "-static"' ./infra/control/main && \
    wget -qO - https://api.github.com/repos/v2ray/geoip/releases/latest \
    	| grep browser_download_url | cut -d '"' -f 4 \
    	| wget -i - -O /geoip.dat && \
    wget -qO - https://api.github.com/repos/v2ray/domain-list-community/releases/latest \
    	| grep browser_download_url | cut -d '"' -f 4 \
    	| wget -i - -O /geosite.dat && \
    cp release/config/config.json /config.json


FROM alpine:latest

COPY --from=builder /v2ray /usr/bin/v2ray/
COPY --from=builder /v2ctl /usr/bin/v2ray/
COPY --from=builder /geoip.dat /usr/bin/v2ray/
COPY --from=builder /geosite.dat /usr/bin/v2ray/
COPY --from=builder /config.json /etc/v2ray/config.json

RUN set -ex && \
    apk --no-cache add ca-certificates && \
    apk add tzdata && \
    ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    mkdir /var/log/v2ray/ && \
    chmod +x /usr/bin/v2ray/v2ctl && \
    chmod +x /usr/bin/v2ray/v2ray

ENV PATH /usr/bin/v2ray:$PATH

CMD ["v2ray", "-config=/etc/v2ray/config.json"]

