package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// stateFilePath 准备应用状态目录，失败时禁用本次持久化。
func stateFilePath() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil || dir == "" {
		fmt.Fprintf(os.Stderr, "ghostfish: 拿不到用户数据目录，本次不保存状态：%v\n", err)
		return ""
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: 创建数据目录失败，本次不保存状态：%v\n", err)
		return ""
	}
	return filepath.Join(dir, "state.json")
}
