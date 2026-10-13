package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultGitHubRepo = "liuzhuogood/MyWebScrcpy"
	githubAPIBase     = "https://api.github.com/repos"
)

// ReleaseAsset 代表 GitHub Release 中的一个资源
type ReleaseAsset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

// ReleaseInfo 代表 GitHub Release 的精简信息
type ReleaseInfo struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	Body        string         `json:"body"`
	PublishedAt time.Time      `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`
}

// CheckResult 是检查更新返回给前端的数据结构
type CheckResult struct {
	Policy
	CanUpgrade     bool   `json:"can_upgrade"`
	Digest         string `json:"digest"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
	ReleaseNotes   string `json:"release_notes"`
	PublishedAt    string `json:"published_at"`
	AssetName      string `json:"asset_name"`
	AssetSize      int64  `json:"asset_size"`
	DownloadURL    string `json:"download_url"`
}

// Checker 负责检查 GitHub Release 更新
type Checker struct {
	repo           string
	currentVersion string
	httpClient     *http.Client
}

// NewChecker 创建 Checker 实例
func NewChecker(currentVersion string) *Checker {
	v := strings.TrimSpace(currentVersion)
	if v == "" {
		// 尝试从 VERSION 文件读取
		if data, err := os.ReadFile("VERSION"); err == nil {
			v = strings.TrimSpace(string(data))
		}
	}
	if v == "" {
		v = "v0.0.0"
	}
	return &Checker{
		repo:           defaultGitHubRepo,
		currentVersion: v,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// SetRepo 自定义仓库，方便测试
func (c *Checker) SetRepo(repo string) {
	c.repo = repo
}

// GetCurrentVersion 获取当前版本号
func (c *Checker) GetCurrentVersion() string {
	return c.currentVersion
}

// CheckUpdate 查询 GitHub 最新 Release 并对比版本
func (c *Checker) CheckUpdate() (*CheckResult, error) {
	url := fmt.Sprintf("%s/%s/releases/latest", githubAPIBase, c.repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "MyWebScrcpy-Updater")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release info failed: %w", err)
	}

	hasUpdate := CompareVersions(release.TagName, c.currentVersion) > 0

	// 匹配当前平台适合的 asset
	targetAsset := MatchAsset(release.Assets, runtime.GOOS, runtime.GOARCH)

	result := &CheckResult{
		CurrentVersion: c.currentVersion,
		LatestVersion:  release.TagName,
		HasUpdate:      hasUpdate,
		ReleaseNotes:   release.Body,
		PublishedAt:    release.PublishedAt.Format(time.RFC3339),
	}

	if targetAsset != nil {
		result.AssetName = targetAsset.Name
		result.AssetSize = targetAsset.Size
		result.DownloadURL = targetAsset.BrowserDownloadURL
		result.Digest = targetAsset.Digest
	}

	return result, nil
}

// MatchAsset 根据操作系统和架构匹配 GitHub Release 中的对应文件名
func MatchAsset(assets []ReleaseAsset, goos, goarch string) *ReleaseAsset {
	var expectedSuffix string
	switch {
	case goos == "linux" && goarch == "amd64":
		expectedSuffix = "linux-amd64"
	case goos == "linux" && goarch == "arm64":
		expectedSuffix = "linux-arm64"
	case goos == "darwin" && goarch == "amd64":
		expectedSuffix = "macos-amd64"
	case goos == "darwin" && goarch == "arm64":
		expectedSuffix = "macos-arm64"
	case goos == "windows" && goarch == "amd64":
		expectedSuffix = "windows-amd64.exe"
	default:
		expectedSuffix = fmt.Sprintf("%s-%s", goos, goarch)
	}

	cliSuffix := expectedSuffix + "-cli"
	if goos == "windows" && goarch == "amd64" {
		cliSuffix = "windows-cli-amd64.exe"
	}
	// Prefer the CLI distribution. The exact historical name remains supported
	// for releases published before CLI and GUI assets were separated.
	for _, name := range []string{"mywebscrcpy-" + cliSuffix, "mywebscrcpy-" + expectedSuffix} {
		for _, a := range assets {
			if a.Name == name {
				return &a
			}
		}
	}
	return nil
}

// CompareVersions 比较两个版本号 (如 v1.12.0 和 v1.13.0)
// 返回 1 表示 v1 > v2; -1 表示 v1 < v2; 0 表示相等
func CompareVersions(v1, v2 string) int {
	clean1 := strings.TrimPrefix(strings.TrimSpace(v1), "v")
	clean2 := strings.TrimPrefix(strings.TrimSpace(v2), "v")

	parts1 := parseVersionParts(clean1)
	parts2 := parseVersionParts(clean2)

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		p1 := 0
		if i < len(parts1) {
			p1 = parts1[i]
		}
		p2 := 0
		if i < len(parts2) {
			p2 = parts2[i]
		}

		if p1 > p2 {
			return 1
		} else if p1 < p2 {
			return -1
		}
	}

	return 0
}

func parseVersionParts(v string) []int {
	// 去除 prerelease / metadata，例如 1.13.0-rc1 -> 1.13.0
	mainPart := strings.Split(v, "-")[0]
	mainPart = strings.Split(mainPart, "+")[0]
	strParts := strings.Split(mainPart, ".")
	var parts []int
	for _, p := range strParts {
		n, err := strconv.Atoi(p)
		if err != nil {
			parts = append(parts, 0)
		} else {
			parts = append(parts, n)
		}
	}
	return parts
}
