package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	_ "unsafe"
	"x-ui/config"
	"x-ui/database"
	"x-ui/logger"
	"x-ui/v2ui"
	"x-ui/web"
	"x-ui/web/global"
	"x-ui/web/service"

	"github.com/op/go-logging"
)

var (
	version   = "Unknown"
	buildTime = "Unknown"
)

func initEnvironment() error {
	// 确保必要的目录存在
	dirs := []string{
		"/etc/x-ui",
		"/usr/local/x-ui",
		filepath.Dir(config.GetDBPath()),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败 %s: %v", dir, err)
		}
	}
	return nil
}

func initLogger() error {
	var level logging.Level

	switch config.GetLogLevel() {
	case config.Debug:
		level = logging.DEBUG
	case config.Info:
		level = logging.INFO
	case config.Warn:
		level = logging.WARNING
	case config.Error:
		level = logging.ERROR
	default:
		return fmt.Errorf("未知的日志级别: %s", config.GetLogLevel())
	}

	logger.InitLogger(level)
	return nil
}

func runWebServer() {
	log.Printf("%v %v (构建时间: %v)", config.GetName(), version, buildTime)

	if err := initEnvironment(); err != nil {
		log.Fatal("初始化环境失败:", err)
	}

	if err := initLogger(); err != nil {
		log.Fatal("初始化日志失败:", err)
	}

	if err := database.InitDB(config.GetDBPath()); err != nil {
		log.Fatal("初始化数据库失败:", err)
	}

	// 创建根上下文并添加取消功能
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := web.NewServer()
	global.SetWebServer(server)

	// 启动服务器
	go func() {
		if err := server.Start(); err != nil {
			logger.Error("启动服务器失败: %v", err)
			cancel()
		}
	}()

	// 监听系统信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	for {
		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGHUP:
				// 重新加载配置
				logger.Info("接收到 SIGHUP 信号，重新加载服务...")
				if err := server.Stop(); err != nil {
					logger.Warning("停止服务器失败:", err)
				}
				server = web.NewServer()
				global.SetWebServer(server)
				go func() {
					if err := server.Start(); err != nil {
						logger.Error("重启服务器失败: %v", err)
						cancel()
					}
				}()
			default:
				// 优雅关闭
				logger.Info("接收到信号 %v，开始优雅关闭...", sig)
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer shutdownCancel()

				if err := server.Stop(); err != nil {
					logger.Error("关闭服务器时发生错误: %v", err)
				}

				// 等待关闭完成或超时
				select {
				case <-shutdownCtx.Done():
					if shutdownCtx.Err() == context.DeadlineExceeded {
						logger.Warning("关闭超时，强制退出")
					}
				}
				return
			}
		case <-ctx.Done():
			logger.Info("上下文已取消，退出主循环")
			return
		}
	}
}

func resetSetting() {
	if err := database.InitDB(config.GetDBPath()); err != nil {
		fmt.Printf("初始化数据库失败: %v\n", err)
		return
	}

	settingService := service.SettingService{}
	if err := settingService.ResetSettings(); err != nil {
		fmt.Printf("重置设置失败: %v\n", err)
	} else {
		fmt.Println("重置设置成功")
	}
}

func updateSetting(port int, username string, password string) {
	if err := database.InitDB(config.GetDBPath()); err != nil {
		fmt.Printf("初始化数据库失败: %v\n", err)
		return
	}

	settingService := service.SettingService{}

	if port > 0 {
		if port < 1 || port > 65535 {
			fmt.Println("端口号无效，请使用 1-65535 之间的值")
			return
		}
		if err := settingService.SetPort(port); err != nil {
			fmt.Printf("设置端口失败: %v\n", err)
		} else {
			fmt.Printf("设置端口 %v 成功\n", port)
		}
	}

	if username != "" || password != "" {
		if len(username) < 4 {
			fmt.Println("用户名长度必须大于等于4位")
			return
		}
		if len(password) < 6 {
			fmt.Println("密码长度必须大于等于6位")
			return
		}

		userService := service.UserService{}
		if err := userService.UpdateFirstUser(username, password); err != nil {
			fmt.Printf("设置用户名和密码失败: %v\n", err)
		} else {
			fmt.Println("设置用户名和密码成功")
		}
	}
}

func showBanner() {
	banner := `
██╗  ██╗      ██╗   ██╗██╗
╚██╗██╔╝      ██║   ██║██║
 ╚███╔╝█████╗ ██║   ██║██║
 ██╔██╗╚════╝ ██║   ██║██║
██╔╝ ██╗      ╚██████╔╝██║
╚═╝  ╚═╝       ╚═════╝ ╚═╝
`
	fmt.Println(banner)
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Build Time: %s\n\n", buildTime)
}

func main() {
	// 捕获panic
	defer func() {
		if r := recover(); r != nil {
			log.Printf("程序发生panic: %v\n", r)
			os.Exit(1)
		}
	}()

	if len(os.Args) < 2 {
		showBanner()
		runWebServer()
		return
	}

	var showVersion bool
	flag.BoolVar(&showVersion, "v", false, "显示版本信息")

	runCmd := flag.NewFlagSet("run", flag.ExitOnError)

	v2uiCmd := flag.NewFlagSet("v2-ui", flag.ExitOnError)
	var dbPath string
	v2uiCmd.StringVar(&dbPath, "db", "/etc/v2-ui/v2-ui.db", "设置 v2-ui 数据库文件路径")

	settingCmd := flag.NewFlagSet("setting", flag.ExitOnError)
	var port int
	var username string
	var password string
	var reset bool
	settingCmd.BoolVar(&reset, "reset", false, "重置所有设置")
	settingCmd.IntVar(&port, "port", 0, "设置面板端口")
	settingCmd.StringVar(&username, "username", "", "设置登录用户名")
	settingCmd.StringVar(&password, "password", "", "设置登录密码")

	oldUsage := flag.Usage
	flag.Usage = func() {
		oldUsage()
		fmt.Println()
		fmt.Println("命令:")
		fmt.Println("    run            运行 Web 面板")
		fmt.Println("    v2-ui         从 v2-ui 迁移")
		fmt.Println("    setting        修改设置")
	}

	flag.Parse()
	if showVersion {
		showBanner()
		return
	}

	switch os.Args[1] {
	case "run":
		if err := runCmd.Parse(os.Args[2:]); err != nil {
			fmt.Println("解析run命令参数失败:", err)
			return
		}
		runWebServer()
	case "v2-ui":
		if err := v2uiCmd.Parse(os.Args[2:]); err != nil {
			fmt.Println("解析v2-ui命令参数失败:", err)
			return
		}
		err := v2ui.MigrateFromV2UI(dbPath)
		if err != nil {
			fmt.Println("从v2-ui迁移失败:", err)
		}
	case "setting":
		if err := settingCmd.Parse(os.Args[2:]); err != nil {
			fmt.Println("解析setting命令参数失败:", err)
			return
		}
		if reset {
			resetSetting()
		} else {
			updateSetting(port, username, password)
		}
	default:
		fmt.Println("未知命令:", os.Args[1])
		flag.Usage()
	}
}
