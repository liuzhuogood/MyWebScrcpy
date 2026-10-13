package main

import (
	"fmt"
	"log"
	"mywebscrcpy/internal/scheduler"
	"mywebscrcpy/internal/updater"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The gate covers in-flight requests, including long-lived device connections.
// Update routes are excluded so progress and status stay available.
var updateGate sync.RWMutex
var updatePrepared atomic.Bool
var updatePrepare = func() error { return fmt.Errorf("更新服务尚未就绪") }
var updateResume = func() {}

func updateMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/dependencies") || strings.HasPrefix(r.URL.Path, "/api/version/") || strings.HasPrefix(r.URL.Path, "/api/desktop/") {
			next.ServeHTTP(w, r)
			return
		}
		if receipt := os.Getenv("MYWEBSCRCPY_UPDATE_RECEIPT"); receipt != "" {
			if _, err := os.Stat(receipt + ".confirmed"); err != nil {
				http.Error(w, "正在验证更新，请稍后重试", http.StatusServiceUnavailable)
				return
			}
		}
		if !updateGate.TryRLock() {
			http.Error(w, "正在安装更新，请稍后重试", http.StatusServiceUnavailable)
			return
		}
		defer updateGate.RUnlock()
		next.ServeHTTP(w, r)
	})
}

// Keep cron disabled until the supervisor has committed the new startup.
func startUpdateAwareScheduler(sched *scheduler.Scheduler) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		receipt := os.Getenv("MYWEBSCRCPY_UPDATE_RECEIPT")
		if receipt != "" {
			for {
				if _, err := os.Stat(receipt + ".confirmed"); err == nil {
					break
				}
				select {
				case <-stop:
					return
				case <-time.After(100 * time.Millisecond):
				}
			}
		}
		if err := sched.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "[Scheduler] 启动失败: %v\n", err)
		}
	}()
	return func() { close(stop); <-done; sched.Stop() }
}

func handleUpdateCommand() bool {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(strings.TrimSpace(string(versionBytes)))
		return true
	}
	if len(os.Args) == 3 && os.Args[1] == "--update-worker" {
		if err := updater.RunWorker(os.Args[2]); err != nil {
			log.Printf("[Updater] %v", err)
			os.Exit(1)
		}
		return true
	}
	if len(os.Args) == 4 && os.Args[1] == "--update-backup" {
		if err := updater.BackupRuntime(os.Args[2], os.Args[3]); err != nil {
			log.Fatal(err)
		}
		return true
	}
	if len(os.Args) == 3 && os.Args[1] == "--watch-app-update" {
		if err := updater.WatchApp(os.Args[2]); err != nil {
			log.Printf("[Updater] %v", err)
			os.Exit(1)
		}
		return true
	}
	return false
}

func serviceUpdatePolicy(desktopMode bool, addr string) updater.Policy {
	policy := updater.DetectPolicy(desktopMode)
	if _, port, err := net.SplitHostPort(addr); err == nil && port == "0" && policy.Method == "binary" {
		policy.Method = "manual"
		policy.Reason = "动态端口服务请手动更新并重新连接"
	}
	return policy
}
