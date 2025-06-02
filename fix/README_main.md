# X-UI 数据库迁移修复工具

## 问题概述

将 FranzKafkaYu/x-ui 面板的数据库导入到 3x-ui 后无法登录的问题修复工具。

这个问题的根本原因是两个面板处理用户密码的方式不同:
- FranzKafkaYu/x-ui: 使用明文存储密码
- 3x-ui: 使用bcrypt加密存储密码

当您尝试导入旧数据库时，3x-ui尝试用bcrypt方式验证明文密码，因此无法登录。

## 解决方案

我们提供了三种方式修复此问题：

### 1. 一键修复脚本 (推荐)

最简单的方式：
```bash
bash quick_fix.sh
```

### 2. Go语言编译工具

如果您熟悉Go语言：
```bash
go build -o password_fix password_fix.go
./password_fix /etc/x-ui/x-ui.db
```

### 3. 手动SQL修复

如果您熟悉SQL：
```bash
sqlite3 /etc/x-ui/x-ui.db
> UPDATE users SET password = 'admin';
> .exit
```

## 修复后步骤

1. 使用以下凭据登录面板：
   - 用户名: (保持原用户名)
   - 密码: `admin`

2. 立即在面板设置中修改密码

## 文件说明

- `quick_fix.sh`: 一键修复脚本
- `password_fix.go`: Go语言修复工具源码
- `README.md`: 修复工具使用说明
- `DIFFERENCES.md`: 两个面板之间的主要差异

## 注意事项

- 本工具会备份原始数据库文件
- 所有用户密码将被重置为 'admin'
- 执行修复前请确保进行了备份

## 在线获取帮助

如有任何问题，请访问GitHub仓库获取最新版本和帮助：
https://github.com/875706361/x-ui_he

## 版本和许可

- 版本: 1.0
- 许可: MIT 