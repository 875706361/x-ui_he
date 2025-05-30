// 所有服务已移至独立文件：
// - xray.go: XrayService
// - setting.go: SettingService
// - inbound.go: InboundService
// - user.go: UserService
// - panel.go: PanelService
// - server.go: ServerService

// 服务接口定义文件

package service

import (
	"x-ui/xray"
)

// XrayService 定义Xray服务接口
type XrayService interface {
	// Xray状态管理
	IsXrayRunning() bool
	GetXrayErr() error
	GetXrayResult() string
	GetXrayVersion() string
	
	// 配置管理
	GetXrayConfig() (*xray.Config, error)
	GetXrayTraffic() (map[string]int64, error)
	
	// 操作
	RestartXray(force bool) error
	StopXray() error
	SetToNeedRestart()
	IsNeedRestartAndSetFalse() bool
	InvalidateCache()
	
	// 设置依赖
	SetInboundService(inboundService InboundService)
	SetSettingService(settingService SettingService)
}

// SettingService 定义设置服务接口
type SettingService interface {
	GetSecret() ([]byte, error)
	GetBasePath() (string, error)
	GetPort() (int, error)
	GetListen() (string, error)
	GetCertFile() (string, error)
	GetKeyFile() (string, error)
	GetXrayConfigTemplate() (string, error)
	GetTimeLocation() (string, error)
	ResetSettings() error
	SetPort(port int) error
}

// InboundService 定义入站服务接口
type InboundService interface {
	GetAllInbounds() ([]Inbound, error)
	AddTraffic(traffics map[string]int64) error
	DisableInvalidInbounds() (int, error)
	DisableExhaustedInbounds() (int, error)
}

// ServerService 定义服务器状态服务接口
type ServerService interface {
	GetStatus(lastStatus *Status) *Status
	GetXrayVersions() ([]string, error)
	UpdateXray(version string) error
	SetXrayService(xrayService XrayService)
}

// Inbound 入站配置
type Inbound struct {
	ID      int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Enable  bool   `json:"enable"`
	// 其他必要字段...
	
	// 生成Xray配置
	GenXrayInboundConfig() *xray.InboundConfig
}
