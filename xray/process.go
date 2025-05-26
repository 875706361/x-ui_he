package xray

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"x-ui/util/common"

	"github.com/Workiva/go-datastructures/queue"
	statsservice "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
)

var trafficRegex = regexp.MustCompile("(inbound|outbound)>>>([^>]+)>>>traffic>>>(downlink|uplink)")

func GetBinaryName() string {
	return fmt.Sprintf("xray-%s-%s", runtime.GOOS, runtime.GOARCH)
}

func GetBinaryPath() string {
	return "bin/" + GetBinaryName()
}

func GetConfigPath() string {
	return "bin/config.json"
}

func GetGeositePath() string {
	return "bin/geosite.dat"
}

func GetGeoipPath() string {
	return "bin/geoip.dat"
}

func stopProcess(p *Process) {
	p.Stop()
}

type Process struct {
	*process
}

type TrafficCache struct {
	traffics []*Traffic
	lastTime time.Time
	mutex    sync.RWMutex
}

func (c *TrafficCache) Set(traffics []*Traffic) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.traffics = traffics
	c.lastTime = time.Now()
}

func (c *TrafficCache) Get() ([]*Traffic, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if time.Since(c.lastTime) > 2*time.Second {
		return nil, false
	}
	return c.traffics, true
}

func NewProcess(xrayConfig *Config) *Process {
	p := &Process{newProcess(xrayConfig)}
	runtime.SetFinalizer(p, stopProcess)
	return p
}

type process struct {
	cmd *exec.Cmd

	version string
	apiPort int

	config       *Config
	lines        *queue.Queue
	exitErr      error
	trafficCache *TrafficCache
}

func newProcess(config *Config) *process {
	return &process{
		version:      "Unknown",
		config:       config,
		lines:        queue.New(100),
		trafficCache: &TrafficCache{},
	}
}

func (p *process) IsRunning() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	if p.cmd.ProcessState == nil {
		return true
	}
	return false
}

func (p *process) GetErr() error {
	return p.exitErr
}

func (p *process) GetResult() string {
	if p.lines.Empty() && p.exitErr != nil {
		return p.exitErr.Error()
	}
	items, _ := p.lines.TakeUntil(func(item interface{}) bool {
		return true
	})
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, item.(string))
	}
	return strings.Join(lines, "\n")
}

func (p *process) GetVersion() string {
	return p.version
}

func (p *Process) GetAPIPort() int {
	return p.apiPort
}

func (p *Process) GetConfig() *Config {
	return p.config
}

func (p *process) refreshAPIPort() {
	for _, inbound := range p.config.InboundConfigs {
		if inbound.Tag == "api" {
			p.apiPort = inbound.Port
			break
		}
	}
}

func (p *process) refreshVersion() {
	cmd := exec.Command(GetBinaryPath(), "-version")
	data, err := cmd.Output()
	if err != nil {
		p.version = "Unknown"
	} else {
		datas := bytes.Split(data, []byte(" "))
		if len(datas) <= 1 {
			p.version = "Unknown"
		} else {
			p.version = string(datas[1])
		}
	}
}

func (p *process) Start() (err error) {
	if p.IsRunning() {
		return errors.New("xray is already running")
	}

	defer func() {
		if err != nil {
			p.exitErr = err
		}
	}()

	data, err := json.MarshalIndent(p.config, "", "  ")
	if err != nil {
		return common.NewErrorf("生成 xray 配置文件失败: %v", err)
	}
	configPath := GetConfigPath()
	err = os.WriteFile(configPath, data, fs.ModePerm)
	if err != nil {
		return common.NewErrorf("写入配置文件失败: %v", err)
	}

	cmd := exec.Command(GetBinaryPath(), "-c", configPath)
	p.cmd = cmd

	stdReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	errReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	go func() {
		defer func() {
			common.Recover("")
			stdReader.Close()
		}()
		reader := bufio.NewReaderSize(stdReader, 8192)
		for {
			line, _, err := reader.ReadLine()
			if err != nil {
				return
			}
			if p.lines.Len() >= 100 {
				p.lines.Get(1)
			}
			p.lines.Put(string(line))
		}
	}()

	go func() {
		defer func() {
			common.Recover("")
			errReader.Close()
		}()
		reader := bufio.NewReaderSize(errReader, 8192)
		for {
			line, _, err := reader.ReadLine()
			if err != nil {
				return
			}
			if p.lines.Len() >= 100 {
				p.lines.Get(1)
			}
			p.lines.Put(string(line))
		}
	}()

	go func() {
		err := cmd.Run()
		if err != nil {
			p.exitErr = err
		}
	}()

	p.refreshVersion()
	p.refreshAPIPort()

	return nil
}

func (p *process) Stop() error {
	if !p.IsRunning() {
		return errors.New("xray is not running")
	}
	return p.cmd.Process.Kill()
}

func (p *process) GetTraffic(reset bool) ([]*Traffic, error) {
	if p.apiPort == 0 {
		return nil, common.NewError("xray api port wrong:", p.apiPort)
	}

	// 检查缓存
	if !reset {
		if traffics, ok := p.trafficCache.Get(); ok {
			return traffics, nil
		}
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, fmt.Sprintf("127.0.0.1:%v", p.apiPort), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := statsservice.NewStatsServiceClient(conn)
	request := &statsservice.QueryStatsRequest{
		Reset_: reset,
	}
	resp, err := client.QueryStats(ctx, request)
	if err != nil {
		return nil, err
	}

	tagTrafficMap := make(map[string]*Traffic)
	traffics := make([]*Traffic, 0)

	for _, stat := range resp.GetStat() {
		matchs := trafficRegex.FindStringSubmatch(stat.Name)
		if len(matchs) < 4 {
			continue
		}

		isInbound := matchs[1] == "inbound"
		tag := matchs[2]
		isDown := matchs[3] == "downlink"

		if tag == "api" {
			continue
		}

		traffic, ok := tagTrafficMap[tag]
		if !ok {
			traffic = &Traffic{
				IsInbound: isInbound,
				Tag:       tag,
			}
			tagTrafficMap[tag] = traffic
			traffics = append(traffics, traffic)
		}

		if isDown {
			traffic.Down = stat.Value
		} else {
			traffic.Up = stat.Value
		}
	}

	// 更新缓存
	if !reset {
		p.trafficCache.Set(traffics)
	}

	return traffics, nil
}
