#!/bin/bash

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# 显示标题
echo -e "${GREEN}==============================================${NC}"
echo -e "${GREEN}   x-ui Reality密钥修复工具 - 快速修复脚本${NC}"
echo -e "${GREEN}==============================================${NC}"

# 检查是否以root运行
if [ "$(id -u)" != "0" ]; then
   echo -e "${RED}此脚本需要以root权限运行${NC}" 
   exit 1
fi

# 定义数据库路径
DB_PATH="/etc/x-ui/x-ui.db"

# 检查数据库文件是否存在
if [ ! -f "$DB_PATH" ]; then
    echo -e "${RED}错误: 数据库文件不存在: $DB_PATH${NC}"
    echo -e "${YELLOW}请检查x-ui安装路径或手动指定数据库路径${NC}"
    exit 1
fi

# 创建临时目录
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR" || exit 1

echo -e "${YELLOW}正在下载修复工具...${NC}"

# 下载修复工具
curl -sLo reality_key_fix.go https://raw.githubusercontent.com/875706361/x-ui_he/master/fix/reality_key_fix.go

if [ $? -ne 0 ]; then
    echo -e "${RED}下载修复工具失败${NC}"
    exit 1
fi

# 检查Go是否安装
if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}正在安装Go...${NC}"
    
    # 检测系统
    if command -v apt &> /dev/null; then
        apt update && apt install -y golang
    elif command -v yum &> /dev/null; then
        yum install -y golang
    else
        echo -e "${RED}无法自动安装Go，请手动安装后重试${NC}"
        echo -e "${YELLOW}您可以访问 https://golang.org/doc/install 获取安装指南${NC}"
        exit 1
    fi
fi

echo -e "${YELLOW}正在初始化Go模块...${NC}"
# 初始化Go模块
go mod init reality_key_fix
if [ $? -ne 0 ]; then
    echo -e "${RED}初始化Go模块失败${NC}"
    exit 1
fi

echo -e "${YELLOW}正在安装依赖包...${NC}"
# 安装依赖
go get gorm.io/driver/sqlite
go get gorm.io/gorm
if [ $? -ne 0 ]; then
    echo -e "${RED}安装依赖包失败${NC}"
    exit 1
fi

echo -e "${YELLOW}正在编译修复工具...${NC}"
# 打印Go模块环境信息
go env
# 生成并更新go.mod文件
go mod tidy
# 编译
go build -o reality_key_fix reality_key_fix.go

if [ $? -ne 0 ]; then
    echo -e "${RED}编译修复工具失败${NC}"
    exit 1
fi

echo -e "${YELLOW}正在执行修复...${NC}"
./reality_key_fix "$DB_PATH"

if [ $? -ne 0 ]; then
    echo -e "${RED}修复失败${NC}"
    exit 1
fi

# 清理临时文件
cd / && rm -rf "$TEMP_DIR"

echo -e "${GREEN}正在重启x-ui面板...${NC}"
systemctl restart x-ui

if [ $? -ne 0 ]; then
    echo -e "${RED}重启面板失败，请手动重启${NC}"
    echo -e "${YELLOW}您可以运行: systemctl restart x-ui${NC}"
else
    echo -e "${GREEN}面板已重启${NC}"
fi

echo -e "${GREEN}✅ Reality密钥修复工具执行完成!${NC}"
echo -e "${YELLOW}如果您的入站配置使用Reality但链接无法使用，请检查服务器设置${NC}" 