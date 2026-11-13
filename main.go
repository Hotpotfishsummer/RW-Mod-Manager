package main

import (
	"embed"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"rw_mod_manager/internal/applog"
	"rw_mod_manager/internal/models"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	_ = applog.Init("info")
	defer applog.Close()

	app := NewApp()
	logLvl := applog.GetLevel()
	wailsLogLvl := applog.ToWailsLogLevel(logLvl)

	applog.Info("App", "========== 应用启动 v%s (%s/%s, LogLevel=%s, LogFile=%s) ==========",
		models.AppVersion, runtime.GOOS, runtime.GOARCH, logLvl, applog.GetLogFilePath())

	err := wails.Run(&options.App{
		Title:            "Rusted Warfare Mod & Map Manager | 铁锈战争 模组与地图管理器",
		Width:            1240,
		Height:           820,
		MinWidth:         960,
		MinHeight:        640,
		Frameless:        true,
		CSSDragProperty:  "--wails-draggable",
		CSSDragValue:     "drag",
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 1},
		Logger:           applog.NewWailsLogger(),
		LogLevel:         wailsLogLvl,
		LogLevelProduction: wailsLogLvl,
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app.AssetHandler(),
		},
		EnableDefaultContextMenu: true,
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			BackdropType:                      windows.Mica,
			DisableFramelessWindowDecorations: false,
			Theme:                             windows.SystemDefault,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		applog.Error("App", "Wails 运行异常退出: %v", err)
		println("Error:", err.Error())
	}
}
