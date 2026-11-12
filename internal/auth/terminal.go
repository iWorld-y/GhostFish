package auth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"golang.org/x/term"
)

// promptMu 防止多个标签页同时向同一终端读取凭据。
var promptMu sync.Mutex

// serialPrompt 按顺序执行终端输入，避免不同站点的提示交错。
func serialPrompt(ctx context.Context, read func() *mygo.AuthResponse) *mygo.AuthResponse {
	promptMu.Lock()
	defer promptMu.Unlock()
	if ctx.Err() != nil {
		return nil
	}
	cred := read()
	if ctx.Err() != nil {
		return nil
	}
	return cred
}

// promptCredential 在终端里问用户名和密码，密码不回显。
func promptCredential(ctx context.Context, need *Need) *mygo.AuthResponse {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil
	}
	return serialPrompt(ctx, func() *mygo.AuthResponse {
		fmt.Fprintf(os.Stderr, "%s（realm %q）需要登录\n用户名：", need.Host, need.Realm)
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return nil
		}
		user := strings.TrimSpace(line)
		if user == "" || ctx.Err() != nil {
			return nil
		}
		fmt.Fprint(os.Stderr, "密码（不回显）：")
		pass, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return nil
		}
		return &mygo.AuthResponse{Username: user, Password: string(pass)}
	})
}
