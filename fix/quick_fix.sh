#!/bin/bash

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# 显示标题
echo -e "${GREEN}==============================================${NC}"
echo -e "${GREEN}   x-ui数据库密码修复工具 - 快速修复脚本${NC}"
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

# 创建备份
BACKUP_PATH="${DB_PATH}.bak.$(date +%s)"
echo -e "${YELLOW}正在创建数据库备份...${NC}"
cp "$DB_PATH" "$BACKUP_PATH"

if [ $? -ne 0 ]; then
    echo -e "${RED}备份数据库失败${NC}"
    exit 1
fi

echo -e "${GREEN}数据库备份已创建: $BACKUP_PATH${NC}"

# 检查sqlite3是否安装
if ! command -v sqlite3 &> /dev/null; then
    echo -e "${YELLOW}正在安装sqlite3...${NC}"
    if command -v apt &> /dev/null; then
        apt update && apt install -y sqlite3
    elif command -v yum &> /dev/null; then
        yum install -y sqlite3
    else
        echo -e "${RED}无法自动安装sqlite3，请手动安装后重试${NC}"
        exit 1
    fi
fi

# 执行SQL命令修改密码
echo -e "${YELLOW}正在重置用户密码...${NC}"
sqlite3 "$DB_PATH" "UPDATE users SET password = 'admin';"

if [ $? -ne 0 ]; then
    echo -e "${RED}重置密码失败${NC}"
    echo -e "${YELLOW}正在恢复备份...${NC}"
    cp "$BACKUP_PATH" "$DB_PATH"
    exit 1
fi

# 重启面板
echo -e "${YELLOW}正在重启x-ui面板...${NC}"
systemctl restart x-ui

if [ $? -ne 0 ]; then
    echo -e "${RED}重启面板失败，请手动重启${NC}"
else
    echo -e "${GREEN}面板已重启${NC}"
fi

echo -e "${GREEN}✅ 密码修复完成!${NC}"
echo -e "${GREEN}现在可以使用以下凭据登录:${NC}"
echo -e "   用户名: ${YELLOW}[保持原有用户名]${NC}"
echo -e "   密码: ${YELLOW}admin${NC}"
echo -e "${RED}重要: 登录后请立即修改密码!${NC}"
echo -e "${GREEN}备份文件路径: $BACKUP_PATH${NC}" 