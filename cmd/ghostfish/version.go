package main

import "strings"

// displayVersion 兼容命令行注入的版本和 MyGo 应用包中的版本。
func displayVersion(buildVersion, packageVersion string) string {
	if buildVersion != "" && buildVersion != "dev" {
		return buildVersion
	}
	if packageVersion != "" {
		return "v" + strings.TrimPrefix(packageVersion, "v")
	}
	return "dev"
}
