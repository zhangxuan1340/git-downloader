package logger

import (
    "fmt"
    "io"
    "log"
    "os"
    "path/filepath"
    "time"
)

var (
    logFile   *os.File
    logger    *log.Logger
    logDir    string
    logPrefix string
)

// Init 初始化日志系统
func Init(dir string, prefix string) error {
    logDir = dir
    logPrefix = prefix

    if err := os.MkdirAll(logDir, 0755); err != nil {
        return fmt.Errorf("创建日志目录失败: %w", err)
    }

    logFileName := fmt.Sprintf("%s_%s.log", logPrefix, time.Now().Format("20060102_150405"))
    logPath := filepath.Join(logDir, logFileName)

    f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return fmt.Errorf("创建日志文件失败: %w", err)
    }
    logFile = f

    // 同时输出到文件和控制台
    multiWriter := io.MultiWriter(os.Stdout, logFile)
    logger = log.New(multiWriter, "", log.LstdFlags)

    return nil
}

// Close 关闭日志文件
func Close() {
    if logFile != nil {
        logFile.Close()
    }
}

// Info 记录信息日志
func Info(format string, v ...interface{}) {
    logger.Printf("[INFO] "+format, v...)
}

// Warn 记录警告日志
func Warn(format string, v ...interface{}) {
    logger.Printf("[WARN] "+format, v...)
}

// Error 记录错误日志
func Error(format string, v ...interface{}) {
    logger.Printf("[ERROR] "+format, v...)
}

// CleanOldLogs 清理超过指定天数的旧日志
func CleanOldLogs(days int) error {
    files, err := os.ReadDir(logDir)
    if err != nil {
        return err
    }

    cutoff := time.Now().AddDate(0, 0, -days)
    for _, file := range files {
        if file.IsDir() {
            continue
        }
        info, err := file.Info()
        if err != nil {
            continue
        }
        if info.ModTime().Before(cutoff) {
            path := filepath.Join(logDir, file.Name())
            if err := os.Remove(path); err == nil {
                Info("已删除旧日志: %s", path)
            }
        }
    }
    return nil
}
