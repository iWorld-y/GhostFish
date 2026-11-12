package auth

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/egoist/mygo"
)

// authEnv 让用户通过环境变量提供凭据，格式为 user:pass。
const authEnv = "GHOSTFISH_AUTH"

// Resolve 依次尝试环境变量、系统钥匙串、终端输入。
func Resolve(ctx context.Context, need *Need) *mygo.AuthResponse {
	if ctx.Err() != nil {
		return nil
	}
	if c := credentialFromEnv(); c != nil {
		fmt.Fprintln(os.Stderr, "ghostfish: 用环境变量里的凭据")
		return c
	}
	if ctx.Err() != nil {
		return nil
	}
	if c := FromKeychain(mygo.Secrets, *need); c != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: 用钥匙串里 %s 的凭据\n", need.Host)
		return c
	}
	return promptCredential(ctx, need)
}

func credentialFromEnv() *mygo.AuthResponse {
	raw := strings.TrimSpace(os.Getenv(authEnv))
	if raw == "" {
		return nil
	}
	user, pass, ok := strings.Cut(raw, ":")
	if !ok || user == "" {
		return nil
	}
	return &mygo.AuthResponse{Username: user, Password: pass}
}
