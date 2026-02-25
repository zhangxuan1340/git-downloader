# GitHub/GitLab Release 下载器 Makefile
# 支持多平台编译：Windows、macOS、Linux

# 项目名称
APP_NAME = github_download

# 版本号
VERSION = 1.0.0

# 作者信息
AUTHOR = Your Name

# 默认目标
all: build

# 清理构建产物
clean:
	@echo "清理构建产物..."
	@rm -rf ./bin
	@rm -rf ./logs

# 创建输出目录
prepare:
	@echo "创建输出目录..."
	@mkdir -p ./bin

# 构建所有平台
build-all: clean prepare build-windows build-macos build-macos-arm build-linux build-linux-arm build-linux-arm64
	@echo "所有平台构建完成！"

# 构建当前平台
build:
	@echo "构建当前平台..."
	@go build -o ./bin/$(APP_NAME) .
	@echo "构建完成: ./bin/$(APP_NAME)"

# 构建 Windows 平台 (64位)
build-windows:
	@echo "构建 Windows 平台 (64位)..."
	@GOOS=windows GOARCH=amd64 go build -o ./bin/$(APP_NAME)-windows-amd64.exe .
	@echo "构建完成: ./bin/$(APP_NAME)-windows-amd64.exe"

# 构建 macOS 平台 (64位)
build-macos:
	@echo "构建 macOS 平台 (64位)..."
	@GOOS=darwin GOARCH=amd64 go build -o ./bin/$(APP_NAME)-darwin-amd64 .
	@echo "构建完成: ./bin/$(APP_NAME)-darwin-amd64"

# 构建 macOS 平台 (ARM64)
build-macos-arm:
	@echo "构建 macOS 平台 (ARM64)..."
	@GOOS=darwin GOARCH=arm64 go build -o ./bin/$(APP_NAME)-darwin-arm64 .
	@echo "构建完成: ./bin/$(APP_NAME)-darwin-arm64"

# 构建 Linux 平台 (64位)
build-linux:
	@echo "构建 Linux 平台 (64位)..."
	@GOOS=linux GOARCH=amd64 go build -o ./bin/$(APP_NAME)-linux-amd64 .
	@echo "构建完成: ./bin/$(APP_NAME)-linux-amd64"

# 构建 Linux 平台 (ARM32位)
build-linux-arm:
	@echo "构建 Linux 平台 (ARM32位)..."
	@GOOS=linux GOARCH=arm go build -o ./bin/$(APP_NAME)-linux-arm .
	@echo "构建完成: ./bin/$(APP_NAME)-linux-arm"

# 构建 Linux 平台 (ARM64位)
build-linux-arm64:
	@echo "构建 Linux 平台 (ARM64位)..."
	@GOOS=linux GOARCH=arm64 go build -o ./bin/$(APP_NAME)-linux-arm64 .
	@echo "构建完成: ./bin/$(APP_NAME)-linux-arm64"

# 运行程序
run:
	@go run .

# 运行测试
test:
	@echo "运行测试..."
	@go test ./...

# 查看帮助
help:
	@echo "GitHub/GitLab Release 下载器 构建工具"
	@echo ""
	@echo "可用命令:"
	@echo "  make clean           清理构建产物"
	@echo "  make prepare         创建输出目录"
	@echo "  make build           构建当前平台"
	@echo "  make build-all       构建所有平台"
	@echo "  make build-windows   构建 Windows 平台"
	@echo "  make build-macos     构建 macOS 平台 (Intel)"
	@echo "  make build-macos-arm 构建 macOS 平台 (Apple Silicon)"
	@echo "  make build-linux     构建 Linux 平台 (64位)"
	@echo "  make build-linux-arm 构建 Linux 平台 (ARM32位)"
	@echo "  make build-linux-arm64 构建 Linux 平台 (ARM64位)"
	@echo "  make run             运行程序"
	@echo "  make test            运行测试"
	@echo "  make help            查看帮助"
	@echo ""
	@echo "构建产物会输出到 ./bin 目录"

# 安装到系统 (Linux/macOS)
install:
	@echo "安装到系统..."
	@go build -o /usr/local/bin/$(APP_NAME) .
	@echo "安装完成: /usr/local/bin/$(APP_NAME)"

# 卸载
uninstall:
	@echo "卸载程序..."
	@rm -f /usr/local/bin/$(APP_NAME)
	@echo "卸载完成"

# 打包发布包
package:
	@echo "打包发布包..."
	@mkdir -p ./bin/package
	@cp ./bin/$(APP_NAME)-windows-amd64.exe ./bin/package/
	@cp ./bin/$(APP_NAME)-darwin-amd64 ./bin/package/
	@cp ./bin/$(APP_NAME)-darwin-arm64 ./bin/package/
	@cp ./bin/$(APP_NAME)-linux-amd64 ./bin/package/
	@cp ./bin/$(APP_NAME)-linux-arm ./bin/package/
	@cp ./bin/$(APP_NAME)-linux-arm64 ./bin/package/
	@cp ./README.md ./bin/package/
	@cp ./conf/repos.conf.example ./bin/package/
	@cp ./conf/proxies.txt ./bin/package/
	@echo "打包完成: ./bin/package/"

.PHONY: all clean prepare build build-all build-windows build-macos build-macos-arm build-linux build-linux-arm build-linux-arm64 run test help install uninstall package