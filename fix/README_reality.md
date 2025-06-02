# Reality密钥丢失问题修复指南

## 问题描述

在将FranzKafkaYu/x-ui面板数据库导入到3x-ui或其他x-ui分支后，使用Reality协议的入站配置可能会出现公钥丢失的问题。这会导致客户端连接失败，因为链接中缺少必要的`pbk`参数（公钥）。

## 问题原因

Reality协议需要一对密钥（私钥和公钥）才能正常工作：
- **私钥**：存储在服务器端，用于解密客户端发送的数据
- **公钥**：分享给客户端，客户端使用它来加密数据

当数据库迁移时，这些密钥信息可能因为数据结构差异或JSON字段解析问题而丢失。

## 解决方案

我们提供了三种恢复Reality密钥的方法：

### 方法一：使用自动修复脚本（推荐）

这是最简单的方法，脚本会自动检测并修复所有问题：

```bash
bash -c "$(curl -L https://raw.githubusercontent.com/875706361/x-ui_he/master/fix/reality_fix.sh)"
```

该脚本会：
1. 下载并编译Reality密钥修复工具
2. 自动检测并修复所有丢失Reality密钥的入站配置
3. 重启x-ui服务以应用更改

### 方法二：手动编译和运行修复工具

如果您希望手动控制修复过程：

1. 下载修复工具源代码：
```bash
wget https://raw.githubusercontent.com/875706361/x-ui_he/master/fix/reality_key_fix.go
```

2. 编译工具：
```bash
go build -o reality_key_fix reality_key_fix.go
```

3. 运行修复工具：
```bash
./reality_key_fix /etc/x-ui/x-ui.db  # 替换为您的数据库路径
```

4. 重启x-ui服务：
```bash
systemctl restart x-ui
```

### 方法三：手动重新生成密钥

如果您只有少量入站配置，也可以手动重新生成密钥：

1. 登录到x-ui面板
2. 编辑使用Reality的入站配置
3. 在"传输配置"部分，点击"生成"按钮重新生成密钥对
4. 保存配置

## 预防措施

为避免未来迁移时出现此问题，请您：

1. **在迁移前备份数据库**：永远保留原始数据库的备份
2. **导出客户端配置**：在迁移前导出所有客户端的配置信息
3. **记录Reality密钥**：将每个入站的Reality私钥/公钥保存下来

## 技术说明

Reality密钥在数据库中以JSON格式存储在`inbounds`表的`stream_settings`字段中，结构大致如下：

```json
{
  "network": "tcp",
  "security": "reality",
  "realitySettings": {
    "show": true,
    "dest": "www.microsoft.com:443",
    "serverNames": "www.microsoft.com",
    "privateKey": "私钥内容",
    "shortIds": [""],
    "settings": {
      "publicKey": "公钥内容",
      "fingerprint": "chrome"
    }
  }
}
```

修复工具通过重新生成这些密钥对并更新数据库来解决问题。

## 注意事项

- 密钥修复后，客户端需要使用新的链接或更新配置
- 更新后的公钥会显示在修复工具的输出中
- 如果您设置了自定义的fingerprint或其他高级参数，修复后可能需要重新配置 