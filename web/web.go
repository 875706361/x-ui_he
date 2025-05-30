package web

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"x-ui/config"
	"x-ui/logger"
	"x-ui/web/controller"
	"x-ui/web/job"
	"x-ui/web/service"

	"github.com/BurntSushi/toml"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/robfig/cron/v3"
	"golang.org/x/text/language"
)

//go:embed assets/*
var assetsFS embed.FS

//go:embed html/*
var htmlFS embed.FS

//go:embed translation/*
var i18nFS embed.FS

var (
	startTime = time.Now()
	version   = "Unknown"
)

const (
	shutdownTimeout = 30 * time.Second
	readTimeout     = 15 * time.Second
	writeTimeout    = 15 * time.Second
	maxHeaderBytes  = 1 << 20 // 1MB
)

type wrapAssetsFS struct {
	embed.FS
}

func (f *wrapAssetsFS) Open(name string) (fs.File, error) {
	file, err := f.FS.Open("assets/" + name)
	if err != nil {
		return nil, err
	}
	return &wrapAssetsFile{
		File: file,
	}, nil
}

type wrapAssetsFile struct {
	fs.File
}

func (f *wrapAssetsFile) Stat() (fs.FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &wrapAssetsFileInfo{
		FileInfo: info,
	}, nil
}

type wrapAssetsFileInfo struct {
	fs.FileInfo
}

func (f *wrapAssetsFileInfo) ModTime() time.Time {
	return startTime
}

// Server 表示Web服务器的结构体
type Server struct {
	httpServer *http.Server
	listener   net.Listener

	// 控制器
	index  *controller.IndexController
	server *controller.ServerController
	xui    *controller.XUIController

	// 服务
	xrayService    *service.XrayService
	settingService *service.SettingService
	inboundService *service.InboundService

	// 定时任务
	cron *cron.Cron

	// 上下文管理
	ctx    context.Context
	cancel context.CancelFunc

	// 状态
	started bool
	mu      sync.Mutex
}

// NewServer 创建一个新的Web服务器实例
func NewServer() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		ctx:     ctx,
		cancel:  cancel,
		started: false,
	}
}

// 初始化服务
func (s *Server) initializeServices() error {
	s.xrayService = service.NewXrayService(s.ctx)
	s.settingService = service.NewSettingService(s.ctx)
	s.inboundService = service.NewInboundService(s.ctx)

	// 设置服务之间的依赖关系
	s.xrayService.SetInboundService(s.inboundService)
	s.xrayService.SetSettingService(s.settingService)

	// 初始化serverService
	serverService := service.NewServerService(s.ctx)
	serverService.SetXrayService(s.xrayService)

	return nil
}

// 初始化控制器
func (s *Server) initializeControllers(router *gin.RouterGroup) {
	s.index = controller.NewIndexController(router)
	s.server = controller.NewServerController(router)
	s.xui = controller.NewXUIController(router)
}

// 获取HTML文件列表
func (s *Server) getHtmlFiles() ([]string, error) {
	files := make([]string, 0)
	dir, _ := os.Getwd()
	err := fs.WalkDir(os.DirFS(dir), "web/html", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("遍历HTML文件失败: %v", err)
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// 获取HTML模板
func (s *Server) getHtmlTemplate(funcMap template.FuncMap) (*template.Template, error) {
	t := template.New("").Funcs(funcMap)
	err := fs.WalkDir(htmlFS, "html", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("遍历模板文件失败: %v", err)
		}

		if d.IsDir() {
			newT, err := t.ParseFS(htmlFS, path+"/*.html")
			if err != nil {
				logger.Warning("解析模板目录失败:", err)
				return nil
			}
			t = newT
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}

// 初始化路由
func (s *Server) initRouter() (*gin.Engine, error) {
	if config.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()

	// 添加中间件
	engine.Use(s.recoveryMiddleware())
	engine.Use(s.corsMiddleware())
	engine.Use(s.securityHeadersMiddleware())
	engine.Use(s.rateLimitMiddleware())

	secret, err := s.settingService.GetSecret()
	if err != nil {
		return nil, fmt.Errorf("获取密钥失败: %v", err)
	}

	basePath, err := s.settingService.GetBasePath()
	if err != nil {
		return nil, fmt.Errorf("获取基础路径失败: %v", err)
	}

	assetsBasePath := basePath + "assets/"
	store := cookie.NewStore([]byte(secret))
	engine.Use(sessions.Sessions("session", store))

	if len(basePath) > 0 && basePath != "/" {
		engine.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, basePath)
		})
	}

	engine.GET(basePath, func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, basePath+"panel/")
	})

	// 初始化国际化
	if err := s.initI18n(engine); err != nil {
		return nil, err
	}

	// 设置模板函数
	funcMap := template.FuncMap{
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	// 解析HTML模板
	t, err := s.getHtmlTemplate(funcMap)
	if err != nil {
		return nil, err
	}
	engine.SetHTMLTemplate(t)

	// 设置静态文件服务
	engine.StaticFS(assetsBasePath, http.FS(&wrapAssetsFS{FS: assetsFS}))

	// 初始化API路由
	g := engine.Group(basePath)

	// 初始化服务
	if err := s.initializeServices(); err != nil {
		return nil, err
	}

	// 初始化控制器
	s.initializeControllers(g)

	return engine, nil
}

// 恢复中间件 - 处理panic
func (s *Server) recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Web服务器panic: %v", r)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "服务器内部错误",
				})
			}
		}()
		c.Next()
	}
}

// CORS中间件 - 处理跨域请求
func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// 安全头中间件 - 添加安全相关的HTTP头
func (s *Server) securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
		c.Writer.Header().Set("Referrer-Policy", "no-referrer-when-downgrade")
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:;")
		c.Next()
	}
}

// 速率限制中间件 - 防止API滥用
func (s *Server) rateLimitMiddleware() gin.HandlerFunc {
	// 定义每个IP的请求限制
	type clientLimit struct {
		count    int
		lastSeen time.Time
	}

	// 保存IP限制状态的映射
	var (
		ipLimits      = make(map[string]*clientLimit)
		mu            sync.Mutex
		limit         = 100              // 每分钟最大请求数
		window        = time.Minute      // 时间窗口
		cleanInterval = time.Minute * 10 // 清理间隔
	)

	// 定期清理过期记录
	go func() {
		ticker := time.NewTicker(cleanInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				mu.Lock()
				now := time.Now()
				for ip, cl := range ipLimits {
					if now.Sub(cl.lastSeen) > window {
						delete(ipLimits, ip)
					}
				}
				mu.Unlock()
			case <-s.ctx.Done():
				return
			}
		}
	}()

	return func(c *gin.Context) {
		// 静态资源请求不限制
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Next()
			return
		}

		ip := c.ClientIP()
		mu.Lock()

		if ipLimits[ip] == nil {
			ipLimits[ip] = &clientLimit{count: 0, lastSeen: time.Now()}
		}

		cl := ipLimits[ip]

		// 如果已经过了时间窗口，重置计数
		if time.Since(cl.lastSeen) > window {
			cl.count = 0
			cl.lastSeen = time.Now()
		}

		cl.count++
		blocked := cl.count > limit
		mu.Unlock()

		if blocked {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "请求过于频繁，请稍后再试",
			})
			return
		}

		c.Next()
	}
}

// 初始化国际化
func (s *Server) initI18n(engine *gin.Engine) error {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	// 加载翻译文件
	err := fs.WalkDir(i18nFS, "translation", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".toml") {
			return nil
		}
		content, err := i18nFS.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = bundle.ParseMessageFileBytes(content, path)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 为每个请求设置本地化器
	engine.Use(func(c *gin.Context) {
		locale := c.Query("locale")
		if locale == "" {
			locale = c.GetHeader("Accept-Language")
		}
		localizer := i18n.NewLocalizer(bundle, locale)
		c.Set("localizer", localizer)
		c.Next()
	})

	return nil
}

// 启动定时任务
func (s *Server) startTask() error {
	logger.Info("开始初始化定时任务")
	s.cron = cron.New(cron.WithSeconds())
	c := s.cron

	// 添加定时任务
	var err error
	// 统计和通知任务
	statsNotifyJob := job.NewStatsNotifyJob(s.xrayService, s.settingService, s.inboundService)
	err = statsNotifyJob.Add(c)
	if err != nil {
		return fmt.Errorf("添加统计通知任务失败: %v", err)
	}

	// Xray 重载任务
	xrayReloadJob := job.NewXrayReloadJob(s.xrayService)
	err = xrayReloadJob.Add(c)
	if err != nil {
		return fmt.Errorf("添加Xray重载任务失败: %v", err)
	}

	c.Start()
	logger.Info("定时任务初始化完成")
	return nil
}

// 启动Web服务
func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("服务器已经启动")
	}
	s.started = true
	s.mu.Unlock()

	// 初始化路由
	engine, err := s.initRouter()
	if err != nil {
		return err
	}

	// 启动定时任务
	if err := s.startTask(); err != nil {
		return err
	}

	// 获取端口
	port, err := s.settingService.GetPort()
	if err != nil {
		return err
	}

	// 启动服务器
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	s.listener = listener

	s.httpServer = &http.Server{
		Handler:        engine,
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		MaxHeaderBytes: maxHeaderBytes,
	}

	logger.Info("Web服务器启动在端口 %d", port)
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Error("Web服务器启动失败: %v", err)
		}
	}()

	return nil
}

// 停止Web服务
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return errors.New("服务器未启动")
	}
	s.started = false
	s.mu.Unlock()

	// 停止定时任务
	if s.cron != nil {
		logger.Info("停止定时任务")
		s.cron.Stop()
	}

	// 取消上下文
	s.cancel()

	// 优雅关闭HTTP服务器
	if s.httpServer != nil {
		logger.Info("停止Web服务器")
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return err
		}
	}

	// 关闭监听器
	if s.listener != nil {
		return s.listener.Close()
	}

	return nil
}

// GetCtx 获取服务器上下文
func (s *Server) GetCtx() context.Context {
	return s.ctx
}

// GetCron 获取定时任务调度器
func (s *Server) GetCron() *cron.Cron {
	return s.cron
}
