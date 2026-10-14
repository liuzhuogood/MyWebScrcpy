package dependencies

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// The locked standalone archives contain relative file links. Materialize only
// links to verified files inside this archive; the installed tree has no links.
func extractPythonArchive(ctx context.Context, archive, dest string, files map[string]installedFile) error {
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
	links := map[string]string{}
	seen := map[string]bool{}
	var expanded int64
	write := func(name string, size int64, mode fs.FileMode, reader io.Reader) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		expanded += size
		if size < 0 || size > 256<<20 || expanded > 1<<30 {
			return fmt.Errorf("Python 解包大小超过限制")
		}
		p := filepath.Join(dest, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return err
		}
		out, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return err
		}
		if err := out.Chmod(mode); err != nil {
			out.Close()
			return err
		}
		h := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(out, h), io.LimitReader(reader, size+1))
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if n != size {
			return fmt.Errorf("Python 解包长度不正确")
		}
		files[name] = installedFile{SHA256: hex.EncodeToString(h.Sum(nil)), Mode: mode}
		return nil
	}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		name := strings.TrimSuffix(h.Name, "/")
		if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:") || (name != "python" && !strings.HasPrefix(name, "python/")) {
			return fmt.Errorf("Python 压缩包路径无效")
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		name = strings.TrimPrefix(name, "python/")
		if !fs.ValidPath(name) || name == "." || name == "python" || seen[name] || len(seen) >= 20000 {
			return fmt.Errorf("Python 压缩包文件无效或重复")
		}
		seen[name] = true
		// Relocating a runtime regenerates bytecode caches. They are mutable
		// derived files, so never include upstream .pyc files in the integrity map.
		if strings.HasSuffix(name, ".pyc") {
			expanded += h.Size
			if h.Size < 0 || h.Size > 256<<20 || expanded > 1<<30 {
				return fmt.Errorf("Python 解包大小超过限制")
			}
			continue
		}
		mode := fs.FileMode(0644)
		if h.Mode&0111 != 0 {
			mode = 0755
		}
		switch h.Typeflag {
		case tar.TypeReg:
			if err := write(name, h.Size, mode, tr); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			if strings.ContainsAny(h.Linkname, "\\:") || path.IsAbs(h.Linkname) {
				return fmt.Errorf("Python 压缩包链接无效")
			}
			target := path.Clean(path.Join(path.Dir(name), h.Linkname))
			if h.Typeflag == tar.TypeLink {
				if !strings.HasPrefix(h.Linkname, "python/") {
					return fmt.Errorf("Python 压缩包硬链接无效")
				}
				target = strings.TrimPrefix(h.Linkname, "python/")
			}
			if !fs.ValidPath(target) || target == "." {
				return fmt.Errorf("Python 压缩包链接越界")
			}
			links[name] = target
		default:
			return fmt.Errorf("Python 压缩包包含特殊文件")
		}
	}
	for len(links) > 0 {
		progress := false
		for name, target := range links {
			rec, ok := files[target]
			if !ok {
				continue
			}
			p := filepath.Join(dest, filepath.FromSlash(target))
			info, err := os.Stat(p)
			if err != nil {
				return err
			}
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			err = write(name, info.Size(), rec.Mode, in)
			in.Close()
			if err != nil {
				return err
			}
			delete(links, name)
			progress = true
		}
		if !progress {
			return fmt.Errorf("Python 压缩包链接缺少目标或存在循环")
		}
	}
	return nil
}
