package base

import (
    "bytes"
    "context"
    "fmt"
    "os"
    "os/exec"
    "strings"
    "time"
)

// RunCMD 以统一的方式执行外部命令，返回 stdout+stderr 文本，并支持简单重试。
// dir: 进程工作目录；retries: 额外重试次数（总尝试=1+retries）。
func RunCMD(ctx context.Context, dir, name string, args []string, retries int) (string, error) {
    attempts := 1 + retries
    var out []byte
    var err error
    for attempt := 1; attempt <= attempts; attempt++ {
        cmd := exec.CommandContext(ctx, name, args...)
        cmd.Dir = dir
        buf := &bytes.Buffer{}
        cmd.Stdout = buf
        cmd.Stderr = buf
        err = cmd.Run()
        out = buf.Bytes()

        // 成功或不应重试时返回
        if err == nil || !shouldRetry(buf.String(), attempt, attempts) || ctx.Err() != nil {
            break
        }

        // 读取可配置参数
        maxRetries, maxBackoff := getRetryConfigs(retries)
        attempts = 1 + maxRetries
        // 指数退避（基于 200ms）并限制最大退避
        delay := time.Duration(1<<uint(attempt-1)) * 200 * time.Millisecond
        if delay > maxBackoff {
            delay = maxBackoff
        }
        // 调试级重试日志
        Debugf("retrying command (attempt %d/%d) in %s: %s %s\n", attempt+1, attempts, delay, name, strings.Join(args, " "))
        select {
        case <-ctx.Done():
            return string(out), ctx.Err()
        case <-time.After(delay):
        }
    }
    if err != nil {
        return string(out), fmt.Errorf("exec failed: %s %s: %w\n%s", name, strings.Join(args, " "), err, string(out))
    }
    return string(out), nil
}

// shouldRetry 根据输出特征判断是否值得重试（网络/暂时性错误）。
func shouldRetry(output string, attempt, attempts int) bool {
    if attempt >= attempts {
        return false
    }
    low := strings.ToLower(output)
    // 常见网络/暂态错误信号
    keys := []string{
        "timeout", "timed out", "temporary failure", "tls: handshake failure",
        "connection reset", "connection refused", "no route to host", "i/o timeout",
        "couldn't resolve host", "could not resolve host", "name or service not known",
        "remote error", "http 5", "internal server error", "rate limit",
    }
    for _, k := range keys {
        if strings.Contains(low, k) {
            return true
        }
    }
    return false
}

// getRetryConfigs 从环境变量读取重试配置：
// LYNX_RETRIES: 最大重试次数（默认使用传入的 retries）
// LYNX_MAX_BACKOFF_MS: 最大退避时间（毫秒，默认 2000ms）
func getRetryConfigs(defaultRetries int) (int, time.Duration) {
    r := defaultRetries
    if v := strings.TrimSpace(os.Getenv("LYNX_RETRIES")); v != "" {
        if n, err := parsePositiveInt(v); err == nil {
            r = n
        }
    }
    maxBackoff := 2000 * time.Millisecond
    if v := strings.TrimSpace(os.Getenv("LYNX_MAX_BACKOFF_MS")); v != "" {
        if n, err := parsePositiveInt(v); err == nil && n > 0 {
            maxBackoff = time.Duration(n) * time.Millisecond
        }
    }
    return r, maxBackoff
}

func parsePositiveInt(s string) (int, error) {
    var n int
    _, err := fmt.Sscanf(s, "%d", &n)
    if err != nil {
        return 0, err
    }
    if n < 0 {
        n = 0
    }
    return n, nil
}

// 日志门控已由 logger 封装统一处理（见 logger.go）
