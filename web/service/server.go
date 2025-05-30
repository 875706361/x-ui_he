package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"runtime"
	"time"
	"x-ui/logger"
	"x-ui/util/sys"
	"x-ui/xray"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type ProcessState string

const (
	Running ProcessState = "running"
	Stop    ProcessState = "stop"
	Error   ProcessState = "error"
)

// Status 表示系统状态信息
type Status struct {
	T   int64   `json:"t"`   // 当前Unix时间戳
	Cpu float64 `json:"cpu"` // CPU使用率
	Mem struct {
		Current uint64 `json:"current"` // 当前内存使用量
		Total   uint64 `json:"total"`   // 总内存量
	} `json:"mem"`
	Swap struct {
		Current uint64 `json:"current"` // 当前Swap使用量
		Total   uint64 `json:"total"`   // 总Swap量
	} `json:"swap"`
	Disk struct {
		Current uint64 `json:"current"` // 当前硬盘使用量
		Total   uint64 `json:"total"`   // 总硬盘容量
	} `json:"disk"`
	Xray struct {
		State    int    `json:"state"`    // Xray状态(0:停止,1:运行,-1:错误)
		ErrorMsg string `json:"errorMsg"` // 错误信息
		Version  string `json:"version"`  // Xray版本
	} `json:"xray"`
	Uptime   uint64    `json:"uptime"`   // 系统运行时间(秒)
	Loads    []float64 `json:"loads"`    // 系统负载(1,5,15分钟)
	TcpCount int       `json:"tcpCount"` // TCP连接数
	UdpCount int       `json:"udpCount"` // UDP连接数
	NetIO    struct {
		Up   uint64 `json:"up"`   // 上传速度(B/s)
		Down uint64 `json:"down"` // 下载速度(B/s)
	} `json:"netIO"`
	NetTraffic struct {
		Sent uint64 `json:"sent"` // 总发送流量
		Recv uint64 `json:"recv"` // 总接收流量
	} `json:"netTraffic"`
}

// Release 表示GitHub发布信息
type Release struct {
	TagName string `json:"tag_name"`
}

// ServerServiceImpl 提供服务器状态和管理功能
type ServerServiceImpl struct {
	ctx         context.Context
	xrayService XrayService
}

// NewServerService 创建新的ServerService实例
func NewServerService(ctx context.Context) ServerService {
	return &ServerServiceImpl{
		ctx: ctx,
	}
}

// SetXrayService 设置XrayService依赖
func (s *ServerServiceImpl) SetXrayService(xrayService XrayService) {
	s.xrayService = xrayService
}

// GetStatus 获取系统状态信息
func (s *ServerServiceImpl) GetStatus(lastStatus *Status) *Status {
	now := time.Now()
	status := &Status{
		T: now.Unix(),
	}

	// 获取CPU使用率
	percents, err := cpu.Percent(0, false)
	if err != nil {
		logger.Warning("获取CPU使用率失败:", err)
	} else if len(percents) > 0 {
		status.Cpu = percents[0]
	}

	// 获取系统运行时间
	upTime, err := host.Uptime()
	if err != nil {
		logger.Warning("获取系统运行时间失败:", err)
	} else {
		status.Uptime = upTime
	}

	// 获取内存使用情况
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		logger.Warning("获取内存信息失败:", err)
	} else {
		status.Mem.Current = memInfo.Used
		status.Mem.Total = memInfo.Total
	}

	// 获取Swap使用情况
	swapInfo, err := mem.SwapMemory()
	if err != nil {
		logger.Warning("获取Swap信息失败:", err)
	} else {
		status.Swap.Current = swapInfo.Used
		status.Swap.Total = swapInfo.Total
	}

	// 获取硬盘使用情况
	distInfo, err := disk.Usage("/")
	if err != nil {
		logger.Warning("获取硬盘使用信息失败:", err)
	} else {
		status.Disk.Current = distInfo.Used
		status.Disk.Total = distInfo.Total
	}

	// 获取系统负载
	avgState, err := load.Avg()
	if err != nil {
		logger.Warning("获取系统负载失败:", err)
		status.Loads = []float64{0.01, 0.01, 0.01}
	} else {
		// 处理负载值，确保不为0
		load1 := avgState.Load1
		load5 := avgState.Load5
		load15 := avgState.Load15

		// 确保值不为0
		if load1 <= 0 {
			load1 = 0.01
		}
		if load5 <= 0 {
			load5 = 0.01
		}
		if load15 <= 0 {
			load15 = 0.01
		}

		status.Loads = []float64{load1, load5, load15}
	}

	// 获取网络IO统计
	ioStats, err := net.IOCounters(false)
	if err != nil {
		logger.Warning("获取网络IO统计失败:", err)
	} else if len(ioStats) > 0 {
		ioStat := ioStats[0]
		status.NetTraffic.Sent = ioStat.BytesSent
		status.NetTraffic.Recv = ioStat.BytesRecv

		// 计算网络速度
		if lastStatus != nil {
			duration := now.Sub(time.Unix(lastStatus.T, 0))
			seconds := float64(duration) / float64(time.Second)

			// 防止除以零
			if seconds > 0 {
				up := uint64(float64(status.NetTraffic.Sent-lastStatus.NetTraffic.Sent) / seconds)
				down := uint64(float64(status.NetTraffic.Recv-lastStatus.NetTraffic.Recv) / seconds)
				status.NetIO.Up = up
				status.NetIO.Down = down
			}
		}
	} else {
		logger.Warning("未找到网络接口信息")
	}

	// 获取TCP/UDP连接数
	status.TcpCount, err = sys.GetTCPCount()
	if err != nil {
		logger.Warning("获取TCP连接数失败:", err)
	}

	status.UdpCount, err = sys.GetUDPCount()
	if err != nil {
		logger.Warning("获取UDP连接数失败:", err)
	}

	// 获取Xray状态
	if s.xrayService != nil {
		if s.xrayService.IsXrayRunning() {
			status.Xray.State = 1
			status.Xray.ErrorMsg = ""
			status.Xray.Version = s.xrayService.GetXrayVersion()
		} else {
			status.Xray.State = 0
			err := s.xrayService.GetXrayErr()
			if err != nil {
				status.Xray.State = -1
				status.Xray.ErrorMsg = err.Error()
			} else {
				status.Xray.ErrorMsg = s.xrayService.GetXrayResult()
			}
		}
	}

	return status
}

// GetXrayVersions 获取可用的Xray版本列表
func (s *ServerServiceImpl) GetXrayVersions() ([]string, error) {
	// 从GitHub API获取发布版本
	url := "https://api.github.com/repos/XTLS/Xray-core/releases"

	// 创建带超时的请求
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求GitHub API失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API返回状态码: %d", resp.StatusCode)
	}

	// 读取响应内容
	var buffer bytes.Buffer
	if _, err := io.Copy(&buffer, resp.Body); err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析JSON
	releases := make([]Release, 0)
	if err := json.Unmarshal(buffer.Bytes(), &releases); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w", err)
	}

	// 提取版本信息
	versions := make([]string, 0, len(releases))
	for _, release := range releases {
		versions = append(versions, release.TagName)
	}

	return versions, nil
}

// downloadXRay 下载指定版本的Xray
func (s *ServerServiceImpl) downloadXRay(version string) (string, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// 转换OS名称
	switch osName {
	case "darwin":
		osName = "macos"
	}

	// 转换架构名称
	switch arch {
	case "amd64":
		arch = "64"
	case "arm64":
		arch = "arm64-v8a"
	}

	// 构建下载URL
	fileName := fmt.Sprintf("Xray-%s-%s.zip", osName, arch)
	url := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/%s/%s", version, fileName)

	// 创建带超时的请求
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Minute) // 下载可能需要更长时间
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("创建下载请求失败: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载Xray失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载返回状态码: %d", resp.StatusCode)
	}

	// 保存文件
	os.Remove(fileName) // 删除可能存在的旧文件
	file, err := os.Create(fileName)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("保存文件失败: %w", err)
	}

	return fileName, nil
}

// UpdateXray 更新Xray到指定版本
func (s *ServerServiceImpl) UpdateXray(version string) error {
	// 下载Xray
	zipFileName, err := s.downloadXRay(version)
	if err != nil {
		return fmt.Errorf("下载Xray失败: %w", err)
	}

	// 打开ZIP文件
	zipFile, err := os.Open(zipFileName)
	if err != nil {
		return fmt.Errorf("打开ZIP文件失败: %w", err)
	}
	defer func() {
		zipFile.Close()
		os.Remove(zipFileName) // 清理下载的文件
	}()

	// 读取ZIP文件内容
	stat, err := zipFile.Stat()
	if err != nil {
		return fmt.Errorf("获取文件状态失败: %w", err)
	}
	reader, err := zip.NewReader(zipFile, stat.Size())
	if err != nil {
		return fmt.Errorf("解析ZIP文件失败: %w", err)
	}

	// 停止当前运行的Xray
	if s.xrayService != nil {
		s.xrayService.StopXray()
	}

	// 安装完成后重启Xray
	defer func() {
		if s.xrayService != nil {
			if err := s.xrayService.RestartXray(true); err != nil {
				logger.Error("重启Xray失败:", err)
			}
		}
	}()

	// 从ZIP文件中复制文件的辅助函数
	copyZipFile := func(zipName string, fileName string) error {
		zipFile, err := reader.Open(zipName)
		if err != nil {
			return fmt.Errorf("打开ZIP内文件失败: %w", err)
		}
		defer zipFile.Close()

		os.Remove(fileName) // 删除可能存在的旧文件
		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_TRUNC, fs.ModePerm)
		if err != nil {
			return fmt.Errorf("创建目标文件失败: %w", err)
		}
		defer file.Close()

		if _, err := io.Copy(file, zipFile); err != nil {
			return fmt.Errorf("复制文件内容失败: %w", err)
		}

		return nil
	}

	// 复制必要的文件
	if err := copyZipFile("xray", xray.GetBinaryPath()); err != nil {
		return fmt.Errorf("安装xray二进制文件失败: %w", err)
	}

	if err := copyZipFile("geosite.dat", xray.GetGeositePath()); err != nil {
		return fmt.Errorf("安装geosite.dat失败: %w", err)
	}

	if err := copyZipFile("geoip.dat", xray.GetGeoipPath()); err != nil {
		return fmt.Errorf("安装geoip.dat失败: %w", err)
	}

	logger.Info("成功更新Xray到版本: %s", version)
	return nil
}
