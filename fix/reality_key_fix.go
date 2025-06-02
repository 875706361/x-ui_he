package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 入站配置模型
type Inbound struct {
	Id             int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId         int    `json:"userId"`
	Up             int64  `json:"up"`
	Down           int64  `json:"down"`
	Total          int64  `json:"total"`
	Remark         string `json:"remark"`
	Enable         bool   `json:"enable"`
	ExpiryTime     int64  `json:"expiryTime"`
	Listen         string `json:"listen"`
	Port           int    `json:"port"`
	Protocol       string `json:"protocol"`
	Settings       string `json:"settings"`
	StreamSettings string `json:"streamSettings"`
	Tag            string `json:"tag"`
	Sniffing       string `json:"sniffing"`
}

// Reality配置结构
type RealitySettings struct {
	Show         bool     `json:"show"`
	Dest         string   `json:"dest"`
	Xver         int      `json:"xver"`
	ServerNames  string   `json:"serverNames"`
	PrivateKey   string   `json:"privateKey"`
	MinClientVer string   `json:"minClientVer"`
	MaxClientVer string   `json:"maxClientVer"`
	MaxTimeDiff  int      `json:"maxTimeDiff"`
	ShortIds     []string `json:"shortIds"`
	Settings     struct {
		PublicKey   string `json:"publicKey"`
		Fingerprint string `json:"fingerprint"`
		ServerName  string `json:"serverName"`
		SpiderX     string `json:"spiderX"`
	} `json:"settings"`
}

// StreamSettings结构
type StreamSettings struct {
	Network     string           `json:"network"`
	Security    string           `json:"security"`
	TlsSettings json.RawMessage  `json:"tlsSettings,omitempty"`
	Reality     *RealitySettings `json:"realitySettings,omitempty"`
	// 其他字段省略
}

// 生成Reality密钥对
func generateRealityKeyPair() (string, string, error) {
	// 调用xray命令生成密钥对
	cmd := exec.Command("xray", "x25519")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", err
	}

	// 解析输出
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")

	// 提取私钥和公钥
	var privateKey, publicKey string
	for _, line := range lines {
		if strings.Contains(line, "Private key:") {
			privateKey = strings.TrimSpace(strings.TrimPrefix(line, "Private key:"))
		} else if strings.Contains(line, "Public key:") {
			publicKey = strings.TrimSpace(strings.TrimPrefix(line, "Public key:"))
		}
	}

	if privateKey == "" || publicKey == "" {
		return "", "", fmt.Errorf("无法生成Reality密钥对")
	}

	return privateKey, publicKey, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: reality_key_fix <数据库路径>")
		fmt.Println("例如: reality_key_fix /etc/x-ui/x-ui.db")
		return
	}

	dbPath := os.Args[1]

	// 检查数据库文件是否存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("数据库文件不存在: %s", dbPath)
	}

	// 创建备份
	backupPath := dbPath + ".reality_bak." + fmt.Sprint(os.Getpid())
	data, err := os.ReadFile(dbPath)
	if err != nil {
		log.Fatalf("读取数据库文件失败: %v", err)
	}

	err = os.WriteFile(backupPath, data, 0644)
	if err != nil {
		log.Fatalf("创建数据库备份失败: %v", err)
	}

	fmt.Printf("已创建备份: %s\n", backupPath)

	// 打开数据库
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}

	// 查询所有入站配置
	var inbounds []Inbound
	err = db.Find(&inbounds).Error
	if err != nil {
		log.Fatalf("查询入站配置失败: %v", err)
	}

	fmt.Printf("找到 %d 个入站配置\n", len(inbounds))

	fixedCount := 0
	// 逐个处理入站配置
	for i, inbound := range inbounds {
		// 只处理VLESS和TROJAN协议，因为只有这些协议支持Reality
		if inbound.Protocol != "vless" && inbound.Protocol != "trojan" {
			continue
		}

		var streamSettings StreamSettings
		err = json.Unmarshal([]byte(inbound.StreamSettings), &streamSettings)
		if err != nil {
			fmt.Printf("解析入站配置#%d流设置失败，跳过: %v\n", inbound.Id, err)
			continue
		}

		// 仅处理Reality安全类型
		if streamSettings.Security != "reality" {
			continue
		}

		fmt.Printf("正在处理入站配置 #%d (%s)...\n", inbound.Id, inbound.Remark)

		// 检查是否需要修复Reality配置
		if streamSettings.Reality == nil ||
			streamSettings.Reality.PrivateKey == "" ||
			streamSettings.Reality.Settings.PublicKey == "" {

			fmt.Printf("  检测到Reality密钥丢失，正在生成新密钥...\n")

			// 生成新的密钥对
			privateKey, publicKey, err := generateRealityKeyPair()
			if err != nil {
				fmt.Printf("  生成Reality密钥失败: %v，跳过\n", err)
				continue
			}

			// 如果Reality结构为nil，需要初始化
			if streamSettings.Reality == nil {
				streamSettings.Reality = &RealitySettings{
					Show:        true,
					Dest:        "www.microsoft.com:443",
					ServerNames: "www.microsoft.com",
					ShortIds:    []string{""},
				}
			}

			// 更新密钥
			streamSettings.Reality.PrivateKey = privateKey
			streamSettings.Reality.Settings.PublicKey = publicKey

			// 如果fingerprint为空，设置为默认值
			if streamSettings.Reality.Settings.Fingerprint == "" {
				streamSettings.Reality.Settings.Fingerprint = "chrome"
			}

			// 更新流设置
			newStreamSettings, err := json.Marshal(streamSettings)
			if err != nil {
				fmt.Printf("  序列化流设置失败: %v，跳过\n", err)
				continue
			}

			// 更新数据库中的流设置
			err = db.Model(&Inbound{}).Where("id = ?", inbound.Id).Update("stream_settings", string(newStreamSettings)).Error
			if err != nil {
				fmt.Printf("  更新入站配置#%d失败: %v，跳过\n", inbound.Id, err)
				continue
			}

			fmt.Printf("  已修复Reality配置，新密钥对已生成\n")
			fmt.Printf("  私钥: %s\n", privateKey)
			fmt.Printf("  公钥: %s\n", publicKey)

			fixedCount++
		} else {
			fmt.Printf("  Reality配置正常，无需修复\n")
		}
	}

	if fixedCount > 0 {
		fmt.Printf("\n已成功修复 %d 个Reality配置\n", fixedCount)
		fmt.Println("请重启x-ui面板以应用更改: systemctl restart x-ui")
	} else {
		fmt.Println("\n未发现需要修复的Reality配置")
	}

	fmt.Printf("备份文件路径: %s\n", backupPath)
}
