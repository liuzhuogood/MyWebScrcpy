package main

import (
	"flag"
	"fmt"
	"log"
	"mywebscrcpy/internal/dependencies"
	"mywebscrcpy/internal/desktop"
	"os"
)

type runtimeOptions struct {
	HTTPS         *bool
	Listen, Ready *string
	Exit          bool
}

func parseRuntimeOptions() runtimeOptions {
	httpsFlag := flag.Bool("https", false, "启用 HTTPS（使用内置证书）")
	guiFlag := flag.Bool("gui", false, "启动桌面窗口")
	noGUIFlag := flag.Bool("no-gui", false, "仅启动服务")
	listenFlag := flag.String("listen", "", "服务监听地址（默认 0.0.0.0:8080）")
	dataDirFlag := flag.String("data-dir", "", "运行数据根目录（默认当前工作目录）")
	printModeFlag := flag.Bool("print-mode", false, "输出自动选择的启动模式")
	readyFlag := flag.String("desktop-ready", "", "桌面宿主专用就绪文件")
	flag.Parse()
	if flag.NArg() != 0 {
		log.Fatalf("不支持的位置参数: %v", flag.Args())
	}
	mode, err := desktop.Select(desktop.Options{GUI: *guiFlag, NoGUI: *noGUIFlag, Shell: desktop.ShellPath(), DesktopAvailable: desktop.DesktopAvailable()})
	if err != nil {
		log.Fatal(err)
	}
	if *printModeFlag {
		fmt.Println(mode)
		return runtimeOptions{Exit: true}
	}
	if mode == desktop.GUI {
		if *httpsFlag || *listenFlag != "" || *dataDirFlag != "" || *readyFlag != "" {
			log.Fatal("桌面模式使用独立监听和数据目录；自定义服务参数请搭配 --no-gui")
		}
		fallback, launchErr := desktop.Launch(desktop.ShellPath(), *guiFlag)
		if !fallback {
			if launchErr != nil {
				log.Fatal(launchErr)
			}
			return runtimeOptions{Exit: true}
		}
		log.Printf("[Desktop] 图形环境初始化失败，进入服务模式: %v", launchErr)
	}
	if *dataDirFlag != "" {
		if err := os.MkdirAll(*dataDirFlag, 0700); err != nil {
			log.Fatalf("创建数据目录失败: %v", err)
		}
		if err := os.Chdir(*dataDirFlag); err != nil {
			log.Fatalf("进入数据目录失败: %v", err)
		}
	}

	if err := dependencies.Load(); err != nil {
		log.Fatalf("依赖配置失败: %v", err)
	}

	return runtimeOptions{HTTPS: httpsFlag, Listen: listenFlag, Ready: readyFlag}
}
