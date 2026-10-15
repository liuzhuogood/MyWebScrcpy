package dependencies

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func platform() string { return runtime.GOOS + "/" + runtime.GOARCH }
func extractArchive(ctx context.Context, archive, sourceURL, dest, id, component string, files map[string]installedFile) error {
	if id == "python" {
		return extractPythonArchive(ctx, archive, dest, files)
	}
	if id != "adb" {
		return fmt.Errorf("未知依赖")
	}
	var expanded int64
	write := func(name string, size int64, reader io.Reader) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:") || name == "." {
			return fmt.Errorf("压缩包路径无效: %q", name)
		}
		relative := name
		if id == "adb" {
			if !strings.HasPrefix(name, "platform-tools/") {
				return fmt.Errorf("ADB 压缩包根目录无效")
			}
			relative = strings.TrimPrefix(name, "platform-tools/")

		}
		if !fs.ValidPath(relative) || relative == "." {
			return fmt.Errorf("依赖文件路径无效")
		}
		if _, ok := files[relative]; ok {
			return fmt.Errorf("压缩包包含重复文件: %s", relative)
		}
		expanded += size
		if size < 0 || size > 256<<20 || expanded > 1<<30 || len(files) > 20000 {
			return fmt.Errorf("解包大小超过限制")
		}
		mode := fs.FileMode(0644)
		if relative == exeName("adb") || relative == exeName("fastboot") {
			mode = 0755
		}
		p := filepath.Join(dest, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return err
		}
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return err
		}
		if err = f.Chmod(mode); err != nil {
			f.Close()
			return err
		}
		h := sha256.New()
		n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(reader, size+1))
		ce := f.Close()
		if err != nil {
			return err
		}
		if ce != nil {
			return ce
		}
		if n != size {
			return fmt.Errorf("解包长度不正确")
		}
		files[relative] = installedFile{SHA256: hex.EncodeToString(h.Sum(nil)), Mode: mode}
		return nil
	}
	u, err := url.Parse(sourceURL)
	if err != nil {
		return err
	}
	if strings.HasSuffix(u.Path, ".zip") {
		z, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer z.Close()
		for _, f := range z.File {
			name := strings.TrimSuffix(f.Name, "/")
			if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:") {
				return fmt.Errorf("压缩包路径无效")
			}
			if f.FileInfo().IsDir() {
				continue
			}
			if !f.Mode().IsRegular() {
				return fmt.Errorf("拒绝压缩包链接或特殊文件")
			}
			r, e := f.Open()
			if e != nil {
				return e
			}
			e = write(f.Name, int64(f.UncompressedSize64), r)
			r.Close()
			if e != nil {
				return e
			}
		}
		return nil
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(h.Name, "/")
		if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:") {
			return fmt.Errorf("压缩包路径无效")
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		if h.Typeflag != tar.TypeReg {
			return fmt.Errorf("拒绝压缩包链接或特殊文件")
		}
		if err := write(h.Name, h.Size, tr); err != nil {
			return err
		}
	}
}
