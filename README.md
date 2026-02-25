# GitHub/GitLab Release 下载器

一个强大的命令行工具，用于批量下载 GitHub 和 GitLab 仓库的 Release 版本，支持多平台编译和代理加速。

## 功能特性

- ✅ **GitHub 支持**：下载指定仓库的最新或所有 Release 版本
- ✅ **GitLab 支持**：支持多个 GitLab 实例，默认使用 `git.ryujinx.app`
- ✅ **多平台支持**：支持 Windows、macOS、Linux（包括 ARM 架构）
- ✅ **代理加速**：内置 GitHub 加速代理，提高下载速度
- ✅ **进度条**：显示下载进度，支持大文件下载
- ✅ **批量下载**：通过配置文件批量下载多个仓库
- ✅ **完整性校验**：支持 SHA256 哈希校验
- ✅ **并发处理**：支持多仓库同时下载

## 环境要求

- Go 1.16 或更高版本
- Git（用于克隆和推送代码）

## 安装方法

### 方法 1：直接编译

```bash
# 克隆仓库
git clone https://github.com/zhangxuan1340/git-downloader.git
cd git-downloader

# 编译当前平台
make build

# 或编译所有平台
make build-all
```

### 方法 2：使用预编译二进制文件

从 `bin` 目录获取对应平台的可执行文件：
- Windows: `github_download-windows-amd64.exe`
- macOS (Intel): `github_download-darwin-amd64`
- macOS (Apple Silicon): `github_download-darwin-arm64`
- Linux (64位): `github_download-linux-amd64`
- Linux (ARM32): `github_download-linux-arm`
- Linux (ARM64): `github_download-linux-arm64`

## 使用方法

### 1. 直接下载单个仓库

#### GitHub 仓库

```bash
# 下载最新 Release
./github_download nginx nginx

# 下载所有 Release
./github_download nginx nginx all
```

#### GitLab 仓库（支持多个 GitLab 实例）

```bash
# 下载最新 Release（使用默认 GitLab 实例: git.ryujinx.app）
./github_download gitlab ryubing canary

# 下载所有 Release（使用默认 GitLab 实例: git.ryujinx.app）
./github_download gitlab ryubing canary all

# 下载最新 Release（使用自定义 GitLab 实例）
./github_download gitlab git.example.com owner repo

# 下载所有 Release（使用自定义 GitLab 实例）
./github_download gitlab git.example.com owner repo all
```

### 2. 通过配置文件批量下载

1. 复制示例配置文件：

```bash
cp conf/repos.conf.example conf/repos.conf
```

2. 编辑 `conf/repos.conf` 文件，添加需要下载的仓库：

```conf
# GitHub 仓库示例
github junegunn fzf
github cli cli gh-proxy.com

# GitLab 仓库示例

# 使用默认 GitLab 实例 (git.ryujinx.app)
gitlab ryubing canary

# 使用自定义 GitLab 实例
gitlab git.example.com owner repo
gitlab gitlab.com group project

# 默认 GitHub 仓库（不指定类型）
starship starship
```

3. 运行下载器：

```bash
./github_download
```

### 3. 命令行选项

```bash
# 查看帮助
./github_download -h

# 指定下载目录
./github_download -top /path/to/downloads

# 指定配置文件
./github_download -conf /path/to/repos.conf

# 指定代理列表文件
./github_download -proxies /path/to/proxies.txt

# 设置并发数
./github_download -j 3
```

## 配置说明

### 仓库配置文件 (`conf/repos.conf`)

格式：
- `github <所有者> <仓库名> [代理]` - GitHub 仓库
- `gitlab <所有者> <仓库名> [代理]` - GitLab 仓库（使用默认实例: git.ryujinx.app）
- `gitlab <主机名> <所有者> <仓库名> [代理]` - GitLab 仓库（使用自定义实例）
- `<所有者> <仓库名> [代理]` - 默认 GitHub 仓库

### 代理配置文件 (`conf/proxies.txt`)

每行一个 GitHub 加速代理域名：

```txt
# GitHub 加速代理列表（每行一个域名）
# 程序会按顺序尝试，直到下载成功
gh-proxy.com
ghps.cc
hub.fastgit.xyz
cf.ghproxy.cc
```

## 注意事项

1. **GitLab 支持**：支持多个 GitLab 实例，默认使用 `git.ryujinx.app`，但也可以指定其他 GitLab 实例，包括 `gitlab.com` 公共仓库。

2. **代理使用**：GitLab 下载不会使用 GitHub 代理，会直接从源站下载。

3. **权限要求**：确保对目标下载目录有写入权限。

4. **网络环境**：在网络环境较差时，可能需要多次尝试下载。

5. **文件大小**：对于大文件，下载时间可能较长，请耐心等待。

## 常见问题

### Q: 为什么 GitLab 下载没有进度条？
**A**: 已修复，现在会从 HTTP 响应头获取文件大小并显示进度条。

### Q: 为什么 GitLab 下载速度很慢？
**A**: GitLab 下载不使用 GitHub 代理，直接从源站下载，速度取决于您的网络环境。

### Q: 如何添加新的 GitHub 加速代理？
**A**: 编辑 `conf/proxies.txt` 文件，添加新的代理域名。

### Q: 如何下载特定版本的 Release？
**A**: 当前版本暂不支持指定版本下载，只能下载最新或所有版本。

## 构建说明

使用 Makefile 进行构建：

```bash
# 清理构建产物
make clean

# 构建所有平台
make build-all

# 打包发布包
make package
```

## 项目结构

```
GitHub-download/
├── bin/             # 构建产物
├── conf/            # 配置文件
│   ├── repos.conf.example     # 仓库配置示例
│   ├── proxies.txt            # 代理配置
│   └── proxies.txt.example    # 代理配置示例
├── config/          # 配置加载模块
├── downloader/      # 下载核心模块
├── logger/          # 日志模块
├── main.go          # 主程序
├── Makefile         # 构建脚本
└── README.md        # 项目说明
```

## 许可证

MIT License

## 作者

- Author: zhangxuan1340
- Repository: https://github.com/zhangxuan1340/git-downloader

## 更新日志

### v1.0.0
- ✅ 初始版本发布
- ✅ 支持 GitHub Release 下载
- ✅ 支持 GitLab (git.ryujinx.app) Release 下载
- ✅ 多平台编译支持
- ✅ 代理加速功能
- ✅ 进度条显示
- ✅ 批量下载支持

### v1.1.0
- ✅ 支持多个 GitLab 实例
- ✅ 新增命令格式：`gitlab <主机名> <所有者> <仓库名> [all]`
- ✅ 新增配置格式：`gitlab <主机名> <所有者> <仓库名> [代理]`
- ✅ 保持向后兼容，默认使用 git.ryujinx.app
