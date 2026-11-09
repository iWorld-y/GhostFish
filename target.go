package main

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// targetURL 返回启动网址；没有参数时返回空网址，多余参数被忽略。
func targetURL(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	return parseURL(args[0])
}

// parseURL 校验 HTTP 网址，并为缺少协议的网址补全 HTTPS。
func parseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("目标网址为空")
	}
	schemeEnd := strings.Index(raw, "://")
	if schemeEnd < 0 || schemeEnd > strings.IndexAny(raw, "/?#") {
		// 主机名加数字端口不是协议；其他带冒号的输入不能当成网址凭据。
		if parsed, err := url.Parse(raw); err == nil && parsed.Scheme != "" {
			scheme := strings.ToLower(parsed.Scheme)
			port := raw[len(parsed.Scheme)+1:]
			if end := strings.IndexAny(port, "/?#"); end >= 0 {
				port = port[:end]
			}
			_, portErr := strconv.ParseUint(port, 10, 16)
			switch scheme {
			case "http", "https", "mailto", "tel", "data", "javascript", "file", "ftp", "ws", "wss":
				return "", fmt.Errorf("只支持 http 与 https，收到 %q", scheme)
			default:
				if portErr != nil {
					return "", fmt.Errorf("只支持 http 与 https，收到 %q", scheme)
				}
			}
		}
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("网址无法解析：%w", err)
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return "", fmt.Errorf("只支持 http 与 https，收到 %q", u.Scheme)
	}
	if u.Host == "" {
		return "", errors.New("网址缺少主机名")
	}
	return u.String(), nil
}
