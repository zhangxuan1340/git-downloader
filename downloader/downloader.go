package downloader

import (
    "bufio"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "time"

    "github.com/schollz/progressbar/v3"
    "github-downloader/logger"
)

const (
    defaultProxy   = "gh-proxy.com" // 当代理列表为空时的最终默认值
    maxRetries     = 3
    retryDelay     = 5 * time.Second
    githubAPI      = "https://api.github.com/repos/%s/%s/releases/latest"
    githubAPIAll   = "https://api.github.com/repos/%s/%s/releases"
    gitlabAPI      = "https://git.ryujinx.app/api/v4/projects/%s%%2F%s/releases/%s"
    gitlabAPIAll   = "https://git.ryujinx.app/api/v4/projects/%s%%2F%s/releases"
)

// Asset 表示 release 中的一个资产
type Asset struct {
    Name               string `json:"name"`
    Size               int64  `json:"size"`
    BrowserDownloadURL string `json:"browser_download_url"`
    Digest             string `json:"digest,omitempty"` // 非标准字段，可能不存在
}

// GitLabAsset 表示 GitLab release 中的一个资产
type GitLabAsset struct {
    Name        string `json:"name"`
    Size        int64  `json:"size"`
    DownloadURL string `json:"download_url"`
    URL         string `json:"url"`
}

// Release 表示 GitHub release 信息
type Release struct {
    TagName string  `json:"tag_name"`
    Body    string  `json:"body"`
    Assets  []Asset `json:"assets"`
}

// GitLabRelease 表示 GitLab release 信息
type GitLabRelease struct {
    TagName     string        `json:"tag_name"`
    Description string        `json:"description"`
    Assets      struct {
        Links []GitLabAsset `json:"links"`
    } `json:"assets"`
}

// Downloader 处理下载逻辑
type Downloader struct {
    topDir     string
    proxies    []string // 全局代理列表
    client     *http.Client
    userAgent  string
}

// NewDownloader 创建下载器
func NewDownloader(topDir string, proxies []string) *Downloader {
    return &Downloader{
        topDir:  topDir,
        proxies: proxies,
        client: &http.Client{
            Timeout: 300 * time.Second,
        },
        userAgent: "Mozilla/5.0 (compatible; GithubDownloader/1.0)",
    }
}

// ProcessRepo 处理单个仓库（仅最新版本）
func (d *Downloader) ProcessRepo(owner, repo, specifiedProxy string) error {
    logger.Info("========================================")
    logger.Info("开始处理仓库: %s/%s", owner, repo)

    // 1. 获取最新 release 信息
    release, err := d.fetchLatestRelease(owner, repo)
    if err != nil {
        logger.Error("获取 release 失败: %v", err)
        return err
    }
    if release.TagName == "" {
        logger.Warn("仓库 %s/%s 没有可用的 Release", owner, repo)
        return nil
    }

    logger.Info("当前版本: %s", release.TagName)

    // 2. 创建版本目录
    versionDir := filepath.Join(d.topDir, repo, release.TagName)
    if err := os.MkdirAll(versionDir, 0755); err != nil {
        logger.Error("无法创建目录 %s: %v", versionDir, err)
        return err
    }

    // 3. 保存 release notes
    notesFile := filepath.Join(versionDir, "release_notes.txt")
    notesContent := release.Body
    if notesContent == "" {
        notesContent = "No release notes provided"
    }
    if err := os.WriteFile(notesFile, []byte(notesContent), 0644); err != nil {
        logger.Warn("无法写入 release notes: %v", err)
    } else {
        logger.Info("Release 日志已保存到: %s", notesFile)
    }

    // 4. 处理每个资产
    var downloadedFiles []string
    for _, asset := range release.Assets {
        // 提取 SHA256（如果存在）
        sha256 := extractSHA256(asset.Digest)
        if sha256 != "" {
            logger.Info("官方 SHA256: %s", sha256)
        } else {
            logger.Info("没有可用的官方 SHA256 哈希值")
        }

        // 下载文件（使用代理列表）
        localPath := filepath.Join(versionDir, asset.Name)
        if err := d.downloadFileWithProxyList(asset.BrowserDownloadURL, localPath, asset.Size, sha256, specifiedProxy); err != nil {
            logger.Error("下载 %s 失败: %v", asset.Name, err)
            continue
        }
        downloadedFiles = append(downloadedFiles, localPath)
        logger.Info("完成下载: %s", asset.Name)
    }

    // 5. 校验文件
    if err := d.verifyFiles(versionDir, release, downloadedFiles); err != nil {
        logger.Error("校验失败: %v", err)
        return err
    }

    logger.Info("仓库 %s/%s 处理完成", owner, repo)
    return nil
}

// ProcessRepoAll 处理单个仓库的所有版本
func (d *Downloader) ProcessRepoAll(owner, repo, specifiedProxy string) error {
    logger.Info("========================================")
    logger.Info("开始处理仓库: %s/%s（所有版本）", owner, repo)

    // 1. 获取所有 release 信息
    releases, err := d.fetchAllReleases(owner, repo)
    if err != nil {
        logger.Error("获取所有 release 失败: %v", err)
        return err
    }
    if len(releases) == 0 {
        logger.Warn("仓库 %s/%s 没有可用的 Release", owner, repo)
        return nil
    }

    logger.Info("找到 %d 个 Release 版本", len(releases))

    // 2. 遍历处理每个 release
    for i, release := range releases {
        if release.TagName == "" {
            logger.Warn("跳过空版本号的 Release")
            continue
        }

        logger.Info("========================================")
        logger.Info("处理版本 %d/%d: %s", i+1, len(releases), release.TagName)

        // 3. 创建版本目录
        versionDir := filepath.Join(d.topDir, repo, release.TagName)
        if err := os.MkdirAll(versionDir, 0755); err != nil {
            logger.Error("无法创建目录 %s: %v", versionDir, err)
            continue
        }

        // 4. 保存 release notes
        notesFile := filepath.Join(versionDir, "release_notes.txt")
        notesContent := release.Body
        if notesContent == "" {
            notesContent = "No release notes provided"
        }
        if err := os.WriteFile(notesFile, []byte(notesContent), 0644); err != nil {
            logger.Warn("无法写入 release notes: %v", err)
        } else {
            logger.Info("Release 日志已保存到: %s", notesFile)
        }

        // 5. 处理每个资产
        var downloadedFiles []string
        for _, asset := range release.Assets {
            // 提取 SHA256（如果存在）
            sha256 := extractSHA256(asset.Digest)
            if sha256 != "" {
                logger.Info("官方 SHA256: %s", sha256)
            } else {
                logger.Info("没有可用的官方 SHA256 哈希值")
            }

            // 下载文件（使用代理列表）
            localPath := filepath.Join(versionDir, asset.Name)
            if err := d.downloadFileWithProxyList(asset.BrowserDownloadURL, localPath, asset.Size, sha256, specifiedProxy); err != nil {
                logger.Error("下载 %s 失败: %v", asset.Name, err)
                continue
            }
            downloadedFiles = append(downloadedFiles, localPath)
            logger.Info("完成下载: %s", asset.Name)
        }

        // 6. 校验文件
        if err := d.verifyFiles(versionDir, release, downloadedFiles); err != nil {
            logger.Error("校验失败: %v", err)
            continue
        }

        logger.Info("版本 %s 处理完成", release.TagName)
    }

    logger.Info("仓库 %s/%s 所有版本处理完成", owner, repo)
    return nil
}

// ProcessGitLabRepo 处理单个 GitLab 仓库（仅最新版本）
func (d *Downloader) ProcessGitLabRepo(owner, repo, specifiedProxy string) error {
    logger.Info("========================================")
    logger.Info("开始处理 GitLab 仓库: %s/%s", owner, repo)

    // 1. 获取最新 release 信息
    release, err := d.fetchGitLabLatestRelease(owner, repo)
    if err != nil {
        logger.Error("获取 release 失败: %v", err)
        return err
    }
    if release.TagName == "" {
        logger.Warn("仓库 %s/%s 没有可用的 Release", owner, repo)
        return nil
    }

    logger.Info("当前版本: %s", release.TagName)

    // 2. 创建版本目录
    versionDir := filepath.Join(d.topDir, repo, release.TagName)
    if err := os.MkdirAll(versionDir, 0755); err != nil {
        logger.Error("无法创建目录 %s: %v", versionDir, err)
        return err
    }

    // 3. 保存 release notes
    notesFile := filepath.Join(versionDir, "release_notes.txt")
    notesContent := release.Body
    if notesContent == "" {
        notesContent = "No release notes provided"
    }
    if err := os.WriteFile(notesFile, []byte(notesContent), 0644); err != nil {
        logger.Warn("无法写入 release notes: %v", err)
    } else {
        logger.Info("Release 日志已保存到: %s", notesFile)
    }

    // 4. 处理每个资产
    var downloadedFiles []string
    for _, asset := range release.Assets {
        // GitLab 资产没有官方 SHA256
        logger.Info("没有可用的官方 SHA256 哈希值")

        // 下载文件（使用代理列表）
        localPath := filepath.Join(versionDir, asset.Name)
        if err := d.downloadFileWithProxyList(asset.BrowserDownloadURL, localPath, asset.Size, "", specifiedProxy); err != nil {
            logger.Error("下载 %s 失败: %v", asset.Name, err)
            continue
        }
        downloadedFiles = append(downloadedFiles, localPath)
        logger.Info("完成下载: %s", asset.Name)
    }

    // 5. 校验文件
    if err := d.verifyFiles(versionDir, release, downloadedFiles); err != nil {
        logger.Error("校验失败: %v", err)
        return err
    }

    logger.Info("仓库 %s/%s 处理完成", owner, repo)
    return nil
}

// ProcessGitLabRepoAll 处理单个 GitLab 仓库的所有版本
func (d *Downloader) ProcessGitLabRepoAll(owner, repo, specifiedProxy string) error {
    logger.Info("========================================")
    logger.Info("开始处理 GitLab 仓库: %s/%s（所有版本）", owner, repo)

    // 1. 获取所有 release 信息
    releases, err := d.fetchGitLabAllReleases(owner, repo)
    if err != nil {
        logger.Error("获取所有 release 失败: %v", err)
        return err
    }
    if len(releases) == 0 {
        logger.Warn("仓库 %s/%s 没有可用的 Release", owner, repo)
        return nil
    }

    logger.Info("找到 %d 个 Release 版本", len(releases))

    // 2. 遍历处理每个 release
    for i, release := range releases {
        if release.TagName == "" {
            logger.Warn("跳过空版本号的 Release")
            continue
        }

        logger.Info("========================================")
        logger.Info("处理版本 %d/%d: %s", i+1, len(releases), release.TagName)

        // 3. 创建版本目录
        versionDir := filepath.Join(d.topDir, repo, release.TagName)
        if err := os.MkdirAll(versionDir, 0755); err != nil {
            logger.Error("无法创建目录 %s: %v", versionDir, err)
            continue
        }

        // 4. 保存 release notes
        notesFile := filepath.Join(versionDir, "release_notes.txt")
        notesContent := release.Body
        if notesContent == "" {
            notesContent = "No release notes provided"
        }
        if err := os.WriteFile(notesFile, []byte(notesContent), 0644); err != nil {
            logger.Warn("无法写入 release notes: %v", err)
        } else {
            logger.Info("Release 日志已保存到: %s", notesFile)
        }

        // 5. 处理每个资产
        var downloadedFiles []string
        for _, asset := range release.Assets {
            // GitLab 资产没有官方 SHA256
            logger.Info("没有可用的官方 SHA256 哈希值")

            // 下载文件（使用代理列表）
            localPath := filepath.Join(versionDir, asset.Name)
            if err := d.downloadFileWithProxyList(asset.BrowserDownloadURL, localPath, asset.Size, "", specifiedProxy); err != nil {
                logger.Error("下载 %s 失败: %v", asset.Name, err)
                continue
            }
            downloadedFiles = append(downloadedFiles, localPath)
            logger.Info("完成下载: %s", asset.Name)
        }

        // 6. 校验文件
        if err := d.verifyFiles(versionDir, release, downloadedFiles); err != nil {
            logger.Error("校验失败: %v", err)
            continue
        }

        logger.Info("版本 %s 处理完成", release.TagName)
    }

    logger.Info("GitLab 仓库 %s/%s 所有版本处理完成", owner, repo)
    return nil
}

// fetchLatestRelease 调用 GitHub API 获取最新 release
func (d *Downloader) fetchLatestRelease(owner, repo string) (*Release, error) {
    url := fmt.Sprintf(githubAPI, owner, repo)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("User-Agent", d.userAgent)
    // 可选的 GitHub Token：设置 Authorization: token YOUR_TOKEN

    resp, err := d.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API 返回状态码 %d", resp.StatusCode)
    }

    var release Release
    if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
        return nil, err
    }
    return &release, nil
}

// fetchAllReleases 调用 GitHub API 获取所有 release
func (d *Downloader) fetchAllReleases(owner, repo string) ([]*Release, error) {
    url := fmt.Sprintf(githubAPIAll, owner, repo)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("User-Agent", d.userAgent)
    // 可选的 GitHub Token：设置 Authorization: token YOUR_TOKEN

    resp, err := d.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API 返回状态码 %d", resp.StatusCode)
    }

    var releases []*Release
    if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
        return nil, err
    }
    return releases, nil
}

// fetchGitLabLatestRelease 调用 GitLab API 获取最新 release
func (d *Downloader) fetchGitLabLatestRelease(owner, repo string) (*Release, error) {
    url := fmt.Sprintf(gitlabAPIAll, owner, repo)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("User-Agent", d.userAgent)

    resp, err := d.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API 返回状态码 %d", resp.StatusCode)
    }

    var gitlabReleases []*GitLabRelease
    if err := json.NewDecoder(resp.Body).Decode(&gitlabReleases); err != nil {
        return nil, err
    }

    if len(gitlabReleases) == 0 {
        return nil, fmt.Errorf("仓库 %s/%s 没有可用的 Release", owner, repo)
    }

    // 转换为 GitHub Release 格式
    gitlabRelease := gitlabReleases[0]
    release := &Release{
        TagName: gitlabRelease.TagName,
        Body:    gitlabRelease.Description,
        Assets:  make([]Asset, 0),
    }

    logger.Info("GitLab 资产数量: %d", len(gitlabRelease.Assets.Links))
    for i, link := range gitlabRelease.Assets.Links {
        logger.Info("资产 %d: 名称=%s, 大小=%d, DownloadURL=%s, URL=%s", i, link.Name, link.Size, link.DownloadURL, link.URL)
        downloadURL := link.DownloadURL
        if downloadURL == "" {
            downloadURL = link.URL
        }
        asset := Asset{
            Name:               link.Name,
            Size:               link.Size,
            BrowserDownloadURL: downloadURL,
        }
        release.Assets = append(release.Assets, asset)
    }

    return release, nil
}

// fetchGitLabAllReleases 调用 GitLab API 获取所有 release
func (d *Downloader) fetchGitLabAllReleases(owner, repo string) ([]*Release, error) {
    url := fmt.Sprintf(gitlabAPIAll, owner, repo)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("User-Agent", d.userAgent)

    resp, err := d.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API 返回状态码 %d", resp.StatusCode)
    }

    var gitlabReleases []*GitLabRelease
    if err := json.NewDecoder(resp.Body).Decode(&gitlabReleases); err != nil {
        return nil, err
    }

    if len(gitlabReleases) == 0 {
        return nil, fmt.Errorf("仓库 %s/%s 没有可用的 Release", owner, repo)
    }

    // 转换为 GitHub Release 格式
    releases := make([]*Release, 0, len(gitlabReleases))
    for _, gitlabRelease := range gitlabReleases {
        release := &Release{
            TagName: gitlabRelease.TagName,
            Body:    gitlabRelease.Description,
            Assets:  make([]Asset, 0),
        }

        for _, link := range gitlabRelease.Assets.Links {
            asset := Asset{
                Name:               link.Name,
                Size:               link.Size,
                BrowserDownloadURL: link.DownloadURL,
            }
            release.Assets = append(release.Assets, asset)
        }

        releases = append(releases, release)
    }

    return releases, nil
}

// downloadFileWithProxyList 尝试使用代理列表下载，支持切换代理和进度条
func (d *Downloader) downloadFileWithProxyList(url, localPath string, expectedSize int64, expectedSHA256, specifiedProxy string) error {
    // 检查本地文件是否已存在且完整
    if info, err := os.Stat(localPath); err == nil {
        if info.Size() == expectedSize {
            if expectedSHA256 != "" {
                // 验证哈希
                if ok, err := verifySHA256(localPath, expectedSHA256); err == nil && ok {
                    logger.Info("文件已存在且哈希匹配: %s", filepath.Base(localPath))
                    return nil
                } else if err != nil {
                    logger.Warn("无法验证哈希: %v", err)
                } else {
                    logger.Warn("文件哈希不匹配，重新下载: %s", filepath.Base(localPath))
                }
            } else {
                // 没有哈希，仅大小匹配则视为完整
                logger.Info("文件已存在且大小匹配: %s", filepath.Base(localPath))
                return nil
            }
        } else {
            logger.Warn("文件大小不匹配，重新下载: %s", filepath.Base(localPath))
        }
        // 不匹配则删除旧文件重新下载
        os.Remove(localPath)
    }

    // 确定要尝试的代理列表
    var proxiesToTry []string
    if specifiedProxy != "" {
        // 如果仓库指定了代理，只尝试这个代理
        proxiesToTry = []string{specifiedProxy}
    } else {
        // 使用全局代理列表，如果列表为空则使用默认代理
        proxiesToTry = d.proxies
        if len(proxiesToTry) == 0 {
            proxiesToTry = []string{defaultProxy}
        }
    }

    logger.Info("开始下载: %s (大小: %s)", filepath.Base(localPath), byteCountIEC(expectedSize))

    // 检查是否是 GitLab 链接
    isGitLabURL := strings.Contains(url, "git.ryujinx.app") || strings.Contains(url, "gitlab.com")
    
    var lastErr error
    
    if isGitLabURL {
        // GitLab 链接不使用代理，直接下载
        logger.Info("GitLab 链接，直接下载: %s", filepath.Base(url))
        
        // 重试机制
        for attempt := 1; attempt <= maxRetries; attempt++ {
            err := d.downloadWithProgress(url, localPath+".tmp", expectedSize)
            if err != nil {
                lastErr = err
                logger.Warn("下载失败 (尝试 %d/%d): %v", attempt, maxRetries, err)
                time.Sleep(retryDelay)
                continue
            }

            // 重命名临时文件
            if err := os.Rename(localPath+".tmp", localPath); err != nil {
                lastErr = fmt.Errorf("重命名临时文件失败: %w", err)
                // 清理临时文件
                if _, err := os.Stat(localPath+".tmp"); err == nil {
                    os.Remove(localPath+".tmp")
                }
                break
            }

            // 验证大小
            info, err := os.Stat(localPath)
            if err != nil {
                lastErr = fmt.Errorf("无法获取下载文件信息: %w", err)
                // 清理已下载的文件
                os.Remove(localPath)
                break
            }
            if expectedSize > 0 && info.Size() != expectedSize {
                lastErr = fmt.Errorf("下载文件大小不匹配: 期望 %d, 实际 %d", expectedSize, info.Size())
                os.Remove(localPath)
                continue
            }

            // 验证哈希（如果提供）
            if expectedSHA256 != "" {
                ok, err := verifySHA256(localPath, expectedSHA256)
                if err != nil {
                    lastErr = fmt.Errorf("哈希验证失败: %w", err)
                    os.Remove(localPath)
                    break
                }
                if !ok {
                    lastErr = fmt.Errorf("哈希值不匹配")
                    os.Remove(localPath)
                    continue
                }
                logger.Info("✅ 文件哈希验证成功: %s", filepath.Base(localPath))
            } else {
                // 没有官方哈希值，验证文件大小
                logger.Info("✅ 文件大小验证成功: %s (%s)", filepath.Base(localPath), byteCountIEC(info.Size()))
            }

            // 成功
            return nil
        }
        
        return fmt.Errorf("下载失败: %w", lastErr)
    } else {
        // GitHub 链接使用代理列表
        for _, proxy := range proxiesToTry {
            // 构建代理 URL
            proxyURL := strings.Replace(url, "https://github.com", fmt.Sprintf("https://%s/github.com", proxy), 1)
            logger.Info("尝试使用代理: %s", proxy)

            // 重试机制（每个代理最多尝试 maxRetries 次）
            for attempt := 1; attempt <= maxRetries; attempt++ {
                err := d.downloadWithProgress(proxyURL, localPath+".tmp", expectedSize)
                if err != nil {
                    lastErr = err
                    logger.Warn("下载失败 (代理 %s, 尝试 %d/%d): %v", proxy, attempt, maxRetries, err)
                    time.Sleep(retryDelay)
                    continue
                }

                // 重命名临时文件
                if err := os.Rename(localPath+".tmp", localPath); err != nil {
                    lastErr = fmt.Errorf("重命名临时文件失败: %w", err)
                    // 清理临时文件
                    if _, err := os.Stat(localPath+".tmp"); err == nil {
                        os.Remove(localPath+".tmp")
                    }
                    break
                }

                // 验证大小
                info, err := os.Stat(localPath)
                if err != nil {
                    lastErr = fmt.Errorf("无法获取下载文件信息: %w", err)
                    // 清理已下载的文件
                    os.Remove(localPath)
                    break
                }
                if info.Size() != expectedSize {
                    lastErr = fmt.Errorf("下载文件大小不匹配: 期望 %d, 实际 %d", expectedSize, info.Size())
                    os.Remove(localPath)
                    continue
                }

                // 验证哈希（如果提供）
                if expectedSHA256 != "" {
                    ok, err := verifySHA256(localPath, expectedSHA256)
                    if err != nil {
                        lastErr = fmt.Errorf("哈希验证失败: %w", err)
                        os.Remove(localPath)
                        break
                    }
                    if !ok {
                        lastErr = fmt.Errorf("哈希值不匹配")
                        os.Remove(localPath)
                        continue
                    }
                    logger.Info("✅ 文件哈希验证成功: %s", filepath.Base(localPath))
                } else {
                    // 没有官方哈希值，再次验证文件大小
                    logger.Info("✅ 文件大小验证成功: %s (%s)", filepath.Base(localPath), byteCountIEC(info.Size()))
                }

                // 成功
                return nil
            }

            // 如果这个代理的所有重试都失败，继续尝试下一个代理
        }

        return fmt.Errorf("所有代理尝试均失败: %w", lastErr)
    }
}

// downloadWithProgress 下载文件并显示进度条
func (d *Downloader) downloadWithProgress(url, tmpPath string, expectedSize int64) error {
    // 创建临时文件
    out, err := os.Create(tmpPath)
    if err != nil {
        return err
    }
    defer out.Close()

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        // 下载失败，清理临时文件
        if _, err := os.Stat(tmpPath); err == nil {
            os.Remove(tmpPath)
        }
        return err
    }
    req.Header.Set("User-Agent", d.userAgent)

    resp, err := d.client.Do(req)
    if err != nil {
        // 下载失败，清理临时文件
        if _, err := os.Stat(tmpPath); err == nil {
            os.Remove(tmpPath)
        }
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        // 下载失败，清理临时文件
        if _, err := os.Stat(tmpPath); err == nil {
            os.Remove(tmpPath)
        }
        return fmt.Errorf("HTTP 错误: %s", resp.Status)
    }

    // 处理文件大小
    var bar *progressbar.ProgressBar
    var writer io.Writer = out
    fileSize := expectedSize
    
    // 如果预期大小为0，尝试从Content-Length头获取
    if fileSize == 0 {
        contentLength := resp.Header.Get("Content-Length")
        if contentLength != "" {
            if size, err := strconv.ParseInt(contentLength, 10, 64); err == nil && size > 0 {
                fileSize = size
                logger.Info("从响应头获取文件大小: %s", byteCountIEC(fileSize))
            }
        }
    }
    
    if fileSize > 0 {
        // 创建进度条
        bar = progressbar.DefaultBytes(
            fileSize,
            "下载中",
        )
        writer = io.MultiWriter(out, bar)
    } else {
        logger.Info("文件大小未知，开始下载...")
    }

    // 将响应体复制到文件，同时更新进度条
    written, err := io.Copy(writer, resp.Body)
    if err != nil {
        // 下载失败，清理临时文件
        if _, err := os.Stat(tmpPath); err == nil {
            os.Remove(tmpPath)
        }
        return err
    }

    // 确保进度条完成（显示100%）
    if bar != nil {
        bar.Finish()
        fmt.Println() // 换行，避免与后续日志重叠
    }

    // 验证下载的字节数是否与预期一致
    if expectedSize > 0 && written != expectedSize {
        // 下载不完整，清理临时文件
        if _, err := os.Stat(tmpPath); err == nil {
            os.Remove(tmpPath)
        }
        return fmt.Errorf("下载不完整: 期望 %d 字节，实际下载 %d 字节", expectedSize, written)
    }

    // 强制刷新文件缓冲区，确保所有数据写入磁盘
    if err := out.Sync(); err != nil {
        // 刷新失败，清理临时文件
        if _, err := os.Stat(tmpPath); err == nil {
            os.Remove(tmpPath)
        }
        return fmt.Errorf("无法刷新文件缓冲区: %w", err)
    }

    // 下载成功，临时文件将由调用者重命名
    return nil
}

// verifyFiles 校验下载的文件
func (d *Downloader) verifyFiles(dir string, release *Release, downloadedFiles []string) error {
    // 检查是否所有文件都有官方哈希
    allHaveHash := true
    for _, asset := range release.Assets {
        if extractSHA256(asset.Digest) == "" {
            allHaveHash = false
            break
        }
    }
    if allHaveHash {
        logger.Info("✅ 所有文件都已通过官方哈希验证，跳过额外校验")
        return nil
    }

    logger.Info("正在检查校验文件...")
    // 常见校验文件名
    checksumFiles := []string{
        "SHA256SUMS", "SHA512SUMS",
        "sha256sum.txt", "sha512sum.txt",
        "checksums.txt", release.TagName + "_checksums.txt",
    }

    for _, name := range checksumFiles {
        path := filepath.Join(dir, name)
        if _, err := os.Stat(path); err == nil {
            logger.Info("找到校验文件: %s，开始验证...", name)

            // 切换到目录执行校验
            if err := verifyChecksumFile(path, dir); err != nil {
                logger.Error("❌ 文件校验失败: %v", err)
                return err
            }
            logger.Info("✅ 所有文件校验成功！")
            return nil
        }
    }

    logger.Warn("⚠️ 未找到官方校验文件，正在生成本地校验...")
    // 生成 checksums.txt（排除自身）
    checksumPath := filepath.Join(dir, "checksums.txt")
    f, err := os.Create(checksumPath)
    if err != nil {
        return err
    }
    defer f.Close()

    entries, err := os.ReadDir(dir)
    if err != nil {
        return err
    }

    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        name := entry.Name()
        if name == "checksums.txt" {
            continue
        }
        fullPath := filepath.Join(dir, name)
        hash, err := computeSHA256(fullPath)
        if err != nil {
            logger.Warn("计算哈希失败 %s: %v", name, err)
            continue
        }
        fmt.Fprintf(f, "%s  %s\n", hash, name)
    }

    logger.Info("📝 本地校验文件已生成: %s", checksumPath)
    logger.Info("您可以通过以下命令手动验证：sha256sum -c %s", checksumPath)
    return nil
}

// 辅助函数

// extractSHA256 从 digest 字符串中提取 SHA256 值（格式如 "sha256:xxx"）
func extractSHA256(digest string) string {
    if digest == "" {
        return ""
    }
    re := regexp.MustCompile(`sha256:([a-fA-F0-9]{64})`)
    matches := re.FindStringSubmatch(digest)
    if len(matches) >= 2 {
        return matches[1]
    }
    return ""
}

// verifySHA256 验证文件哈希
func verifySHA256(path, expected string) (bool, error) {
    actual, err := computeSHA256(path)
    if err != nil {
        return false, err
    }
    return strings.EqualFold(actual, expected), nil
}

// computeSHA256 计算文件的 SHA256 哈希
func computeSHA256(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer f.Close()

    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", err
    }
    return hex.EncodeToString(h.Sum(nil)), nil
}

// verifyChecksumFile 执行 sha256sum -c 类似的功能
func verifyChecksumFile(checksumPath, dir string) error {
    f, err := os.Open(checksumPath)
    if err != nil {
        return err
    }
    defer f.Close()

    scanner := bufio.NewScanner(f)
    lineNum := 0
    for scanner.Scan() {
        lineNum++
        line := strings.TrimSpace(scanner.Text())
        if line == "" {
            continue
        }
        // 格式：<hash>  <filename> 或 <hash> *<filename>
        parts := strings.Fields(line)
        if len(parts) < 2 {
            continue
        }
        expectedHash := parts[0]
        filename := parts[1]
        if strings.HasPrefix(filename, "*") {
            filename = filename[1:]
        }
        filename = filepath.Base(filename)

        fullPath := filepath.Join(dir, filename)
        actualHash, err := computeSHA256(fullPath)
        if err != nil {
            return fmt.Errorf("无法计算 %s 的哈希: %w", filename, err)
        }
        if !strings.EqualFold(actualHash, expectedHash) {
            return fmt.Errorf("%s: 哈希不匹配 (期望 %s, 实际 %s)", filename, expectedHash, actualHash)
        }
    }
    return scanner.Err()
}

// byteCountIEC 将字节数转换为人类可读格式（如 1.2 MiB）
func byteCountIEC(b int64) string {
    const unit = 1024
    if b < unit {
        return strconv.FormatInt(b, 10) + " B"
    }
    div, exp := int64(unit), 0
    for n := b / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
