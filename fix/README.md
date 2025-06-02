# 密码修复工具

这个工具用于解决将FranzKafkaYu/x-ui面板数据库导入3x-ui后无法登录的问题。

## 问题原因

问题分析：
1. FranzKafkaYu/x-ui面板中密码存储为明文
2. 3x-ui面板中密码使用bcrypt加密存储
3. 导入数据库后，3x-ui面板会尝试使用bcrypt验证明文密码，导致登录失败

## 解决方案

该工具将重置数据库中的用户密码为"admin"，使您可以重新登录到面板。

## 使用方法

### 直接编译运行

1. 安装golang环境
2. 编译工具
   ```bash
   cd fix
   go build -o password_fix password_fix.go
   ```
3. 运行工具
   ```bash
   ./password_fix /etc/x-ui/x-ui.db
   ```

### 或手动执行SQL命令

如果您不想编译此工具，也可以手动执行以下SQL命令：

1. 创建数据库备份
   ```bash
   cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.bak
   ```

2. 使用SQLite客户端运行
   ```bash
   sqlite3 /etc/x-ui/x-ui.db
   ```

3. 在SQLite提示符下执行：
   ```sql
   UPDATE users SET password = 'admin';
   .exit
   ```

## 执行后步骤

1. 工具执行完成后，您可以使用以下凭据登录：
   - 用户名：(保持原样，通常为admin)
   - 密码：admin

2. 登录成功后，请立即在面板中修改您的密码

## 注意事项

- 执行前会自动创建数据库备份
- 所有用户密码都会重置为"admin"
- 此工具只修复密码问题，不会修改其他数据 