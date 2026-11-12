package script

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// 版本校验必须拒绝格式错误和版本不一致的标签，手动触发不能启用发布。
func TestReleaseMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("发布脚本依赖 POSIX shell，在 macOS 和 Linux 运行")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, version, event, ref string
		ok, release               bool
	}{
		{"合法标签", "2", "push", "refs/tags/v2", true, true},
		{"大版本号", "10086", "push", "refs/tags/v10086", true, true},
		{"零版本", "0", "push", "refs/tags/v0", false, false},
		{"前导零", "02", "push", "refs/tags/v02", false, false},
		{"负数", "-2", "workflow_dispatch", "refs/heads/master", false, false},
		{"旧标签格式", "2", "push", "refs/tags/v2.0.0", false, false},
		{"版本不一致", "2", "push", "refs/tags/v3", false, false},
		{"非法标签", "2", "push", "refs/tags/v2-beta", false, false},
		{"旧版本格式", "1.2.3", "workflow_dispatch", "refs/heads/main", false, false},
		{"手动构建", "2", "workflow_dispatch", "refs/heads/main", true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			config, _ := json.Marshal(map[string]string{"version": tt.version})
			if err := os.WriteFile(filepath.Join(dir, "mygo.json"), config, 0o600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(dir, "output")
			cmd := exec.Command("bash", filepath.Join(root, "release_metadata.sh"))
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GITHUB_EVENT_NAME="+tt.event, "GITHUB_REF="+tt.ref, "GITHUB_OUTPUT="+output)
			log, err := cmd.CombinedOutput()
			if (err == nil) != tt.ok {
				t.Fatalf("校验结果与预期不符：%v %s", err, log)
			}
			if tt.ok {
				data, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(data), "is_release=true") != tt.release {
					t.Fatalf("发布标志错误：%s", data)
				}
			}
		})
	}
}

// 模拟 GitHub 响应，确保冲突、权限错误和资产缺失都不会公开 Release。
func TestPublishReleaseSafety(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("模拟可执行脚本依赖 POSIX shell，在 macOS 和 Linux 运行")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		scenario string
		ok, edit bool
	}{
		{"success", true, true},
		{"exists", false, false},
		{"existing-draft", false, false},
		{"forbidden", false, false},
		{"network", false, false},
		{"create-fails", false, false},
		{"missing-asset", false, false},
	} {
		t.Run(tt.scenario, func(t *testing.T) {
			dir := t.TempDir()
			assetsDir := filepath.Join(dir, "assets")
			if err := os.Mkdir(assetsDir, 0o700); err != nil {
				t.Fatal(err)
			}
			names := []string{"GhostFish-2.dmg", "ghostfish-darwin-amd64", "ghostfish-darwin-arm64", "ghostfish-linux-amd64", "ghostfish-windows-amd64.exe"}
			assets := make([]map[string]string, 0, len(names))
			for _, name := range names {
				if err := os.WriteFile(filepath.Join(assetsDir, name), []byte("asset"), 0o600); err != nil {
					t.Fatal(err)
				}
				assets = append(assets, map[string]string{"name": name, "state": "uploaded"})
			}
			if tt.scenario == "missing-asset" {
				assets = assets[:len(assets)-1]
			}
			body, _ := json.Marshal([]any{[]any{map[string]any{"tag_name": "v2", "draft": true, "assets": assets}}})
			response := filepath.Join(dir, "response")
			if err := os.WriteFile(response, body, 0o600); err != nil {
				t.Fatal(err)
			}
			fakeGH := `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE_LOG"
if [[ "$1" == api ]]; then
  if [[ -f "$FAKE_CREATED" ]]; then cat "$FAKE_RESPONSE"; exit 0; fi
  case "$FAKE_SCENARIO" in
    exists) printf '[[{"tag_name":"v2","draft":false}]]'; exit 0 ;;
    existing-draft) printf '[[{"tag_name":"v2","draft":true}]]'; exit 0 ;;
    forbidden|network) exit 1 ;;
    *) printf '[[]]'; exit 0 ;;
  esac
fi
if [[ "$1" == release && "$2" == create ]]; then
  touch "$FAKE_CREATED"
  if [[ "$FAKE_SCENARIO" == create-fails ]]; then exit 1; fi
  exit
fi
if [[ "$1" == release && "$2" == edit ]]; then exit 0; fi
exit 1
`
			if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(fakeGH), 0o700); err != nil {
				t.Fatal(err)
			}
			logPath := filepath.Join(dir, "calls")
			cmd := exec.Command("bash", filepath.Join(root, "publish_release.sh"), assetsDir, "2", "v2")
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "GITHUB_REPOSITORY=owner/repo", "FAKE_SCENARIO="+tt.scenario, "FAKE_RESPONSE="+response, "FAKE_LOG="+logPath, "FAKE_CREATED="+filepath.Join(dir, "created"))
			out, err := cmd.CombinedOutput()
			if (err == nil) != tt.ok {
				t.Fatalf("发布结果与预期不符：%v %s", err, out)
			}
			calls, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(calls), "release edit") != tt.edit {
				t.Fatalf("公开操作与预期不符：%s", calls)
			}
			if tt.scenario == "create-fails" {
				if _, err := os.Stat(filepath.Join(dir, "created")); err != nil {
					t.Fatal("模拟上传失败前应已创建草稿")
				}
			}
			if (tt.scenario == "exists" || tt.scenario == "existing-draft" || tt.scenario == "forbidden" || tt.scenario == "network") && strings.Contains(string(calls), "release create") {
				t.Fatalf("冲突或权限错误时不应创建 Release：%s", calls)
			}
		})
	}
}
