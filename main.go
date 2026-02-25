package main

import (
    "flag"
    "fmt"
    "os"
    "path/filepath"
    "sync"

    "github-downloader/config"
    "github-downloader/downloader"
    "github-downloader/logger"
)

const (
    logRetentionDays = 7
    defaultLogPrefix = "download"
)

func main() {
    // 获取程序所在目录
    execDir, err := getExecutableDir()
    if err != nil {
        fmt.Fprintf(os.Stderr, "无法获取程序目录: %v\n", err)
        os.Exit(1)
    }

    // 设置默认路径（相对于程序目录）
    defaultTopDir    := filepath.Join(execDir, "downloads")
    defaultConfig    := filepath.Join(execDir, "conf", "repos.conf")
    defaultProxies   := filepath.Join(execDir, "conf", "proxies.txt")
    defaultLogDir    := filepath.Join(execDir, "logs")

    // 自定义帮助信息
    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, "GitHub/GitLab Release 下载器\n\n")
        fmt.Fprintf(os.Stderr, "用法:\n  %s [选项]\n  或\n  %s <所有者> <仓库名> [all] [选项]\n  或\n  %s gitlab <所有者> <仓库名> [all] [选项]\n\n", os.Args[0], os.Args[0], os.Args[0])
        fmt.Fprintf(os.Stderr, "选项:\n")
        fmt.Fprintf(os.Stderr, "  -top string\n        下载根目录 (默认 \"%s\")\n", defaultTopDir)
        fmt.Fprintf(os.Stderr, "  -conf string\n        配置文件路径 (默认 \"%s\")\n", defaultConfig)
        fmt.Fprintf(os.Stderr, "  -proxies string\n        代理列表文件路径 (默认 \"%s\")\n", defaultProxies)
        fmt.Fprintf(os.Stderr, "  -log string\n        日志目录 (默认 \"%s\")\n", defaultLogDir)
        fmt.Fprintf(os.Stderr, "  -j int\n        并发数（同时处理的仓库数） (默认 1)\n")
        fmt.Fprintf(os.Stderr, "  -h\t显示此帮助信息\n\n")
        fmt.Fprintf(os.Stderr, "示例:\n")
        fmt.Fprintf(os.Stderr, "  1. 使用默认配置:\n     %s\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "  2. 指定自定义配置文件:\n     %s -conf /path/to/repos.conf\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "  3. 指定代理列表文件:\n     %s -proxies /path/to/proxies.txt\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "  4. 设置并发数为 3:\n     %s -j 3\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "  5. 查看帮助:\n     %s -h\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "  6. 下载单个 GitHub 仓库的最新 Release:\n     %s nginx nginx\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "  7. 下载单个 GitHub 仓库的所有 Release:\n     %s nginx nginx all\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "  8. 下载单个 GitLab 仓库的最新 Release:\n     %s gitlab ryubing canary\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "  9. 下载单个 GitLab 仓库的所有 Release:\n     %s gitlab ryubing canary all\n", os.Args[0])
    }

    // 命令行参数
    topDir    := flag.String("top", defaultTopDir, "下载根目录")
    configFile := flag.String("conf", defaultConfig, "配置文件路径")
    proxiesFile := flag.String("proxies", defaultProxies, "代理列表文件路径")
    logDir    := flag.String("log", defaultLogDir, "日志目录")
    concurrent := flag.Int("j", 1, "并发数（同时处理的仓库数）")
    help := flag.Bool("h", false, "显示帮助信息")
    flag.Parse()

    if *help {
        flag.Usage()
        os.Exit(0)
    }

    // 初始化日志
    if err := logger.Init(*logDir, defaultLogPrefix); err != nil {
        fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
        os.Exit(1)
    }
    defer logger.Close()

    // 在程序目录下生成示例配置文件（如果不存在）
    generateExampleConfigs(execDir)

    // 加载代理列表
    var proxies []string
    if _, err := os.Stat(*proxiesFile); err == nil {
        proxies, err = config.LoadProxies(*proxiesFile)
        if err != nil {
            logger.Warn("加载代理列表失败: %v，将使用内置默认代理", err)
        } else {
            logger.Info("成功加载 %d 个代理", len(proxies))
        }
    } else {
        logger.Warn("代理列表文件 %s 不存在，将使用内置默认代理", *proxiesFile)
    }

    // 创建下载器（传入代理列表）
    d := downloader.NewDownloader(*topDir, proxies)

    // 检查是否有位置参数（非标志参数）
    args := flag.Args()
    if len(args) >= 2 {
        // 检查是否是 GitLab 仓库
        isGitLab := false
        var owner, repo, gitLabHost string
        var downloadAll bool

        if args[0] == "gitlab" {
            // GitLab 仓库格式:
            // 格式1: gitlab <主机名> <所有者> <仓库名> [all]
            // 格式2: gitlab <所有者> <仓库名> [all] (默认 git.ryujinx.app)
            if len(args) >= 4 {
                // 格式1: 带主机名
                isGitLab = true
                gitLabHost = args[1]
                owner = args[2]
                repo = args[3]
                downloadAll = len(args) >= 5 && args[4] == "all"
            } else if len(args) >= 3 {
                // 格式2: 无主机名，使用默认值
                isGitLab = true
                gitLabHost = "" // 空值，会使用默认值
                owner = args[1]
                repo = args[2]
                downloadAll = len(args) >= 4 && args[3] == "all"
            }
        } else {
            // GitHub 仓库格式: <所有者> <仓库名> [all]
            owner = args[0]
            repo = args[1]
            downloadAll = len(args) >= 3 && args[2] == "all"
        }

        logger.Info("======== 开始下载指定仓库 ========")
        logger.Info("下载目录: %s", *topDir)
        logger.Info("代理列表: %s", *proxiesFile)
        logger.Info("日志目录: %s", *logDir)
        logger.Info("指定仓库: %s/%s", owner, repo)
        logger.Info("仓库类型: %s", map[bool]string{true: "GitLab", false: "GitHub"}[isGitLab])
        logger.Info("下载模式: %s", map[bool]string{true: "所有 Release", false: "最新 Release"}[downloadAll])

        if isGitLab {
            if downloadAll {
                if err := d.ProcessGitLabRepoAll(gitLabHost, owner, repo, ""); err != nil {
                    logger.Error("处理仓库 %s/%s 失败: %v", owner, repo, err)
                }
            } else {
                if err := d.ProcessGitLabRepo(gitLabHost, owner, repo, ""); err != nil {
                    logger.Error("处理仓库 %s/%s 失败: %v", owner, repo, err)
                }
            }
        } else {
            if downloadAll {
                if err := d.ProcessRepoAll(owner, repo, ""); err != nil {
                    logger.Error("处理仓库 %s/%s 失败: %v", owner, repo, err)
                }
            } else {
                if err := d.ProcessRepo(owner, repo, ""); err != nil {
                    logger.Error("处理仓库 %s/%s 失败: %v", owner, repo, err)
                }
            }
        }

        logger.Info("======== 仓库处理完成 ========")
    } else {
        // 使用配置文件模式
        logger.Info("======== 开始批量下载 ========")
        logger.Info("下载目录: %s", *topDir)
        logger.Info("配置文件: %s", *configFile)
        logger.Info("代理列表: %s", *proxiesFile)
        logger.Info("日志目录: %s", *logDir)

        // 加载仓库配置
        repos, err := config.LoadRepos(*configFile)
        if err != nil {
            if os.IsNotExist(err) {
                logger.Error("配置文件 %s 不存在", *configFile)
                logger.Info("请参考程序目录下的 conf/repos.conf.example 文件创建配置文件")
            } else {
                logger.Error("加载配置文件失败: %v", err)
            }
            os.Exit(1)
        }

        if len(repos) == 0 {
            logger.Warn("配置文件中没有有效的仓库")
            return
        }

        // 使用并发处理
        var wg sync.WaitGroup
        sem := make(chan struct{}, *concurrent)

        for _, repo := range repos {
            wg.Add(1)
            sem <- struct{}{} // 占用一个槽位
            go func(r config.RepoConfig) {
                defer wg.Done()
                defer func() { <-sem }() // 释放槽位

                if r.Type == "gitlab" {
                    if err := d.ProcessGitLabRepo(r.GitLabHost, r.Owner, r.Repo, r.Proxy); err != nil {
                        logger.Error("处理 GitLab 仓库 %s/%s 失败: %v", r.Owner, r.Repo, err)
                    }
                } else {
                    if err := d.ProcessRepo(r.Owner, r.Repo, r.Proxy); err != nil {
                        logger.Error("处理 GitHub 仓库 %s/%s 失败: %v", r.Owner, r.Repo, err)
                    }
                }
            }(repo)
        }

        wg.Wait()

        // 清理旧日志
        logger.Info("======== 清理旧日志 ========")
        if err := logger.CleanOldLogs(logRetentionDays); err != nil {
            logger.Error("清理旧日志失败: %v", err)
        }

        logger.Info("======== 所有仓库处理完成 ========")
    }
}

// getExecutableDir 返回可执行文件所在的目录
func getExecutableDir() (string, error) {
    exe, err := os.Executable()
    if err != nil {
        return "", err
    }
    return filepath.Dir(exe), nil
}

// generateExampleConfigs 在程序目录下生成示例配置文件（如果尚不存在）
func generateExampleConfigs(execDir string) {
    confDir := filepath.Join(execDir, "conf")
    if err := os.MkdirAll(confDir, 0755); err != nil {
        logger.Warn("无法创建配置目录 %s: %v", confDir, err)
        return
    }

    // 生成 repos.conf.example
    reposExample := filepath.Join(confDir, "repos.conf.example")
    if _, err := os.Stat(reposExample); os.IsNotExist(err) {
        content := `# 仓库配置文件
# 格式:
#   github 所有者 仓库名 [代理]  # GitHub 仓库
#   gitlab 所有者 仓库名 [代理]  # GitLab 仓库（使用默认实例: git.ryujinx.app）
#   gitlab <主机名> 所有者 仓库名 [代理]  # GitLab 仓库（使用自定义实例）
#   所有者 仓库名 [代理]          # 默认 GitHub 仓库
# 代理是可选的，如果不指定则使用全局代理列表（见 proxies.txt）
# 示例:
# # GitHub 仓库示例
# junegunn fzf
# cli cli gh-proxy.com
# starship starship
# 
# # GitLab 仓库示例
# 
# # 使用默认 GitLab 实例 (git.ryujinx.app)
# gitlab ryubing canary
# 
# # 使用自定义 GitLab 实例
# gitlab git.example.com owner repo
# gitlab gitlab.com group project
`
        if err := os.WriteFile(reposExample, []byte(content), 0644); err != nil {
            logger.Warn("无法生成示例仓库配置文件: %v", err)
        } else {
            logger.Info("已生成示例仓库配置文件: %s", reposExample)
        }
    }

    // 生成 proxies.txt（实际使用的代理列表，用户可编辑）
    proxiesFile := filepath.Join(confDir, "proxies.txt")
    if _, err := os.Stat(proxiesFile); os.IsNotExist(err) {
        content := `# GitHub 加速代理列表（每行一个域名）
# 程序会按顺序尝试，直到下载成功
# 你可以随时编辑此文件，无需重启程序
gh-proxy.com
ghps.cc
hub.fastgit.xyz
cf.ghproxy.cc
`
        if err := os.WriteFile(proxiesFile, []byte(content), 0644); err != nil {
            logger.Warn("无法生成代理列表文件: %v", err)
        } else {
            logger.Info("已生成代理列表文件: %s（你可直接编辑此文件）", proxiesFile)
        }
    }

    // 生成 proxies.txt.example（仅供参考，不参与程序读取）
    proxiesExample := filepath.Join(confDir, "proxies.txt.example")
    if _, err := os.Stat(proxiesExample); os.IsNotExist(err) {
        content := `# GitHub 加速代理示例（仅供参考）
# 实际使用时，请将需要启用的代理复制到 proxies.txt 中
# 注意：这些代理可能随时失效，请自行测试可用性

gh-proxy.com
ghps.cc
hub.fastgit.xyz
cf.ghproxy.cc
ghproxy.com
git.yzuu.com
`
        if err := os.WriteFile(proxiesExample, []byte(content), 0644); err != nil {
            logger.Warn("无法生成示例代理列表: %v", err)
        } else {
            logger.Info("已生成示例代理列表: %s（仅供参考）", proxiesExample)
        }
    }
}
