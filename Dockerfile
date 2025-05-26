# 构建阶段
FROM golang:1.21-alpine AS builder

# 设置构建参数
ARG VERSION=dev
ARG BUILD_DATE
ARG COMMIT_SHA

# 设置工作目录
WORKDIR /build

# 安装构建依赖
RUN apk add --no-cache git gcc musl-dev

# 复制源代码
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 设置构建信息
RUN go build -ldflags "-s -w \
    -X 'main.Version=${VERSION}' \
    -X 'main.BuildTime=${BUILD_DATE}' \
    -X 'main.GitCommit=${COMMIT_SHA}'" \
    -o x-ui main.go

# 运行阶段
FROM alpine:3.19

# 添加标签
LABEL maintainer="875706361 <875706361@qq.com>"
LABEL version="${VERSION}"
LABEL build_date="${BUILD_DATE}"
LABEL git_commit="${COMMIT_SHA}"

# 安装运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    && rm -rf /var/cache/apk/*

# 创建非 root 用户
RUN adduser -D -H -s /sbin/nologin x-ui

# 设置工作目录
WORKDIR /usr/local/x-ui

# 从构建阶段复制文件
COPY --from=builder /build/x-ui .
COPY --from=builder /build/bin/. ./bin/
COPY --from=builder /build/web/. ./web/

# 设置权限
RUN chown -R x-ui:x-ui . \
    && chmod +x x-ui

# 创建数据卷
VOLUME ["/etc/x-ui"]

# 暴露端口
EXPOSE 54321

# 切换到非 root 用户
USER x-ui

# 设置健康检查
HEALTHCHECK --interval=30s --timeout=3s \
    CMD wget --no-verbose --tries=1 --spider http://localhost:54321/ping || exit 1

# 启动命令
ENTRYPOINT ["./x-ui"]
