package desktop

import (
	"context"
	"errors"
	"io"
	"log"
	"mywebscrcpy/internal/updater"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ServeDesktop owns a loopback-only server. Closing the parent's stdin pipe also
// shuts it down, including when the GUI host crashes or is terminated.
func ServeDesktop(addr, readyPath string, mux *http.ServeMux) error {
	return ServeDesktopWithMaintenance(addr, readyPath, mux, mux, nil, nil)
}

func ServeDesktopWithMaintenance(addr, readyPath string, mux *http.ServeMux, handler http.Handler, prepare func() error, resume func()) error {
	control, err := NewControl()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[Desktop] 端口 %s 监听失败: %v（可能已被占用，请在系统设置中修改端口或释放该端口）", addr, err)
		return err
	}
	defer listener.Close()
	control.Register(mux)
	if prepare != nil {
		mux.HandleFunc("POST /api/desktop/update-prepare", control.authorize(func(w http.ResponseWriter, r *http.Request) {
			if err := prepare(); err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			w.WriteHeader(http.StatusAccepted)
		}))
		mux.HandleFunc("POST /api/desktop/update-resume", control.authorize(func(w http.ResponseWriter, r *http.Request) { resume(); w.WriteHeader(http.StatusAccepted) }))
	}
	if err = control.Publish(readyPath, listener); err != nil {
		return err
	}
	defer os.Remove(readyPath)
	defer os.Remove(EndpointFile)
	if err = updater.PublishHealth(listener, false, "v"+os.Getenv("MYWEBSCRCPY_DESKTOP_VERSION")); err != nil {
		return err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); control.once.Do(func() { close(control.stop) }) }()
	go func() {
		select {
		case <-control.Done():
		case <-ctx.Done():
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("[Desktop] 服务退出失败: %v", err)
			server.Close()
		}
	}()
	log.Printf("[Desktop] 服务就绪: http://%s", listener.Addr())
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
