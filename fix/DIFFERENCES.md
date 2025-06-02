# FranzKafkaYu/x-ui 与 3x-ui 的主要差异

本文档列出了两个面板之间的主要差异，有助于理解数据库迁移可能遇到的问题。

## 数据库结构差异

### 用户表和认证

| 特性 | FranzKafkaYu/x-ui | 3x-ui |
|------|-------------------|-------|
| 密码存储 | 明文存储 | bcrypt加密存储 |
| 认证方式 | 简单密码匹配 | bcrypt密码验证 |
| 双因素认证 | 不支持 | 支持 |

### 入站配置 (Inbound)

| 特性 | FranzKafkaYu/x-ui | 3x-ui |
|------|-------------------|-------|
| 端口唯一性 | `port` 字段有唯一约束 | `port` 字段无唯一约束 |
| 客户端统计 | 不支持 | 支持 `ClientStats` 关联 |
| 协议类型 | 较少协议支持 | 更多协议支持，包括socks和wireguard |

### 额外的表和功能

3x-ui 新增了以下表：
1. `OutboundTraffics` - 出站流量统计
2. `InboundClientIps` - 客户端IP记录
3. `HistoryOfSeeders` - 数据库种子记录

## 代码差异

### 认证逻辑

```diff
// FranzKafkaYu/x-ui
- CheckUser(username string, password string) *model.User
// 3x-ui
+ CheckUser(username string, password string, twoFactorCode string) *model.User
```

FranzKafkaYu/x-ui使用简单的数据库查询对比明文密码：
```go
db.Model(model.User{}).Where("username = ? and password = ?", username, password)
```

3x-ui使用bcrypt验证密码：
```go
if !crypto.CheckPasswordHash(user.Password, password) {
    return nil
}
```

### 会话管理

3x-ui增加了更多安全特性：
- 会话过期时间设置
- 支持双因素认证
- 更完善的会话保存机制

### 数据模型

3x-ui的入站配置模型添加了更多字段：
```diff
type Inbound struct {
    // ...共有字段...
+   ClientStats []xray.ClientTraffic `gorm:"foreignKey:InboundId;references:Id" json:"clientStats"`
+   Allocate    string `json:"allocate" form:"allocate"`
}
```

## UI差异

3x-ui提供了更现代化的界面，包括：
- 多语言支持
- 更丰富的统计图表
- 更多客户端管理功能

## 安全性差异

3x-ui加强了安全性：
- 密码使用bcrypt加密存储
- 支持双因素认证
- 更安全的会话管理
- 登录失败通知

## 针对迁移的建议

1. **不要直接替换数据库**：直接替换可能导致无法登录
2. **使用提供的修复工具**：重置密码为可登录状态
3. **迁移后检查配置**：特别是那些依赖于新增表的功能
4. **备份原始数据**：保留原始数据库，以防需要回退

## 结论

虽然两个面板看起来相似，但在数据结构和认证机制上有重要差异。这些差异使得直接迁移数据库可能会导致登录问题。使用提供的修复工具可以解决密码问题，使您能够顺利迁移到3x-ui面板。 