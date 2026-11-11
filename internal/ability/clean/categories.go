// Package clean implements the junk cleanup ability (垃圾清理/目录扫描):
// category-based directory scanning, size accounting, and safe deletion
// with an admin-confirmed two-phase flow (scan first, run deletes).
package clean

import (
	"os"
	"regexp"
	"strings"
	"time"
)

// Category is one junk category: a set of directory roots walked for junk
// files. Scan never leaves the declared roots.
type Category struct {
	ID      string
	Name    string
	Roots   []string               // path templates with %VAR% placeholders
	Match   func(base string) bool // nil = every file
	MinAge  time.Duration          // 0 = no age guard
	Recycle bool                   // aggregate recycle-bin handling via shell API
	Default bool                   // enabled in the default category set
}

// defaultCategories returns the v0.1 category set. windows_old defaults to
// disabled: deleting it removes the OS rollback path.
func defaultCategories(minAge time.Duration) []Category {
	localAppData := `%LOCALAPPDATA%`
	return []Category{
		{
			ID: "temp", Name: "系统临时文件", Default: true, MinAge: minAge,
			Roots: []string{"%TEMP%", "%TMP%", "%SystemRoot%\\Temp"},
		},
		{
			ID: "recycle_bin", Name: "回收站", Default: true, Recycle: true,
		},
		{
			ID: "windows_update", Name: "Windows 更新缓存", Default: true,
			Roots: []string{"%SystemRoot%\\SoftwareDistribution\\Download"},
		},
		{
			ID: "browser_cache", Name: "浏览器缓存", Default: true,
			Roots: []string{
				localAppData + `\Google\Chrome\User Data\Default\Cache\Cache_Data`,
				localAppData + `\Microsoft\Edge\User Data\Default\Cache\Cache_Data`,
			},
		},
		{
			ID: "wer", Name: "错误报告", Default: true,
			Roots: []string{
				localAppData + `\Microsoft\Windows\WER\ReportQueue`,
				localAppData + `\Microsoft\Windows\WER\ReportArchive`,
			},
		},
		{
			ID: "d3dscache", Name: "DirectX 着色器缓存", Default: true,
			Roots: []string{localAppData + `\D3DSCache`},
		},
		{
			ID: "thumbcache", Name: "缩略图缓存", Default: true,
			Roots: []string{localAppData + `\Microsoft\Windows\Explorer`},
			Match: func(base string) bool {
				return strings.HasPrefix(strings.ToLower(base), "thumbcache_") &&
					strings.HasSuffix(strings.ToLower(base), ".db")
			},
		},
		{
			ID: "windows_old", Name: "旧系统安装", Default: false,
			Roots: []string{`%SystemDrive%\Windows.old`},
		},
	}
}

var envRe = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)

// expandEnv resolves %VAR% placeholders; unknown variables expand to "" so
// missing roots simply do not exist (os.Stat / WalkDir skip them). A package
// variable so tests can inject a mapping without touching process env.
var expandEnv = func(path string) string {
	return envRe.ReplaceAllStringFunc(path, func(m string) string {
		return os.Getenv(m[1 : len(m)-1])
	})
}

// resolveCategories returns the categories in effect: the default set plus
// config opt-ins minus config opt-outs, filtered by the payload subset if
// one is given. Unknown or not-enabled ids produce an error message.
func resolveCategories(defs []Category, enableExtra, disable, subset []string) ([]Category, string) {
	byID := map[string]Category{}
	var order []string
	for _, c := range defs {
		byID[c.ID] = c
		order = append(order, c.ID)
	}
	enabledSet := map[string]bool{}
	for _, id := range order {
		if byID[id].Default {
			enabledSet[id] = true
		}
	}
	for _, id := range enableExtra {
		if _, ok := byID[id]; !ok {
			return nil, "unknown category: " + id
		}
		enabledSet[id] = true
	}
	for _, id := range disable {
		enabledSet[id] = false
	}
	var out []Category
	if len(subset) == 0 {
		for _, id := range order {
			if enabledSet[id] {
				out = append(out, byID[id])
			}
		}
		return out, ""
	}
	for _, id := range subset {
		c, ok := byID[id]
		if !ok {
			return nil, "unknown category: " + id
		}
		if !enabledSet[id] {
			return nil, "category not enabled: " + id
		}
		out = append(out, c)
	}
	return out, ""
}
