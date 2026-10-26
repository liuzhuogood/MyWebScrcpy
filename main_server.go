package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"mywebscrcpy/internal/desktop"
	"mywebscrcpy/internal/updater"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var updateStop = make(chan struct{})

func serveApplication(addr string, httpsFlag *bool, readyFlag *string, mux *http.ServeMux, adbPath string) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		log.Fatalf("监听地址错误: %v", err)
	}
	log.Printf("adb 路径: %s", adbPath)

	if *readyFlag != "" {
		if *httpsFlag || os.Getenv("TLS_CERT") != "" || os.Getenv("TLS_KEY") != "" {
			log.Fatal("桌面内部服务只使用回环 HTTP")
		}
		if err := desktop.ServeDesktopWithMaintenance(addr, *readyFlag, mux, updateMiddleware(deviceActivityMiddleware(mux)), updatePrepare, updateResume); err != nil {
			log.Fatalf("桌面服务失败: %v", err)
		}
		return
	}

	// TLS 配置
	tlsCertFile := os.Getenv("TLS_CERT")
	tlsKeyFile := os.Getenv("TLS_KEY")

	if *httpsFlag || (tlsCertFile != "" && tlsKeyFile != "") {
		var cert tls.Certificate
		var err error

		if tlsCertFile != "" && tlsKeyFile != "" {
			// 使用自定义证书
			cert, err = tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
			if err != nil {
				log.Fatalf("加载自定义证书失败: %v", err)
			}
			log.Printf("使用自定义证书: %s", tlsCertFile)
		} else {
			// 使用内置证书
			cert, err = tls.X509KeyPair(defaultCert, defaultKey)
			if err != nil {
				log.Fatalf("加载内置证书失败: %v", err)
			}
			log.Printf("使用内置证书（自签名）")
		}

		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		listener, err := tls.Listen("tcp", addr, tlsConfig)
		if err != nil {
			log.Fatalf("TLS 监听失败: %v", err)
		}
		log.Printf("MyWebScrcpy 启动 → https://localhost:%s (%s)", port, addr)
		if err := serveManaged(listener, mux, true); err != nil {
			log.Fatalf("server: %v", err)
		}
	} else {
		log.Printf("MyWebScrcpy 启动 → http://localhost:%s (%s)", port, addr)
		log.Printf("提示: 使用 -https 参数启用 HTTPS，或设置 TLS_CERT/TLS_KEY 环境变量")
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("监听失败: %v", err)
		}
		if err := serveManaged(listener, mux, false); err != nil {
			log.Fatalf("server: %v", err)
		}
	}

}

func serveManaged(listener net.Listener, mux *http.ServeMux, secure bool) error {
	server := &http.Server{Handler: updateMiddleware(deviceActivityMiddleware(mux)), ReadHeaderTimeout: 10 * time.Second}
	if err := updater.PublishHealth(listener, secure, strings.TrimSpace(string(versionBytes))); err != nil {
		return err
	}
	stopContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	go func() {
		select {
		case <-updateStop:
		case <-stopContext.Done():
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			server.Close()
		}
	}()
	err := server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
