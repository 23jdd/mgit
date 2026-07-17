package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/23jdd/mgit/repo"
)

type configScope int

const (
	configMerged configScope = iota
	configLocal
	configGlobal
)

type configFile struct {
	Values map[string]string
}

func runConfig(args []string) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	local := fs.Bool("local", false, "读取或写入当前 mgit 仓库配置")
	global := fs.Bool("global", false, "读取或写入用户全局配置")
	list := fs.Bool("list", false, "列出配置")
	unset := fs.Bool("unset", false, "删除配置项")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit config [--local|--global] [--list]")
		fmt.Fprintln(os.Stderr, "      mgit config [--local|--global] <key>")
		fmt.Fprintln(os.Stderr, "      mgit config [--local|--global] <key> <value>")
		fmt.Fprintln(os.Stderr, "      mgit config [--local|--global] --unset <key>")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *local && *global {
		return fmt.Errorf("--local 和 --global 不能同时使用")
	}

	scope := configMerged
	if *local {
		scope = configLocal
	}
	if *global {
		scope = configGlobal
	}

	if *list {
		if fs.NArg() != 0 {
			fs.Usage()
			return fmt.Errorf("--list 不接收 key 或 value")
		}
		return listConfig(scope)
	}
	if *unset {
		if fs.NArg() != 1 {
			fs.Usage()
			return fmt.Errorf("--unset 需要一个 key")
		}
		return unsetConfig(scopeForWrite(scope), fs.Arg(0))
	}

	switch fs.NArg() {
	case 0:
		fs.Usage()
		return nil
	case 1:
		value, ok, err := getConfig(scope, fs.Arg(0))
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("配置不存在：%s", fs.Arg(0))
		}
		fmt.Println(value)
		return nil
	case 2:
		return setConfig(scopeForWrite(scope), fs.Arg(0), fs.Arg(1))
	default:
		fs.Usage()
		return fmt.Errorf("参数过多")
	}
}

func scopeForWrite(scope configScope) configScope {
	if scope == configGlobal {
		return configGlobal
	}
	return configLocal
}

func configPath(scope configScope) (string, error) {
	if scope == configGlobal {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("获取用户目录失败：%w", err)
		}
		return filepath.Join(home, ".mgitconfig"), nil
	}
	return repo.Path("config"), nil
}

func getConfig(scope configScope, key string) (string, bool, error) {
	key, err := normalizeConfigKey(key)
	if err != nil {
		return "", false, err
	}
	if scope == configLocal || scope == configGlobal {
		cfg, err := loadConfigScope(scope)
		if err != nil {
			return "", false, err
		}
		value, ok := cfg.Values[key]
		return value, ok, nil
	}
	localCfg, err := loadConfigScope(configLocal)
	if err != nil {
		return "", false, err
	}
	if value, ok := localCfg.Values[key]; ok {
		return value, true, nil
	}
	globalCfg, err := loadConfigScope(configGlobal)
	if err != nil {
		return "", false, err
	}
	value, ok := globalCfg.Values[key]
	return value, ok, nil
}

func setConfig(scope configScope, key string, value string) error {
	key, err := normalizeConfigKey(key)
	if err != nil {
		return err
	}
	cfg, err := loadConfigScope(scope)
	if err != nil {
		return err
	}
	cfg.Values[key] = value
	return saveConfigScope(scope, cfg)
}

func unsetConfig(scope configScope, key string) error {
	key, err := normalizeConfigKey(key)
	if err != nil {
		return err
	}
	cfg, err := loadConfigScope(scope)
	if err != nil {
		return err
	}
	if _, ok := cfg.Values[key]; !ok {
		return fmt.Errorf("配置不存在：%s", key)
	}
	delete(cfg.Values, key)
	return saveConfigScope(scope, cfg)
}

func listConfig(scope configScope) error {
	values := map[string]string{}
	if scope == configMerged || scope == configGlobal {
		globalCfg, err := loadConfigScope(configGlobal)
		if err != nil {
			return err
		}
		for key, value := range globalCfg.Values {
			values[key] = value
		}
	}
	if scope == configMerged || scope == configLocal {
		localCfg, err := loadConfigScope(configLocal)
		if err != nil {
			return err
		}
		for key, value := range localCfg.Values {
			values[key] = value
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("%s=%s\n", key, values[key])
	}
	return nil
}

func loadConfigScope(scope configScope) (*configFile, error) {
	path, err := configPath(scope)
	if err != nil {
		return nil, err
	}
	return loadConfigFile(path)
}

func saveConfigScope(scope configScope, cfg *configFile) error {
	if scope == configLocal {
		if err := repo.Mark(); err != nil {
			return fmt.Errorf("创建 mgit 标记失败：%w", err)
		}
	}
	path, err := configPath(scope)
	if err != nil {
		return err
	}
	return saveConfigFile(path, cfg)
}

func loadConfigFile(path string) (*configFile, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return &configFile{Values: map[string]string{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取 config 失败：%w", err)
	}
	defer file.Close()

	cfg := &configFile{Values: map[string]string{}}
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.Trim(strings.TrimSpace(value), "\"")
		key := name
		if section != "" {
			key = section + "." + name
		}
		key, err := normalizeConfigKey(key)
		if err != nil {
			continue
		}
		cfg.Values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("解析 config 失败：%w", err)
	}
	return cfg, nil
}

func saveConfigFile(path string, cfg *configFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建 config 目录失败：%w", err)
	}
	sections := map[string]map[string]string{}
	for key, value := range cfg.Values {
		section, name, ok := strings.Cut(key, ".")
		if !ok || section == "" || name == "" {
			continue
		}
		if sections[section] == nil {
			sections[section] = map[string]string{}
		}
		sections[section][name] = value
	}
	sectionNames := make([]string, 0, len(sections))
	for section := range sections {
		sectionNames = append(sectionNames, section)
	}
	sort.Strings(sectionNames)

	var builder strings.Builder
	for i, section := range sectionNames {
		if i > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString("[")
		builder.WriteString(section)
		builder.WriteString("]\n")
		names := make([]string, 0, len(sections[section]))
		for name := range sections[section] {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			builder.WriteByte('\t')
			builder.WriteString(name)
			builder.WriteString(" = ")
			builder.WriteString(quoteConfigValue(sections[section][name]))
			builder.WriteByte('\n')
		}
	}
	return os.WriteFile(path, []byte(builder.String()), 0o644)
}

func normalizeConfigKey(key string) (string, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	section, name, ok := strings.Cut(key, ".")
	if !ok || section == "" || name == "" || strings.Contains(name, ".") {
		return "", fmt.Errorf("无效配置 key：%q，格式应为 section.name", key)
	}
	if strings.ContainsAny(section+name, " \t\r\n[]=;") {
		return "", fmt.Errorf("无效配置 key：%q", key)
	}
	return section + "." + name, nil
}

func quoteConfigValue(value string) string {
	if value == "" || strings.ContainsAny(value, "#;\t\r\n\"") || strings.TrimSpace(value) != value {
		value = strings.ReplaceAll(value, "\\", "\\\\")
		value = strings.ReplaceAll(value, "\"", "\\\"")
		return "\"" + value + "\""
	}
	return value
}
