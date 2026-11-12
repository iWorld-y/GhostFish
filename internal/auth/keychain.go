package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
)

// secretPrefix 区分按完整保护空间保存的凭据与旧条目。
const secretPrefix = "http-auth:v2:"

// authSecretStore 对应系统凭据存储；测试使用内存替身。
type authSecretStore interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
}

// NeedForURL 将网址归一化为包含协议、主机、端口和 realm 的保护空间。
func NeedForURL(rawURL, realm string) (Need, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.User != nil || u.Hostname() == "" {
		return Need{}, false
	}
	scheme := strings.ToLower(u.Scheme)
	port := 0
	switch scheme {
	case "http":
		port = 80
	case "https":
		port = 443
	default:
		return Need{}, false
	}
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return Need{}, false
		}
	}
	return Need{Scheme: scheme, Host: strings.ToLower(u.Hostname()), Port: port, Realm: realm}, true
}

// secretKey 使用固定长度摘要，避免很长的 realm 超出系统存储键长度限制。
func secretKey(need Need) (string, error) {
	if (need.Scheme != "http" && need.Scheme != "https") || need.Host == "" || need.Port < 1 || need.Port > 65535 {
		return "", fmt.Errorf("无效的认证保护空间")
	}
	data, err := json.Marshal(need)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return secretPrefix + hex.EncodeToString(digest[:]), nil
}

// FromKeychain 仅读取指定保护空间内的有效凭据。
func FromKeychain(store authSecretStore, need Need) *mygo.AuthResponse {
	key, err := secretKey(need)
	if err != nil {
		return nil
	}
	raw, err := store.Get(key)
	if err != nil {
		return nil
	}
	var c mygo.AuthResponse
	if err := json.Unmarshal(raw, &c); err != nil || c.Username == "" {
		return nil
	}
	return &c
}

// Save 将凭据写入指定保护空间，返回编码或存储错误。
func Save(store authSecretStore, need Need, cred *mygo.AuthResponse) error {
	key, err := secretKey(need)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	return store.Set(key, raw)
}
