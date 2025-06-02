package controller

import (
	"x-ui/web/service"
	"x-ui/web/session"

	"github.com/gin-gonic/gin"
)

type XrayController struct {
	xrayService    service.XrayService
	inboundService service.InboundService
	settingService service.SettingService
}

func NewXrayController(g *gin.RouterGroup) *XrayController {
	a := &XrayController{}
	a.xrayService = service.XrayService{}
	a.inboundService = service.InboundService{}
	a.settingService = service.SettingService{}

	a.initRouter(g)
	return a
}

func (a *XrayController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/xray")
	g.Use(session.ValidatorMiddleware)

	g.POST("/status", a.Status)
	g.POST("/inbounds", a.Inbounds)
	g.POST("/update", a.Update)
	g.POST("/restart", a.Restart)
	g.POST("/generateRealityKeyPair", a.GenerateRealityKeyPair)
}

func (a *XrayController) Status(c *gin.Context) {
	status := a.xrayService.GetXrayStatus()
	jsonObj(c, status)
}

func (a *XrayController) Inbounds(c *gin.Context) {
	inbounds, err := a.inboundService.GetAllInbounds()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, inbounds)
}

func (a *XrayController) Update(c *gin.Context) {
	err := a.xrayService.UpdateXray()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.updateSucc"), nil)
}

func (a *XrayController) Restart(c *gin.Context) {
	err := a.xrayService.RestartXray()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.restart"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.restartSucc"), nil)
}

// 添加Reality密钥生成API
func (a *XrayController) GenerateRealityKeyPair(c *gin.Context) {
	// 调用xray库生成密钥对
	privateKey, publicKey, err := a.xrayService.GenerateRealityKeyPair()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.xrayConfigError"), err)
		return
	}
	jsonObj(c, map[string]string{
		"privateKey": privateKey,
		"publicKey":  publicKey,
	})
}
