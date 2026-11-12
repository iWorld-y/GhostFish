// Command ghostfish 启动浮动浏览器。
package main

import (
	"fmt"
	"os"

	"ghostfish/internal/app"
	"ghostfish/internal/browser"

	"github.com/egoist/mygo"
)

// version 由发布构建注入：go build -ldflags "-X main.version=v0.1.0" ./cmd/ghostfish。
// 从源码直接构建时是 dev，方便分辨手上这个是不是下载来的。
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-version" || os.Args[1] == "--version") {
		fmt.Printf("ghostfish %s\n", displayVersion(version, mygo.App.Version()))
		return
	}

	target, err := browser.TargetURL(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: %v\n", err)
		fmt.Fprintln(os.Stderr, "用法：ghostfish [网址]")
		fmt.Fprintln(os.Stderr, "      ghostfish -version")
		fmt.Fprintln(os.Stderr, "例：ghostfish https://example.com")
		os.Exit(2)
	}

	if err := app.Run(target); err != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: %v\n", err)
		os.Exit(1)
	}
}
