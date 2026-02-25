package config

import (
    "bufio"
    "os"
    "strings"
)

// LoadProxies 从文件读取代理列表，每行一个域名，支持 # 注释和空行
func LoadProxies(path string) ([]string, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var proxies []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        // 跳过空行和注释
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        // 只取第一个字段（防止意外空格）
        parts := strings.Fields(line)
        if len(parts) > 0 {
            proxies = append(proxies, parts[0])
        }
    }
    if err := scanner.Err(); err != nil {
        return nil, err
    }
    return proxies, nil
}
