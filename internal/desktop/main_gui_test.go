//go:build darwin && gui

package desktop

import (
	"fmt"
	"os"
	"testing"

	"github.com/egoist/mygo"
)

// TestMain 在显式启用 GUI 检查时启动 MyGo 主循环。
func TestMain(m *testing.M) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		os.Exit(m.Run())
	}
	code := 1
	mygo.App.WhenReady(func() {
		go func() {
			code = m.Run()
			mygo.App.Quit()
		}()
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(code)
}
