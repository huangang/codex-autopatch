package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

func parseDefaultOrder(text string) []string {
	pattern := regexp.MustCompile(`DEFAULT_MODEL_ORDER=\[([^\]]+)\]`)
	match := pattern.FindStringSubmatch(text)
	if match == nil {
		return []string{}
	}
	parts := strings.Split(match[1], ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			items = append(items, value)
		}
	}
	return items
}

func findCodexMaxVersions(text string) []string {
	pattern := regexp.MustCompile(`gpt-[0-9](?:\.[0-9]+)?-codex-max`)
	matches := pattern.FindAllString(text, -1)
	unique := map[string]struct{}{}
	for _, match := range matches {
		unique[match] = struct{}{}
	}
	versions := make([]string, 0, len(unique))
	for value := range unique {
		versions = append(versions, value)
	}
	sort.Slice(versions, func(i, j int) bool {
		ai := versionParts(versions[i])
		aj := versionParts(versions[j])
		max := len(ai)
		if len(aj) > max {
			max = len(aj)
		}
		for idx := 0; idx < max; idx++ {
			var left, right int
			if idx < len(ai) {
				left = ai[idx]
			}
			if idx < len(aj) {
				right = aj[idx]
			}
			if left != right {
				return left > right
			}
		}
		return false
	})
	return versions
}

func findGpt5Models(text string) []string {
	pattern := regexp.MustCompile(`gpt-5[\w\.-]*`)
	matches := pattern.FindAllString(text, -1)
	unique := map[string]struct{}{}
	for _, match := range matches {
		unique[match] = struct{}{}
	}
	models := make([]string, 0, len(unique))
	for value := range unique {
		models = append(models, value)
	}
	sort.Strings(models)
	return models
}

func stripQuotes(name string) string {
	trimmed := strings.TrimSpace(name)
	trimmed = strings.TrimPrefix(trimmed, "\"")
	trimmed = strings.TrimPrefix(trimmed, "'")
	trimmed = strings.TrimSuffix(trimmed, "\"")
	trimmed = strings.TrimSuffix(trimmed, "'")
	return trimmed
}

func normalizeName(name string) string {
	raw := stripQuotes(name)
	pattern := regexp.MustCompile(`^(gpt-5)-([0-9]+)([\w\.-]*)$`)
	match := pattern.FindStringSubmatch(raw)
	if match == nil {
		return raw
	}
	return fmt.Sprintf("%s.%s%s", match[1], match[2], match[3])
}

func quote(name string) string {
	return fmt.Sprintf("\"%s\"", normalizeName(name))
}

func versionTuple(name string) []int {
	pattern := regexp.MustCompile(`gpt-5[.-]([0-9]+(?:\.[0-9]+)?)`)
	match := pattern.FindStringSubmatch(name)
	if match == nil {
		return []int{0}
	}
	parts := strings.Split(strings.ReplaceAll(match[1], "-", "."), ".")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		value, _ := strconv.Atoi(part)
		result = append(result, value)
	}
	return result
}

func modelSortKey(name string) ([]int, int, string) {
	version := versionTuple(name)
	for idx := range version {
		version[idx] = -version[idx]
	}
	category := 2
	if strings.Contains(name, "codex-max") {
		category = 0
	} else if strings.Contains(name, "codex") && !strings.Contains(name, "mini") {
		category = 1
	} else if strings.Contains(name, "codex-mini") {
		category = 3
	}
	return version, category, name
}

func compareTuples(left, right []int) int {
	max := len(left)
	if len(right) > max {
		max = len(right)
	}
	for i := 0; i < max; i++ {
		if i >= len(left) {
			return -1
		}
		if i >= len(right) {
			return 1
		}
		if left[i] != right[i] {
			if left[i] < right[i] {
				return -1
			}
			return 1
		}
	}
	return len(left) - len(right)
}

func orderModels(models []string) []string {
	normalized := map[string]struct{}{}
	for _, model := range models {
		if stripQuotes(model) != "" {
			normalized[normalizeName(model)] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(normalized))
	for model := range normalized {
		ordered = append(ordered, model)
	}
	sort.Slice(ordered, func(i, j int) bool {
		aiVersion, aiCategory, aiName := modelSortKey(ordered[i])
		ajVersion, ajCategory, ajName := modelSortKey(ordered[j])
		if cmp := compareTuples(aiVersion, ajVersion); cmp != 0 {
			return cmp < 0
		}
		if aiCategory != ajCategory {
			return aiCategory < ajCategory
		}
		return aiName < ajName
	})
	result := make([]string, 0, len(ordered))
	for _, model := range ordered {
		result = append(result, quote(model))
	}
	return result
}

func buildApikeyList(text string, includeMini bool) []string {
	defaultOrder := parseDefaultOrder(text)
	for i, item := range defaultOrder {
		defaultOrder[i] = stripQuotes(item)
	}
	codexVersions := findCodexMaxVersions(text)
	gpt5Models := findGpt5Models(text)

	candidates := map[string]struct{}{}
	for _, item := range gpt5Models {
		candidates[item] = struct{}{}
	}
	for _, item := range defaultOrder {
		candidates[item] = struct{}{}
	}
	for _, item := range codexVersions {
		candidates[item] = struct{}{}
	}
	if len(candidates) == 0 {
		candidates["gpt-5.1-codex-max"] = struct{}{}
	}
	if !includeMini {
		filtered := map[string]struct{}{}
		for item := range candidates {
			if !strings.Contains(strings.ToLower(item), "mini") {
				filtered[item] = struct{}{}
			}
		}
		candidates = filtered
	}

	models := make([]string, 0, len(candidates))
	for item := range candidates {
		models = append(models, item)
	}
	return orderModels(models)
}

func replaceAuthMethodArray(text, field string, newItems []string) (string, bool) {
	newArray := fmt.Sprintf("[%s]", strings.Join(newItems, ","))
	newField := fmt.Sprintf("%s:%s", field, newArray)

	patternArray := regexp.MustCompile(field + `:\s*\[[^\]]*\]`)
	if patternArray.MatchString(text) {
		return patternArray.ReplaceAllString(text, newField), true
	}

	patternVar := regexp.MustCompile(field + `:[A-Z][A-Z0-9_]*`)
	if patternVar.MatchString(text) {
		return patternVar.ReplaceAllString(text, newField), true
	}

	return text, false
}

func ensureApikey(text string, includeMini bool) (string, bool) {
	newList := buildApikeyList(text, includeMini)
	return replaceAuthMethodArray(text, "apikey", newList)
}

func ensureChatgpt(text string, includeMini bool) (string, bool) {
	newList := buildApikeyList(text, includeMini)
	return replaceAuthMethodArray(text, "chatgpt", newList)
}

func removeAuthOnly(text string) (string, bool) {
	pattern := regexp.MustCompile(`CHAT_GPT_AUTH_ONLY_MODELS=new Set\(\[([^\]]*?)\]\)`)
	match := pattern.FindStringSubmatchIndex(text)
	if match == nil {
		return text, false
	}
	content := text[match[2]:match[3]]
	if strings.TrimSpace(content) == "" {
		return text, false
	}
	replacement := "CHAT_GPT_AUTH_ONLY_MODELS=new Set([])"
	return text[:match[0]] + replacement + text[match[1]:], true
}

func patchFile(filePath string, includeMini bool) {
	backupPath := filePath + ".bak"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		copyFile(filePath, backupPath)
		fmt.Printf("[backup]  %s\n", backupPath)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("[error]   %s\n", err.Error())
		return
	}
	text := string(content)
	changedApikey := false
	changedChatgpt := false
	changedAuth := false

	text, changedApikey = ensureApikey(text, includeMini)
	text, changedChatgpt = ensureChatgpt(text, includeMini)
	text, changedAuth = removeAuthOnly(text)

	if changedApikey || changedChatgpt || changedAuth {
		if err := os.WriteFile(filePath, []byte(text), 0o644); err != nil {
			fmt.Printf("[error]   %s\n", err.Error())
			return
		}
		changes := []string{}
		if changedApikey {
			changes = append(changes, "apikey")
		}
		if changedChatgpt {
			changes = append(changes, "chatgpt")
		}
		if changedAuth {
			changes = append(changes, "auth_only")
		}
		fmt.Printf("[patched] %s (%s)\n", filePath, strings.Join(changes, ", "))
	} else {
		fmt.Printf("[skip]    %s (already compliant)\n", filePath)
	}
}

func autoDiscover() []string {
	roots := []string{filepath.Join(userHomeDir(), ".vscode", "extensions")}
	if runtime.GOOS == "windows" {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile == "" {
			userProfile = userHomeDir()
		}
		roots = append(roots, filepath.Join(userProfile, ".vscode", "extensions"))
	}

	found := []string{}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if !strings.HasPrefix(entry.Name(), "openai.chatgpt") {
				continue
			}
			webview := filepath.Join(root, entry.Name(), "webview", "assets")
			info, err := os.Stat(webview)
			if err != nil || !info.IsDir() {
				continue
			}
			assets, err := os.ReadDir(webview)
			if err != nil {
				continue
			}
			for _, asset := range assets {
				if asset.IsDir() {
					continue
				}
				if match, _ := filepath.Match("index-*.js", asset.Name()); match {
					found = append(found, filepath.Join(webview, asset.Name()))
				}
			}
		}
	}
	return found
}

func autoDiscoverBaks() []string {
	roots := []string{filepath.Join(userHomeDir(), ".vscode", "extensions")}
	if runtime.GOOS == "windows" {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile == "" {
			userProfile = userHomeDir()
		}
		roots = append(roots, filepath.Join(userProfile, ".vscode", "extensions"))
	}

	found := []string{}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if !strings.HasPrefix(entry.Name(), "openai.chatgpt") {
				continue
			}
			webview := filepath.Join(root, entry.Name(), "webview", "assets")
			info, err := os.Stat(webview)
			if err != nil || !info.IsDir() {
				continue
			}
			assets, err := os.ReadDir(webview)
			if err != nil {
				continue
			}
			for _, asset := range assets {
				if asset.IsDir() {
					continue
				}
				if match, _ := filepath.Match("index-*.js.bak", asset.Name()); match {
					found = append(found, filepath.Join(webview, asset.Name()))
				}
			}
		}
	}
	return found
}

func restore(bakFiles []string) int {
	var targets []string
	if len(bakFiles) > 0 {
		targets = bakFiles
	} else {
		targets = autoDiscoverBaks()
	}
	if len(targets) == 0 {
		fmt.Println("没有找到可恢复的 .bak 文件。")
		return 1
	}
	for _, bakPath := range targets {
		if _, err := os.Stat(bakPath); err != nil {
			fmt.Printf("[error]   %s does not exist\n", bakPath)
			continue
		}
		original := strings.TrimSuffix(bakPath, ".bak")
		copyFile(bakPath, original)
		fmt.Printf("[restored] %s <- %s\n", original, bakPath)
	}
	fmt.Println("提示：如仍异常，建议重新安装插件或手动替换原文件。")
	return 0
}

func copyFile(src, dst string) {
	source, err := os.Open(src)
	if err != nil {
		fmt.Printf("[error]   %s\n", err.Error())
		return
	}
	defer source.Close()
	dest, err := os.Create(dst)
	if err != nil {
		fmt.Printf("[error]   %s\n", err.Error())
		return
	}
	defer dest.Close()
	if _, err := io.Copy(dest, source); err != nil {
		fmt.Printf("[error]   %s\n", err.Error())
	}
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func main() {
	args := os.Args[1:]
	files := []string{}
	auto := false
	restoreFlag := false
	includeMini := false

	for _, arg := range args {
		switch arg {
		case "--auto":
			auto = true
		case "--restore":
			restoreFlag = true
		case "--include-mini":
			includeMini = true
		default:
			files = append(files, arg)
		}
	}

	if restoreFlag {
		os.Exit(restore(files))
	}

	targets := []string{}
	if len(files) > 0 {
		targets = append(targets, files...)
	}
	if auto {
		targets = append(targets, autoDiscover()...)
	}

	if len(targets) == 0 {
		fmt.Println("没有找到需要 patch 的文件。请指定文件或使用 --auto。")
		os.Exit(1)
	}

	for _, target := range targets {
		if _, err := os.Stat(target); err != nil {
			fmt.Printf("[error]   %s does not exist\n", target)
			continue
		}
		patchFile(target, includeMini)
	}

	fmt.Println("操作完成。请重启 VS Code 插件以加载新资源。")
}

func versionParts(version string) []int {
	parts := strings.Split(strings.Split(version, "-")[1], ".")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		value, _ := strconv.Atoi(part)
		result = append(result, value)
	}
	return result
}
