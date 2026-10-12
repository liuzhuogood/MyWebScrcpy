package desktop

import (
	"context"
	"errors"
	"io"
	"log"
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
	control, err := NewControl()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	control.Register(mux)
	if err = control.Publish(readyPath, listener); err != nil {
		return err
	}
	defer os.Remove(readyPath)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
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
