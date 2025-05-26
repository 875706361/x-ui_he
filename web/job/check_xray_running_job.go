package job

import (
	"x-ui/logger"
)

type CheckXrayRunningJob struct{}

func NewCheckXrayRunningJob() *CheckXrayRunningJob {
	return &CheckXrayRunningJob{}
}

func (j *CheckXrayRunningJob) Run() {
	// 检查 Xray 是否正在运行
	logger.Info("检查 Xray 运行状态...")
}
