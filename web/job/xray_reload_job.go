package job

import (
	"sync"
	"time"
	"x-ui/logger"
	"x-ui/web/service"

	"github.com/robfig/cron/v3"
)

var (
	lastRestartTime    time.Time
	minRestartInterval = time.Minute * 5 // 最小重启间隔
	restartLock        sync.Mutex
)

type XrayReloadJob struct {
	xrayService *service.XrayService
}

func NewXrayReloadJob(xrayService *service.XrayService) *XrayReloadJob {
	return &XrayReloadJob{
		xrayService: xrayService,
	}
}

func (j *XrayReloadJob) Add(c *cron.Cron) error {
	// 每30秒检查一次是否需要重启Xray
	_, err := c.AddFunc("@every 30s", func() {
		j.Run()
	})
	return err
}

func (j *XrayReloadJob) Run() {
	// 如果没有Xray服务实例，直接返回
	if j.xrayService == nil {
		return
	}

	// 检查Xray是否需要重启
	if j.xrayService.IsNeedRestartAndSetFalse() {
		restartLock.Lock()
		defer restartLock.Unlock()

		now := time.Now()
		// 检查是否满足最小重启间隔
		if now.Sub(lastRestartTime) < minRestartInterval {
			logger.Info("距离上次重启时间不足 %s，暂缓重启", minRestartInterval)
			// 设置需要重启标志，在下一个符合间隔的周期再重启
			j.xrayService.SetToNeedRestart()
			return
		}

		// 更新上次重启时间
		lastRestartTime = now

		logger.Info("检测到Xray需要重启")
		err := j.xrayService.RestartXray(false)
		if err != nil {
			logger.Warning("重启Xray失败:", err)
		}
	}
}
