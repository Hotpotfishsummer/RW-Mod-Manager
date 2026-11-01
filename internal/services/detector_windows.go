//go:build windows

package services

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type rawCandidate struct {
	Path       string
	Source     string // "steam" | "process" | "drive_scan"
	SourceName string
}

// readRegistryString 从指定的注册表根键和路径中读取字符串值
func readRegistryString(root registry.Key, path, key string) string {
	// 尝试直接读取
	if k, err := registry.OpenKey(root, path, registry.QUERY_VALUE); err == nil {
		defer k.Close()
		if val, _, err := k.GetStringValue(key); err == nil && val != "" {
			return val
		}
	}
	// 尝试 WOW64_64KEY
	if k, err := registry.OpenKey(root, path, registry.QUERY_VALUE|registry.WOW64_64KEY); err == nil {
		defer k.Close()
		if val, _, err := k.GetStringValue(key); err == nil && val != "" {
			return val
		}
	}
	// 尝试 WOW64_32KEY
	if k, err := registry.OpenKey(root, path, registry.QUERY_VALUE|registry.WOW64_32KEY); err == nil {
		defer k.Close()
		if val, _, err := k.GetStringValue(key); err == nil && val != "" {
			return val
		}
	}
	return ""
}

// parseLibraryFoldersVDF 解析 Steam 的 libraryfolders.vdf，提取出所有配置的 Steam 库路径
func parseLibraryFoldersVDF(vdfPath string) []string {
	f, err := os.Open(vdfPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var paths []string
	seen := make(map[string]bool)

	// 匹配 "path" "D:\\Program Files\\Steam" 或 "path" "D:/SteamLibrary"
	rePath := regexp.MustCompile(`(?i)"path"\s+"([^"]+)"`)
	// 匹配旧版 Steam 格式："1" "D:\\SteamLibrary"
	reOldPath := regexp.MustCompile(`(?i)"\d+"\s+"([A-Za-z]:[^"]+)"`)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		var candidate string
		if m := rePath.FindStringSubmatch(line); len(m) > 1 {
			candidate = m[1]
		} else if m := reOldPath.FindStringSubmatch(line); len(m) > 1 {
			candidate = m[1]
		}

		if candidate != "" {
			// 将双反斜杠替换为单反斜杠，并转为标准路径
			clean := filepath.Clean(strings.ReplaceAll(candidate, `\\`, `\`))
			lower := strings.ToLower(clean)
			if !seen[lower] {
				seen[lower] = true
				paths = append(paths, clean)
			}
		}
	}
	return paths
}

// detectRunningGameProcesses 扫描当前正在运行的铁锈战争进程目录
func detectRunningGameProcesses() []string {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	var dirs []string
	seen := make(map[string]bool)

	targetExes := map[string]bool{
		"rusted warfare.exe":          true,
		"rusted warfare - 64.exe":     true,
		"rustedwarfare.exe":           true,
		"rustedwarfare-64.exe":        true,
		"rustedwarfare-dedicated.exe": true,
	}

	err = windows.Process32First(snapshot, &entry)
	for err == nil {
		exeName := strings.ToLower(syscall.UTF16ToString(entry.ExeFile[:]))
		if targetExes[exeName] {
			hProc, errProc := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
			if errProc == nil {
				buf := make([]uint16, windows.MAX_LONG_PATH)
				size := uint32(len(buf))
				if errQuery := windows.QueryFullProcessImageName(hProc, 0, &buf[0], &size); errQuery == nil && size > 0 {
					fullPath := syscall.UTF16ToString(buf[:size])
					dir := filepath.Dir(fullPath)
					cleanDir := filepath.Clean(dir)
					lower := strings.ToLower(cleanDir)
					if !seen[lower] {
						seen[lower] = true
						dirs = append(dirs, cleanDir)
					}
				}
				windows.CloseHandle(hProc)
			}
		}
		err = windows.Process32Next(snapshot, &entry)
	}
	return dirs
}

// detectLogicalDrives 获取所有已挂载的逻辑磁盘驱动器根目录
func detectLogicalDrives() []string {
	bitmask, err := windows.GetLogicalDrives()
	if err != nil {
		// 回退常见盘符
		return []string{`C:\`, `D:\`, `E:\`, `F:\`, `G:\`}
	}

	var drives []string
	for i := 0; i < 26; i++ {
		if bitmask&(1<<i) != 0 {
			letter := string(rune('A' + i))
			root := letter + `:\`
			if _, err := os.Stat(root); err == nil {
				drives = append(drives, root)
			}
		}
	}
	return drives
}

// DetectPlatformGameDirs 在 Windows 平台上全面探测铁锈战争可能的安装路径
func (s *GameService) DetectPlatformGameDirs() []rawCandidate {
	var candidates []rawCandidate
	seen := make(map[string]bool)

	addCandidate := func(dir, source, sourceName string) {
		if dir == "" {
			return
		}
		clean := filepath.Clean(dir)
		lower := strings.ToLower(clean)
		if seen[lower] {
			return
		}
		seen[lower] = true
		candidates = append(candidates, rawCandidate{
			Path:       clean,
			Source:     source,
			SourceName: sourceName,
		})
	}

	// 1. 探测 Steam App 647960 官方卸载注册表 (高精确度)
	appRegPaths := []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\Steam App 647960`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\Steam App 647960`},
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall\Steam App 647960`},
	}
	for _, reg := range appRegPaths {
		if loc := readRegistryString(reg.root, reg.path, "InstallLocation"); loc != "" {
			addCandidate(loc, "steam", "Steam 注册表")
		}
	}

	// 2. 探测 Steam 安装路径并解析 libraryfolders.vdf
	var steamRoots []string
	steamRegPaths := []struct {
		root registry.Key
		path string
		key  string
	}{
		{registry.CURRENT_USER, `Software\Valve\Steam`, "SteamPath"},
		{registry.LOCAL_MACHINE, `SOFTWARE\Valve\Steam`, "InstallPath"},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, "InstallPath"},
	}
	for _, reg := range steamRegPaths {
		if p := readRegistryString(reg.root, reg.path, reg.key); p != "" {
			steamRoots = append(steamRoots, filepath.Clean(p))
		}
	}

	// 解析全部 Steam 库
	var steamLibraries []string
	for _, sr := range steamRoots {
		steamLibraries = append(steamLibraries, sr)
		vdfPath := filepath.Join(sr, "steamapps", "libraryfolders.vdf")
		for _, lib := range parseLibraryFoldersVDF(vdfPath) {
			steamLibraries = append(steamLibraries, lib)
		}
	}

	for _, lib := range steamLibraries {
		drive := filepath.VolumeName(lib)
		libName := fmt.Sprintf("Steam 库 (%s)", drive)
		if drive == "" {
			libName = "Steam 库"
		}

		// 检查常规命名
		commonNames := []string{"Rusted Warfare", "rusted warfare", "RustedWarfare"}
		for _, name := range commonNames {
			dir := filepath.Join(lib, "steamapps", "common", name)
			addCandidate(dir, "steam", libName)
		}

		// 检查 appmanifest_647960.acf
		acfPath := filepath.Join(lib, "steamapps", "appmanifest_647960.acf")
		if data, err := os.ReadFile(acfPath); err == nil {
			reInstallDir := regexp.MustCompile(`(?i)"installdir"\s+"([^"]+)"`)
			if m := reInstallDir.FindStringSubmatch(string(data)); len(m) > 1 {
				dir := filepath.Join(lib, "steamapps", "common", m[1])
				addCandidate(dir, "steam", libName)
			}
		}
	}

	// 3. 探测当前正在运行的铁锈战争进程
	for _, pDir := range detectRunningGameProcesses() {
		addCandidate(pDir, "process", "运行中游戏进程")
	}

	// 4. 磁盘全量分区扫描常用目录
	commonRelPaths := []string{
		`SteamLibrary\steamapps\common\Rusted Warfare`,
		`Steam\steamapps\common\Rusted Warfare`,
		`Program Files (x86)\Steam\steamapps\common\Rusted Warfare`,
		`Program Files\Steam\steamapps\common\Rusted Warfare`,
		`Games\Steam\steamapps\common\Rusted Warfare`,
		`Games\SteamLibrary\steamapps\common\Rusted Warfare`,
		`Games\Rusted Warfare`,
		`Games\Rusted Warfare\Rusted Warfare`,
		`Rusted Warfare`,
		`Rusted Warfare\Rusted Warfare`,
		`RustedWarfare`,
		`rw`,
	}

	drives := detectLogicalDrives()
	for _, drive := range drives {
		vol := strings.TrimSuffix(drive, `\`)
		sourceName := fmt.Sprintf("磁盘扫描 (%s)", vol)
		for _, rel := range commonRelPaths {
			target := filepath.Join(drive, rel)
			addCandidate(target, "drive_scan", sourceName)
		}
	}

	// 5. 检查用户桌面、下载与文档目录
	if home, err := os.UserHomeDir(); err == nil {
		userDirs := []string{
			filepath.Join(home, "Desktop", "Rusted Warfare"),
			filepath.Join(home, "Downloads", "Rusted Warfare"),
			filepath.Join(home, "Documents", "Rusted Warfare"),
			filepath.Join(home, "Rusted Warfare"),
		}
		for _, ud := range userDirs {
			addCandidate(ud, "drive_scan", "用户个人目录")
		}
	}

	return candidates
}
