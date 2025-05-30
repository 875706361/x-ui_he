package job

import (
	"runtime"
	"time"
	"x-ui/logger"
	"x-ui/web/service"

	"github.com/robfig/cron/v3"
)

type StatsNotifyJob struct {
	xrayService    *service.XrayService
	settingService *service.SettingService
	inboundService *service.InboundService
	lastStatus     *runtime.MemStats
	lastTime       time.Time
}

func NewStatsNotifyJob(xrayService *service.XrayService, settingService *service.SettingService, inboundService *service.InboundService) *StatsNotifyJob {
	return &StatsNotifyJob{
		xrayService:    xrayService,
		settingService: settingService,
		inboundService: inboundService,
		lastStatus:     &runtime.MemStats{},
		lastTime:       time.Now(),
	}
}

func (j *StatsNotifyJob) Add(c *cron.Cron) error {
	// 每3分钟检查一次系统状态，避免过于频繁消耗CPU
	_, err := c.AddFunc("@every 3m", func() {
		j.Run()
	})
	return err
}

func (j *StatsNotifyJob) Run() {
	now := time.Now()
	defer func() {
		j.lastTime = now
	}()

	// 当运行时间很长（超过3天）时，减少统计频率
	if now.Sub(j.lastTime) < time.Minute*3 {
		return
	}

	// 获取系统内存状态
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 检测内存使用异常（如果分配的对象数量急剧增加，可能存在内存泄漏）
	if j.lastStatus.Mallocs > 0 && (memStats.Mallocs-j.lastStatus.Mallocs) > 1000000 {
		logger.Warning("可能存在内存泄漏，已分配对象数急剧增加")
		// 强制进行一次垃圾回收
		runtime.GC()
	}

	// 更新上次状态
	*j.lastStatus = memStats

	// 检查是否需要处理到期账户
	count, err := j.inboundService.DisableInvalidInbounds()
	if err != nil {
		logger.Warning("禁用失效入站时发生错误:", err)
	} else if count > 0 {
		logger.Info("已禁用 %d 个到期入站", count)
		// 只有在实际禁用了入站时才重启xray
		if j.xrayService != nil {
			j.xrayService.SetToNeedRestart()
		}
	}

	// 检查流量限制
	overCount, err := j.inboundService.DisableExhaustedInbounds()
	if err != nil {
		logger.Warning("禁用流量超限入站时发生错误:", err)
	} else if overCount > 0 {
		logger.Info("已禁用 %d 个流量超限入站", overCount)
		// 只有在实际禁用了入站时才重启xray
		if j.xrayService != nil {
			j.xrayService.SetToNeedRestart()
		}
	}
}
