package main

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// targetURL 从启动参数里取目标网址：必须给一个参数，多给的忽略；
// 没写协议时补 https，非 http/https 或解析不成的一律报错。
func targetURL(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("缺少目标网址")
	}
	raw := strings.TrimSpace(args[0])
	if raw == "" {
		return "", errors.New("目标网址为空")
	}
	if !strings.Contains(raw, "://") {
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
