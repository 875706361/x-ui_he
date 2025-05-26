package controller

import (
	"github.com/gin-gonic/gin"
)

type XUIController struct {
	router *gin.RouterGroup

	inboundController *InboundController
	settingController *SettingController
}

func NewXUIController(router *gin.RouterGroup) *XUIController {
	c := &XUIController{
		router:            router,
		inboundController: NewInboundController(router),
		settingController: NewSettingController(router),
	}
	c.initRouter()
	return c
}

func (c *XUIController) initRouter() {
	c.router.Use(c.checkLogin)
	g := c.router.Group("/xui")
	g.GET("/", c.index)
	g.GET("/inbounds", c.inboundController.index)
	g.GET("/setting", c.settingController.index)
}

func (c *XUIController) checkLogin(ctx *gin.Context) {
	// 检查登录状态
	ctx.Next()
}

func (c *XUIController) index(ctx *gin.Context) {
	ctx.HTML(200, "index.html", nil)
}

func (a *XUIController) inbounds(c *gin.Context) {
	html(c, "inbounds.html", "入站列表", nil)
}

func (a *XUIController) setting(c *gin.Context) {
	html(c, "setting.html", "设置", nil)
}
