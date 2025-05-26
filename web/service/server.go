package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
	"x-ui/logger"
	"x-ui/util/sys"
	"x-ui/xray"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

type ProcessState string

const (
	Running ProcessState = "running"
	Stop    ProcessState = "stop"
	Error   ProcessState = "error"
)

type Status struct {
	T   time.Time `json:"-"`
	Cpu float64   `json:"cpu"`
	Mem struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"mem"`
	Swap struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"swap"`
	Disk struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"disk"`
	Xray struct {
		State    ProcessState `json:"state"`
		ErrorMsg string       `json:"errorMsg"`
		Version  string       `json:"version"`
	} `json:"xray"`
	Uptime   uint64    `json:"uptime"`
	Loads    []float64 `json:"loads"`
	TcpCount int       `json:"tcpCount"`
	UdpCount int       `json:"udpCount"`
	NetIO    struct {
		Up   uint64 `json:"up"`
		Down uint64 `json:"down"`
	} `json:"netIO"`
	NetTraffic struct {
		Sent uint64 `json:"sent"`
		Recv uint64 `json:"recv"`
	} `json:"netTraffic"`
}

type Release struct {
	TagName string `json:"tag_name"`
}

type ServerService struct {
	xrayService    XrayService
	statsCache     *Status
	lastUpdate     time.Time
	mutex          sync.RWMutex
	sampleInterval time.Duration
	errRetryDelay  time.Duration
	httpClient     *http.Client
}

func NewServerService(xrayService XrayService) *ServerService {
	return &ServerService{
		xrayService:    xrayService,
		sampleInterval: 2 * time.Second,
		errRetryDelay:  5 * time.Second,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *ServerService) GetStatus(lastStatus *Status) *Status {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()

	if s.statsCache != nil && now.Sub(s.lastUpdate) < s.sampleInterval {
		return s.statsCache
	}

	status := &Status{
		T: now,
	}

	var wg sync.WaitGroup
	var cpuMutex, memMutex, diskMutex, loadMutex, netMutex sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		if percents, err := cpu.Percent(s.sampleInterval, false); err != nil {
			logger.Warning("获取CPU使用率失败:", err)
		} else if len(percents) > 0 {
			cpuMutex.Lock()
			status.Cpu = percents[0]
			cpuMutex.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if memInfo, err := mem.VirtualMemory(); err != nil {
			logger.Warning("获取内存信息失败:", err)
		} else {
			memMutex.Lock()
			status.Mem.Current = memInfo.Used
			status.Mem.Total = memInfo.Total
			memMutex.Unlock()
		}

		if swapInfo, err := mem.SwapMemory(); err != nil {
			logger.Warning("获取交换分区信息失败:", err)
		} else {
			memMutex.Lock()
			status.Swap.Current = swapInfo.Used
			status.Swap.Total = swapInfo.Total
			memMutex.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if diskInfo, err := disk.Usage("/"); err != nil {
			logger.Warning("获取磁盘使用情况失败:", err)
		} else {
			diskMutex.Lock()
			status.Disk.Current = diskInfo.Used
			status.Disk.Total = diskInfo.Total
			diskMutex.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if loadInfo, err := load.Avg(); err != nil {
			logger.Warning("获取系统负载失败:", err)
		} else {
			loadMutex.Lock()
			status.Loads = []float64{loadInfo.Load1, loadInfo.Load5, loadInfo.Load15}
			loadMutex.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if ioStats, err := net.IOCounters(false); err != nil {
			logger.Warning("获取网络IO统计失败:", err)
		} else if len(ioStats) > 0 {
			netMutex.Lock()
			ioStat := ioStats[0]
			status.NetTraffic.Sent = ioStat.BytesSent
			status.NetTraffic.Recv = ioStat.BytesRecv

			if lastStatus != nil {
				duration := now.Sub(lastStatus.T)
				seconds := float64(duration) / float64(time.Second)
				up := uint64(float64(status.NetTraffic.Sent-lastStatus.NetTraffic.Sent) / seconds)
				down := uint64(float64(status.NetTraffic.Recv-lastStatus.NetTraffic.Recv) / seconds)
				status.NetIO.Up = up
				status.NetIO.Down = down
			}
			netMutex.Unlock()
		}
	}()

	wg.Wait()

	if tcpCount, err := sys.GetTCPCount(); err != nil {
		logger.Warning("获取TCP连接数失败:", err)
	} else {
		status.TcpCount = tcpCount
	}

	if udpCount, err := sys.GetUDPCount(); err != nil {
		logger.Warning("获取UDP连接数失败:", err)
	} else {
		status.UdpCount = udpCount
	}

	if upTime, err := host.Uptime(); err != nil {
		logger.Warning("获取系统运行时间失败:", err)
	} else {
		status.Uptime = upTime
	}

	if s.xrayService.IsXrayRunning() {
		status.Xray.State = Running
		status.Xray.ErrorMsg = ""
	} else {
		if err := s.xrayService.GetXrayErr(); err != nil {
			status.Xray.State = Error
		} else {
			status.Xray.State = Stop
		}
		status.Xray.ErrorMsg = s.xrayService.GetXrayResult()
	}
	status.Xray.Version = s.xrayService.GetXrayVersion()

	s.statsCache = status
	s.lastUpdate = now

	return status
}

func (s *ServerService) GetXrayVersions() ([]string, error) {
	url := "https://api.github.com/repos/XTLS/Xray-core/releases"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buffer := bytes.NewBuffer(make([]byte, 0, 8192))
	_, err = io.Copy(buffer, resp.Body)
	if err != nil {
		return nil, err
	}

	var releases []Release
	if err = json.Unmarshal(buffer.Bytes(), &releases); err != nil {
		return nil, err
	}

	versions := make([]string, 0, len(releases))
	for _, release := range releases {
		versions = append(versions, release.TagName)
	}
	return versions, nil
}

func (s *ServerService) downloadXRay(version string) (string, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch osName {
	case "darwin":
		osName = "macos"
	}

	switch arch {
	case "amd64":
		arch = "64"
	case "arm64":
		arch = "arm64-v8a"
	}

	fileName := fmt.Sprintf("Xray-%s-%s.zip", osName, arch)
	url := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/%s/%s", version, fileName)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	os.Remove(fileName)
	file, err := os.Create(fileName)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	return fileName, nil
}

func (s *ServerService) UpdateXray(version string) error {
	zipFileName, err := s.downloadXRay(version)
	if err != nil {
		return err
	}

	zipFile, err := os.Open(zipFileName)
	if err != nil {
		return err
	}
	defer func() {
		zipFile.Close()
		os.Remove(zipFileName)
	}()

	stat, err := zipFile.Stat()
	if err != nil {
		return err
	}
	reader, err := zip.NewReader(zipFile, stat.Size())
	if err != nil {
		return err
	}

	s.xrayService.StopXray()
	defer func() {
		err := s.xrayService.RestartXray(true)
		if err != nil {
			logger.Error("start xray failed:", err)
		}
	}()

	copyZipFile := func(zipName string, fileName string) error {
		zipFile, err := reader.Open(zipName)
		if err != nil {
			return err
		}
		os.Remove(fileName)
		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_TRUNC, fs.ModePerm)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(file, zipFile)
		return err
	}

	err = copyZipFile("xray", xray.GetBinaryPath())
	if err != nil {
		return err
	}
	err = copyZipFile("geosite.dat", xray.GetGeositePath())
	if err != nil {
		return err
	}
	err = copyZipFile("geoip.dat", xray.GetGeoipPath())
	if err != nil {
		return err
	}

	return nil
}
