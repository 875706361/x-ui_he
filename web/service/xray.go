package service

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"x-ui/logger"
	"x-ui/xray"

	"go.uber.org/atomic"
)

var (
	// 全局变量
	p                 *xray.Process
	lock              sync.Mutex
	isNeedXrayRestart atomic.Bool
	result            string

	// 常见错误定义
	ErrXrayNotRunning = errors.New("xray未运行")

	// 缓存控制
	trafficCacheTTL = time.Second * 10
	configCacheTTL  = time.Minute * 5

	// 内存控制
	lastGCTime = time.Now()
	gcInterval = time.Minute * 30 // 30分钟强制GC一次
)

// XrayServiceImpl 实现XrayService接口
type XrayServiceImpl struct {
	ctx            context.Context
	inboundService InboundService
	settingService SettingService

	// 配置缓存
	configCache     *xray.Config
	configCacheTime time.Time
	configMutex     sync.RWMutex

	// 流量缓存
	trafficCache     map[string]int64
	trafficCacheTime time.Time
	trafficMutex     sync.RWMutex

	// 资源统计
	memStats     runtime.MemStats
	lastMemStats runtime.MemStats
	memStatsTime time.Time
}

// NewXrayService 创建新的XrayService实例
func NewXrayService(ctx context.Context) XrayService {
	service := &XrayServiceImpl{
		ctx:              ctx,
		configCacheTime:  time.Time{}, // 零值，表示未缓存
		trafficCacheTime: time.Time{},
		trafficCache:     make(map[string]int64),
		memStatsTime:     time.Now(),
	}

	// 启动内存监控
	go service.monitorMemory(ctx)

	return service
}

// 监控内存使用情况
func (s *XrayServiceImpl) monitorMemory(ctx context.Context) {
	ticker := time.NewTicker(time.Minute * 10) // 每10分钟检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			runtime.ReadMemStats(&s.memStats)

			// 检测内存增长情况
			if s.lastMemStats.Alloc > 0 {
				increase := float64(s.memStats.Alloc-s.lastMemStats.Alloc) / float64(s.lastMemStats.Alloc)
				if increase > 0.5 { // 内存增长超过50%
					logger.Warning("内存使用量增长显著: %.2f%%，当前: %dMB", increase*100, s.memStats.Alloc/1024/1024)
					// 强制GC
					runtime.GC()
				}
			}

			// 定期GC，避免内存长期增长
			if time.Since(lastGCTime) > gcInterval {
				logger.Debug("执行定期垃圾回收")
				runtime.GC()
				lastGCTime = time.Now()
			}

			s.lastMemStats = s.memStats
			s.memStatsTime = time.Now()

		case <-ctx.Done():
			return
		}
	}
}

// SetInboundService 设置InboundService依赖
func (s *XrayServiceImpl) SetInboundService(inboundService InboundService) {
	s.inboundService = inboundService
}

// SetSettingService 设置SettingService依赖
func (s *XrayServiceImpl) SetSettingService(settingService SettingService) {
	s.settingService = settingService
}

// IsXrayRunning 检查Xray是否正在运行
func (s *XrayServiceImpl) IsXrayRunning() bool {
	return p != nil && p.IsRunning()
}

// GetXrayErr 获取Xray错误
func (s *XrayServiceImpl) GetXrayErr() error {
	if p == nil {
		return nil
	}
	return p.GetErr()
}

// GetXrayResult 获取Xray运行结果
func (s *XrayServiceImpl) GetXrayResult() string {
	if result != "" {
		return result
	}
	if s.IsXrayRunning() {
		return ""
	}
	if p == nil {
		return ""
	}
	result = p.GetResult()
	return result
}

// GetXrayVersion 获取Xray版本
func (s *XrayServiceImpl) GetXrayVersion() string {
	if p == nil {
		return "Unknown"
	}
	return p.GetVersion()
}

// GetXrayConfig 获取Xray配置
func (s *XrayServiceImpl) GetXrayConfig() (*xray.Config, error) {
	// 检查服务依赖
	if s.settingService == nil || s.inboundService == nil {
		return nil, errors.New("服务依赖未设置")
	}

	// 先尝试读取缓存
	s.configMutex.RLock()
	if s.configCache != nil && time.Since(s.configCacheTime) < configCacheTTL {
		defer s.configMutex.RUnlock()
		return s.configCache, nil
	}
	s.configMutex.RUnlock()

	// 缓存失效，重新生成配置
	s.configMutex.Lock()
	defer s.configMutex.Unlock()

	// 再次检查缓存（可能在获取锁的过程中已被其他goroutine更新）
	if s.configCache != nil && time.Since(s.configCacheTime) < configCacheTTL {
		return s.configCache, nil
	}

	// 获取模板配置
	templateConfig, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}

	// 解析模板
	xrayConfig := &xray.Config{}
	if err = json.Unmarshal([]byte(templateConfig), xrayConfig); err != nil {
		return nil, err
	}

	// 获取所有入站配置
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}

	// 添加启用的入站配置
	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		inboundConfig := inbound.GenXrayInboundConfig()
		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}

	// 更新缓存
	s.configCache = xrayConfig
	s.configCacheTime = time.Now()

	return xrayConfig, nil
}

// GetXrayTraffic 获取Xray流量统计
func (s *XrayServiceImpl) GetXrayTraffic() (map[string]int64, error) {
	if !s.IsXrayRunning() {
		return nil, ErrXrayNotRunning
	}

	// 先尝试读取缓存
	s.trafficMutex.RLock()
	if time.Since(s.trafficCacheTime) < trafficCacheTTL {
		result := make(map[string]int64, len(s.trafficCache))
		for k, v := range s.trafficCache {
			result[k] = v
		}
		s.trafficMutex.RUnlock()
		return result, nil
	}
	s.trafficMutex.RUnlock()

	// 获取新的流量数据
	traffic, err := p.GetTraffic(true)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	s.trafficMutex.Lock()
	defer s.trafficMutex.Unlock()

	result := make(map[string]int64, len(traffic))
	for _, t := range traffic {
		result[t.Tag] = t.Value
		s.trafficCache[t.Tag] = t.Value
	}
	s.trafficCacheTime = time.Now()

	return result, nil
}

// RestartXray 重启Xray服务
func (s *XrayServiceImpl) RestartXray(force bool) error {
	lock.Lock()
	defer lock.Unlock()
	logger.Debug("restart xray, force:", force)

	// 获取最新配置
	xrayConfig, err := s.GetXrayConfig()
	if err != nil {
		return err
	}

	// 检查是否需要重启
	if p != nil && p.IsRunning() {
		if !force && p.GetConfig().Equals(xrayConfig) {
			logger.Debug("配置未变化，无需重启xray")
			return nil
		}
		// 停止当前运行的进程
		if err := p.Stop(); err != nil {
			logger.Warning("停止xray时发生错误:", err)
		}
	}

	// 创建新进程并启动
	p = xray.NewProcess(xrayConfig)
	result = ""

	// 清除缓存
	s.InvalidateCache()

	// 启动前先做一次GC
	runtime.GC()
	lastGCTime = time.Now()

	return p.Start()
}

// StopXray 停止Xray服务
func (s *XrayServiceImpl) StopXray() error {
	lock.Lock()
	defer lock.Unlock()
	logger.Debug("stop xray")

	if !s.IsXrayRunning() {
		return ErrXrayNotRunning
	}

	// 清除缓存
	s.InvalidateCache()

	return p.Stop()
}

// SetToNeedRestart 标记Xray需要重启
func (s *XrayServiceImpl) SetToNeedRestart() {
	isNeedXrayRestart.Store(true)
}

// IsNeedRestartAndSetFalse 检查是否需要重启并重置标记
func (s *XrayServiceImpl) IsNeedRestartAndSetFalse() bool {
	return isNeedXrayRestart.CAS(true, false)
}

// InvalidateCache 使配置缓存失效
func (s *XrayServiceImpl) InvalidateCache() {
	s.configMutex.Lock()
	s.configCache = nil
	s.configCacheTime = time.Time{}
	s.configMutex.Unlock()

	s.trafficMutex.Lock()
	s.trafficCache = make(map[string]int64)
	s.trafficCacheTime = time.Time{}
	s.trafficMutex.Unlock()

	// 做一次GC
	runtime.GC()
	lastGCTime = time.Now()
}

// 生成Reality密钥对
func (s *XrayService) GenerateRealityKeyPair() (string, string, error) {
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
		return "", "", errors.New("无法生成Reality密钥对")
	}

	return privateKey, publicKey, nil
}
