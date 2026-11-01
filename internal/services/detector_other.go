//go:build !windows

package services

type rawCandidate struct {
	Path       string
	Source     string
	SourceName string
}

// DetectPlatformGameDirs 在非 Windows 平台上的空实现 / 兜底实现
func (s *GameService) DetectPlatformGameDirs() []rawCandidate {
	return nil
}
