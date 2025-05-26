package service

import (
	"context"
	"os"
	"syscall"
	"time"
	"x-ui/logger"
)

type PanelService struct {
	ctx context.Context
}

func NewPanelService(ctx context.Context) *PanelService {
	return &PanelService{
		ctx: ctx,
	}
}

func (s *PanelService) RestartPanel(delay time.Duration) error {
	p, err := os.FindProcess(syscall.Getpid())
	if err != nil {
		return err
	}
	go func() {
		time.Sleep(delay)
		err := p.Signal(syscall.SIGHUP)
		if err != nil {
			logger.Error("send signal SIGHUP failed:", err)
		}
	}()
	return nil
}
