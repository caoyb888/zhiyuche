# 多阶段构建：编译所有 cmd，运行镜像通过 CMD 选择服务。
ARG GO_IMAGE=docker.m.daocloud.io/library/golang:1.25-alpine
ARG RUN_IMAGE=docker.m.daocloud.io/library/alpine:3.20

FROM ${GO_IMAGE} AS build
ARG VERSION=dev
ENV GOPROXY=https://goproxy.cn,direct CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN mkdir -p /out && for c in api iot ocpp worker simulator; do \
      go build -trimpath -ldflags "-s -w -X github.com/caoyb888/zhiyuche/apps/api/internal/server.Version=${VERSION}" \
        -o /out/zhiyuche-$c ./cmd/$c; \
    done

FROM ${RUN_IMAGE}
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=build /out/ /app/
USER app
EXPOSE 20080
CMD ["/app/zhiyuche-api"]
