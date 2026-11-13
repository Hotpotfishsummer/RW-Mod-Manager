package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"rw_mod_manager/internal/applog"
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
		// 1. 优先使用 javaw.exe，避免 Windows 弹出额外黑框控制台
		javaExe := filepath.Join(gameDir, "jvm64", "bin", "javaw.exe")
		if _, err := os.Stat(javaExe); os.IsNotExist(err) {
			javaExe = filepath.Join(gameDir, "jvm64", "bin", "java.exe")
		}
		if _, err := os.Stat(javaExe); os.IsNotExist(err) {
			// 回退到 32 位 jvm
			javaExe = filepath.Join(gameDir, "jvm", "bin", "javaw.exe")
			if _, err2 := os.Stat(javaExe); os.IsNotExist(err2) {
				javaExe = filepath.Join(gameDir, "jvm", "bin", "java.exe")
			}
		}

		var args []string
		memMb := 4096
		if cfg.MaxMemoryMB > 0 {
			memMb = cfg.MaxMemoryMB
		}

		// 2. 初始堆与最大堆调优：设置初始堆 -Xms，避免大型模组初始化时频繁扩容触发 STW 卡顿
		initMemMb := memMb / 4
		if initMemMb < 512 {
			initMemMb = 512
		} else if initMemMb > 2048 {
			initMemMb = 2048
		}

		args = append(args,
			fmt.Sprintf("-Xms%dM", initMemMb),
			fmt.Sprintf("-Xmx%dM", memMb),
			"-XX:+UseG1GC",
			"-XX:MaxGCPauseMillis=100",
			"-Dfile.encoding=UTF-8",
			"-Djava.library.path=.",
			"-cp", "game-lib.jar;libs/*",
			"com.corrodinggames.rts.java.Main",
		)

		// 3. 自动追加 -log lastrun.log，保证游戏内日志和崩溃堆栈正常输出
		if !strings.Contains(cfg.ExtraArgs, "-log") {
			args = append(args, "-log", "lastrun.log")
		}

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
	applog.Info("LaunchService", "拉起游戏进程: type=%s, dir=%s, exe=%s", cfg.ExecutableType, gameDir, cmd.Path)
	applog.Debug("LaunchService", "启动参数: %v", cmd.Args)
	if err := cmd.Start(); err != nil {
		applog.Error("LaunchService", "启动游戏进程失败: %v", err)
		return err
	}
	if cmd.Process != nil {
		applog.Info("LaunchService", "游戏进程已启动, PID=%d", cmd.Process.Pid)
	}
	return nil
}
