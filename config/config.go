package config

import (
    "bufio"
    "os"
    "strings"
)

// RepoConfig 表示一个仓库的配置
type RepoConfig struct {
    Type       string // 仓库类型：github 或 gitlab
    Owner      string
    Repo       string
    Proxy      string // 代理，如果为空则使用默认
    GitLabHost string // GitLab 实例主机名，例如 git.ryujinx.app
}

// LoadRepos 从文件加载仓库配置
// 格式：
//   github 所有者 仓库名 [代理]  // GitHub 仓库
//   gitlab <主机名> 所有者 仓库名 [代理]  // GitLab 仓库（支持自定义实例）
//   gitlab 所有者 仓库名 [代理]  // GitLab 仓库（默认使用 git.ryujinx.app）
//   所有者 仓库名 [代理]          // 默认 GitHub 仓库
func LoadRepos(path string) ([]RepoConfig, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var repos []RepoConfig
    scanner := bufio.NewScanner(file)
    lineNum := 0

    for scanner.Scan() {
        lineNum++
        line := strings.TrimSpace(scanner.Text())
        // 跳过空行和注释
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        parts := strings.Fields(line)
        if len(parts) < 2 {
            // 格式错误，跳过
            continue
        }

        cfg := RepoConfig{}
        
        // 检查是否指定了仓库类型
        if parts[0] == "github" || parts[0] == "gitlab" {
            if parts[0] == "github" {
                // GitHub 格式：github 所有者 仓库名 [代理]
                if len(parts) < 3 {
                    continue
                }
                cfg.Type = parts[0]
                cfg.Owner = parts[1]
                cfg.Repo = parts[2]
                if len(parts) >= 4 {
                    cfg.Proxy = parts[3]
                }
            } else {
                // GitLab 格式：
                // 格式1: gitlab <主机名> 所有者 仓库名 [代理]
                // 格式2: gitlab 所有者 仓库名 [代理] (默认 git.ryujinx.app)
                if len(parts) >= 4 {
                    // 格式1: 带主机名
                    cfg.Type = parts[0]
                    cfg.GitLabHost = parts[1]
                    cfg.Owner = parts[2]
                    cfg.Repo = parts[3]
                    if len(parts) >= 5 {
                        cfg.Proxy = parts[4]
                    }
                } else if len(parts) >= 3 {
                    // 格式2: 无主机名，使用默认值
                    cfg.Type = parts[0]
                    cfg.GitLabHost = "git.ryujinx.app" // 默认值
                    cfg.Owner = parts[1]
                    cfg.Repo = parts[2]
                    if len(parts) >= 4 {
                        cfg.Proxy = parts[3]
                    }
                } else {
                    continue
                }
            }
        } else {
            // 默认 GitHub 仓库
            cfg.Type = "github"
            cfg.Owner = parts[0]
            cfg.Repo = parts[1]
            if len(parts) >= 3 {
                cfg.Proxy = parts[2]
            }
        }
        
        repos = append(repos, cfg)
    }

    if err := scanner.Err(); err != nil {
        return nil, err
    }

    return repos, nil
}
