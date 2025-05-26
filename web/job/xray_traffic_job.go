package job

import (
	"context"
	"x-ui/logger"
	"x-ui/web/service"
)

type XrayTrafficJob struct {
	ctx            context.Context
	xrayService    service.XrayService
	inboundService service.InboundService
}

func NewXrayTrafficJob(ctx context.Context) *XrayTrafficJob {
	return &XrayTrafficJob{
		ctx: ctx,
	}
}

func (j *XrayTrafficJob) Run() {
	// 统计 Xray 流量
	logger.Info("统计 Xray 流量...")
	if !j.xrayService.IsXrayRunning() {
		return
	}
	traffics, err := j.xrayService.GetXrayTraffic()
	if err != nil {
		logger.Warning("get xray traffic failed:", err)
		return
	}
	err = j.inboundService.AddTraffic(traffics)
	if err != nil {
		logger.Warning("add traffic failed:", err)
	}
}
