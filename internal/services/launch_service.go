package services

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"rw_mod_manager/internal/models"
)

// LaunchService 负责组装启动参数并拉起游戏进程
type LaunchService struct{}

func NewLaunchService() *LaunchService {
	return &LaunchService{}
}

// LaunchGame 根据配置启动铁锈战争
func (s *LaunchService) LaunchGame(gameDir string, cfg models.LaunchConfig) error {
	var cmd *exec.Cmd

	switch cfg.ExecutableType {
	case "java":
		// 使用内置 JVM 启动模式 (可自定义 -Xmx 内存)
		javaExe := filepath.Join(gameDir, "jvm64", "bin", "java.exe")
		var args []string
		memMb := 4096
		if cfg.MaxMemoryMB > 0 {
			memMb = cfg.MaxMemoryMB
		}
		args = append(args, fmt.Sprintf("-Xmx%dM", memMb))
		args = append(args, "-Dfile.encoding=UTF-8", "-Djava.library.path=.", "-cp", "game-lib.jar;libs/*", "com.corrodinggames.rts.java.Main")

		if cfg.Fullscreen {
			args = append(args, "-fullscreen")
		}
		if cfg.NoMods {
			args = append(args, "-nomods")
		}
		if cfg.SafeMode {
			args = append(args, "-safemode")
		}
		if cfg.ExtraArgs != "" {
			extra := strings.Fields(cfg.ExtraArgs)
			args = append(args, extra...)
		}

		cmd = exec.Command(javaExe, args...)
		cmd.Dir = gameDir

	case "exe32":
		exePath := filepath.Join(gameDir, "Rusted Warfare.exe")
		var args []string
		if cfg.Fullscreen {
			args = append(args, "-fullscreen")
		}
		if cfg.NoMods {
			args = append(args, "-nomods")
		}
		if cfg.SafeMode {
			args = append(args, "-safemode")
		}
		if cfg.ExtraArgs != "" {
			extra := strings.Fields(cfg.ExtraArgs)
			args = append(args, extra...)
		}

		cmd = exec.Command(exePath, args...)
		cmd.Dir = gameDir

	default: // "exe64"
		exePath := filepath.Join(gameDir, "Rusted Warfare - 64.exe")
		var args []string
		if cfg.Fullscreen {
			args = append(args, "-fullscreen")
		}
		if cfg.NoMods {
			args = append(args, "-nomods")
		}
		if cfg.SafeMode {
			args = append(args, "-safemode")
		}
		if cfg.ExtraArgs != "" {
			extra := strings.Fields(cfg.ExtraArgs)
			args = append(args, extra...)
		}

		cmd = exec.Command(exePath, args...)
		cmd.Dir = gameDir
	}

	// 非阻塞异步启动
	return cmd.Start()
}
