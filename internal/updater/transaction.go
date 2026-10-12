package updater

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var workerHealthTimeout = 30 * time.Second

type updatePlan struct {
	Target, Candidate, Version, Directory, Stopped, Receipt, Status string
	Args                                                            []string
}

func validDigest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") {
		return false
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && len(decoded) == sha256.Size
}
func verifyFile(path, digest string) error {
	if !validDigest(digest) {
		return fmt.Errorf("缺少 SHA-256 校验值")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		return err
	}
	if "sha256:"+hex.EncodeToString(hash.Sum(nil)) != digest {
		return fmt.Errorf("更新文件 SHA-256 不一致")
	}
	return nil
}
func (s *Service) stageUpgrade(check *CheckResult) error {
	parsed, err := url.Parse(check.DownloadURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" || !strings.HasPrefix(parsed.Path, "/"+s.checker.repo+"/releases/download/"+check.LatestVersion+"/") {
		return fmt.Errorf("更新下载地址不属于已配置仓库的当前发布")
	}
	// Stage on the destination filesystem so installation does not cross volumes.
	dir, err := os.MkdirTemp(filepath.Dir(s.applier.executablePath), ".mywebscrcpy-update-")
	if err != nil {
		return fmt.Errorf("安装目录不可写: %w", err)
	}
	keep := false
	defer func() {
		if !keep {
			os.RemoveAll(dir)
		}
	}()
	candidate := filepath.Join(dir, filepath.Base(s.applier.executablePath))
	if err = s.downloader.DownloadAsset(check.DownloadURL, check.AssetSize, candidate); err != nil {
		return err
	}
	if err = verifyFile(candidate, check.Digest); err != nil {
		return err
	}
	if err = os.Chmod(candidate, 0755); err != nil {
		return err
	}
	ctxOutput, err := candidateVersion(candidate)
	if err != nil || ctxOutput != check.LatestVersion {
		return fmt.Errorf("新版程序的平台或版本验证失败")
	}
	if err = s.prepare(); err != nil {
		return err
	}
	defer func() {
		if !keep && s.resume != nil {
			s.resume()
		}
	}()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	plan := updatePlan{Target: s.applier.executablePath, Candidate: candidate, Version: check.LatestVersion, Directory: cwd, Args: restartArgs(os.Args[1:], cwd), Stopped: filepath.Join(dir, "stopped"), Receipt: filepath.Join(dir, "ready.json"), Status: filepath.Join(cwd, "data", "updater-status.json")}
	planFile := filepath.Join(dir, "plan.json")
	if err = writeJSON(planFile, plan); err != nil {
		return err
	}
	// Copy the old executable to an independent helper, including on Windows.
	helper := filepath.Join(dir, "worker"+filepath.Ext(s.applier.executablePath))
	if err = copyFile(s.applier.executablePath, helper); err != nil {
		return err
	}
	cmd := exec.Command(helper, "--update-worker", planFile)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err = cmd.Start(); err != nil {
		return err
	}
	keep = true
	s.downloader.notify(ProgressInfo{State: StateApplying, Percent: 100})
	log.Printf("[Updater] 已校验 %s，开始受控重启", check.LatestVersion)
	SetExitMarker(plan.Stopped)
	s.stop()
	return nil
}

var exitMarker string

func SetExitMarker(path string) { exitMarker = path }

// MarkExited is deferred before application resources, and runs after cleanup.
func MarkExited() {
	if exitMarker != "" {
		_ = os.WriteFile(exitMarker, []byte("stopped"), 0600)
	}
}
func candidateVersion(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "--version").Output()
	return strings.TrimSpace(string(output)), err
}

func copyFile(source, dest string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func writeJSON(path string, value any) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temp := path + ".tmp"
	if err = os.WriteFile(temp, bytes, 0600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
func waitFile(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fileExists(path) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// RunWorker performs an offline DB backup and supervises the first new startup.
func RunWorker(path string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var plan updatePlan
	if err = json.Unmarshal(bytes, &plan); err != nil {
		return err
	}
	progress := func(state ProgressState, message string) {
		_ = writeJSON(plan.Status, ProgressInfo{State: state, Percent: 100, Error: message, UpdatedAt: time.Now()})
	}
	progress(StateApplying, "")
	if !waitFile(plan.Stopped, 30*time.Second) {
		err = fmt.Errorf("旧进程未完成退出，取消安装")
		progress(StateError, err.Error())
		return err
	}
	// The DB is closed at this point: backup main file plus WAL/SHM without guessing migration compatibility.
	backups := map[string]string{}
	for _, name := range []string{"data/mywebscrcpy.db", "data/mywebscrcpy.db-wal", "data/mywebscrcpy.db-shm", "dependencies.json"} {
		source := filepath.Join(plan.Directory, name)
		if fileExists(source) {
			backup := filepath.Join(filepath.Dir(path), filepath.Base(name)+".backup")
			if err = copyFile(source, backup); err != nil {
				progress(StateError, err.Error())
				startApplication(plan, false)
				return err
			}
			backups[source] = backup
		}
	}
	applier := &Applier{executablePath: plan.Target}
	err = applier.ApplyUpdate(plan.Candidate)
	if runtime.GOOS == "windows" {
		deadline := time.Now().Add(5 * time.Second)
		for err != nil && os.IsPermission(err) && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
			err = applier.ApplyUpdate(plan.Candidate)
		}
	}
	if err != nil {
		progress(StateError, err.Error())
		startApplication(plan, false)
		return err
	}
	child, err := startApplication(plan, true)
	if err == nil {
		err = waitHealthy(plan, workerHealthTimeout)
	}
	if err == nil {
		if err = os.WriteFile(plan.Receipt+".confirmed", []byte("confirmed"), 0600); err != nil {
			progress(StateError, err.Error())
			return err
		}
		progress(StateSuccess, "")
		log.Printf("[Updater] %s 启动验证通过", plan.Version)
		return nil
	}
	if child != nil && child.Process != nil {
		_ = child.Process.Kill()
		_ = child.Wait()
	}
	log.Printf("[Updater] 新版启动失败，恢复旧版本: %v", err)
	_ = os.Remove(plan.Target)
	rollbackErr := os.Rename(plan.Target+".old", plan.Target)
	if rollbackErr == nil {
		for _, name := range []string{"data/mywebscrcpy.db", "data/mywebscrcpy.db-wal", "data/mywebscrcpy.db-shm", "dependencies.json"} {
			source := filepath.Join(plan.Directory, name)
			if removeErr := os.Remove(source); removeErr != nil && !os.IsNotExist(removeErr) {
				rollbackErr = removeErr
				break
			}
			if backup, ok := backups[source]; ok {
				if rollbackErr = copyFile(backup, source); rollbackErr != nil {
					break
				}
			}
		}
	}
	if rollbackErr != nil {
		err = fmt.Errorf("%v；回滚失败: %w", err, rollbackErr)
	} else {
		_, startErr := startApplication(plan, false)
		if startErr != nil {
			err = fmt.Errorf("%v；旧版本启动失败: %w", err, startErr)
		}
	}
	progress(StateError, err.Error())
	return err
}
func startApplication(plan updatePlan, verify bool) (*exec.Cmd, error) {
	cmd := exec.Command(plan.Target, plan.Args...)
	cmd.Dir = plan.Directory
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "MYWEBSCRCPY_UPDATE_RECEIPT=") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	// Worker process must not retain caller stdio pipes; the application owns its log.
	f, err := os.OpenFile(filepath.Join(plan.Directory, "data", "updater.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cmd.Stdout, cmd.Stderr = f, f
	if verify {
		cmd.Env = append(cmd.Env, "MYWEBSCRCPY_UPDATE_RECEIPT="+plan.Receipt)
	}
	err = cmd.Start()
	return cmd, err
}

type healthReceipt struct{ URL, Version string }

// PublishHealth records the actual listener, and the worker also probes HTTP.
func PublishHealth(listener net.Listener, secure bool, version string) error {
	path := os.Getenv("MYWEBSCRCPY_UPDATE_RECEIPT")
	if path == "" {
		return nil
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return err
	}
	scheme := "http"
	if secure {
		scheme = "https"
	}
	return writeJSON(path, healthReceipt{scheme + "://127.0.0.1:" + port + "/api/version/health", version})
}
func waitHealthy(plan updatePlan, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	// This probe is loopback-only and supports the application's existing self-signed TLS.
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	successes := 0
	for time.Now().Before(deadline) {
		bytes, err := os.ReadFile(plan.Receipt)
		var receipt healthReceipt
		if err == nil && json.Unmarshal(bytes, &receipt) == nil && receipt.Version == plan.Version && (strings.HasPrefix(receipt.URL, "http://127.0.0.1:") || strings.HasPrefix(receipt.URL, "https://127.0.0.1:")) {
			response, err := client.Get(receipt.URL)
			if err == nil {
				var health struct {
					Version string `json:"version"`
				}
				decodeErr := json.NewDecoder(response.Body).Decode(&health)
				response.Body.Close()
				if response.StatusCode == 200 && decodeErr == nil && health.Version == plan.Version {
					successes++
					if successes >= 3 {
						return nil
					}
				} else {
					successes = 0
				}
			} else {
				successes = 0
			}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("新版未在规定时间内通过健康检查")
}

// The original service may have already changed cwd for a relative --data-dir.
func restartArgs(args []string, dataDir string) []string {
	result := append([]string(nil), args...)
	for i, arg := range result {
		if arg == "--data-dir" || arg == "-data-dir" {
			if i+1 < len(result) {
				result[i+1] = dataDir
			}
			continue
		}
		if strings.HasPrefix(arg, "--data-dir=") {
			result[i] = "--data-dir=" + dataDir
		}
		if strings.HasPrefix(arg, "-data-dir=") {
			result[i] = "-data-dir=" + dataDir
		}
	}
	return result
}
