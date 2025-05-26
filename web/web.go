package web

import (
	"context"
	"embed"
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

type Server struct {
	httpServer *http.Server
	listener   net.Listener

	index  *controller.IndexController
	server *controller.ServerController
	xui    *controller.XUIController

	xrayService    service.XrayService
	settingService service.SettingService
	inboundService service.InboundService

	cron *cron.Cron

	ctx    context.Context
	cancel context.CancelFunc

	started bool
	mu      sync.Mutex
}

func NewServer() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		ctx:     ctx,
		cancel:  cancel,
		started: false,
	}
}

func (s *Server) initializeServices() error {
	s.xrayService = service.NewXrayService(s.ctx)
	s.settingService = service.NewSettingService(s.ctx)
	s.inboundService = service.NewInboundService(s.ctx)

	return nil
}

func (s *Server) initializeControllers(router *gin.RouterGroup) {
	s.index = controller.NewIndexController(router)
	s.server = controller.NewServerController(router)
	s.xui = controller.NewXUIController(router)
}

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

func (s *Server) initRouter() (*gin.Engine, error) {
	if config.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()

	// 添加自定义中间件
	engine.Use(s.recoveryMiddleware())
	engine.Use(s.corsMiddleware())
	engine.Use(s.securityHeadersMiddleware())

	secret, err := s.settingService.GetSecret()
	if err != nil {
		return nil, fmt.Errorf("获取密钥失败: %v", err)
	}

	basePath, err := s.settingService.GetBasePath()
	if err != nil {
		return nil, fmt.Errorf("获取基础路径失败: %v", err)
	}
	assetsBasePath := basePath + "assets/"

	store := cookie.NewStore(secret)
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7天
		Secure:   !config.IsDebug(),
		HttpOnly: true,
	})

	engine.Use(sessions.Sessions("session", store))
	engine.Use(func(c *gin.Context) {
		c.Set("base_path", basePath)
	})
	engine.Use(func(c *gin.Context) {
		uri := c.Request.RequestURI
		if strings.HasPrefix(uri, assetsBasePath) {
			c.Header("Cache-Control", "max-age=31536000")
		}
	})

	if err := s.initI18n(engine); err != nil {
		return nil, fmt.Errorf("初始化国际化失败: %v", err)
	}

	if config.IsDebug() {
		files, err := s.getHtmlFiles()
		if err != nil {
			return nil, err
		}
		engine.LoadHTMLFiles(files...)
		engine.StaticFS(basePath+"assets", http.FS(os.DirFS("web/assets")))
	} else {
		t, err := s.getHtmlTemplate(engine.FuncMap)
		if err != nil {
			return nil, err
		}
		engine.SetHTMLTemplate(t)
		engine.StaticFS(basePath+"assets", http.FS(&wrapAssetsFS{FS: assetsFS}))
	}

	router := engine.Group(basePath)
	s.initializeControllers(router)

	return engine, nil
}

func (s *Server) recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("服务器发生严重错误:", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "服务器内部错误",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func (s *Server) securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	}
}

func (s *Server) initI18n(engine *gin.Engine) error {
	bundle := i18n.NewBundle(language.SimplifiedChinese)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	err := fs.WalkDir(i18nFS, "translation", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("遍历翻译文件失败: %v", err)
		}
		if d.IsDir() {
			return nil
		}
		data, err := i18nFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("读取翻译文件失败: %v", err)
		}
		_, err = bundle.ParseMessageFileBytes(data, path)
		if err != nil {
			return fmt.Errorf("解析翻译文件失败: %v", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	engine.Use(func(c *gin.Context) {
		accept := c.GetHeader("Accept-Language")
		localizer := i18n.NewLocalizer(bundle, accept)
		c.Set("localizer", localizer)
		c.Next()
	})

	return nil
}

func (s *Server) startTask() error {
	s.cron = cron.New(cron.WithSeconds())

	// 添加定时任务
	if _, err := s.cron.AddJob("@every 10s", job.NewCheckXrayRunningJob()); err != nil {
		return fmt.Errorf("添加Xray运行状态检查任务失败: %v", err)
	}
	if _, err := s.cron.AddJob("@every 1m", job.NewXrayTrafficJob(s.ctx)); err != nil {
		return fmt.Errorf("添加流量统计任务失败: %v", err)
	}

	s.cron.Start()
	return nil
}

func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("服务器已经在运行")
	}
	s.started = true
	s.mu.Unlock()

	if err := s.initializeServices(); err != nil {
		return fmt.Errorf("初始化服务失败: %v", err)
	}

	engine, err := s.initRouter()
	if err != nil {
		return fmt.Errorf("初始化路由失败: %v", err)
	}

	if err := s.startTask(); err != nil {
		return fmt.Errorf("启动定时任务失败: %v", err)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", 54321))
	if err != nil {
		return fmt.Errorf("监听端口失败: %v", err)
	}
	s.listener = listener

	s.httpServer = &http.Server{
		Handler:        engine,
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		MaxHeaderBytes: maxHeaderBytes,
	}

	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP服务器错误:", err)
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return fmt.Errorf("服务器未运行")
	}
	s.started = false
	s.mu.Unlock()

	s.cancel()

	if s.cron != nil {
		s.cron.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("关闭HTTP服务器失败: %v", err)
		}
	}

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			return fmt.Errorf("关闭监听器失败: %v", err)
		}
	}

	return nil
}

func (s *Server) GetCtx() context.Context {
	return s.ctx
}

func (s *Server) GetCron() *cron.Cron {
	return s.cron
}
