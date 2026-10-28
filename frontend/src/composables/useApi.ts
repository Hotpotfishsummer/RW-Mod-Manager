import { ModItem, MapItem, GameStatus, AppConfig } from '../types/api'

// 声明 Wails 注入到 window 对象的全局运行时与绑定
declare global {
  interface Window {
    go?: {
      main: {
        App: {
          GetConfig(): Promise<AppConfig>
          SaveConfig(cfg: AppConfig): Promise<void>
          SelectGameDir(): Promise<string>
          SetGameDir(dir: string): Promise<GameStatus>
          GetGameStatus(): Promise<GameStatus>
          ScanMods(): Promise<ModItem[]>
          ToggleMod(fileName: string, enabled: boolean): Promise<void>
          BatchToggleMods(enableAll: boolean): Promise<void>
          ScanMaps(): Promise<MapItem[]>
          LaunchGame(): Promise<void>
          OpenFolder(folderType: string): Promise<void>
        }
      }
    }
  }
}

// 检查是否在 Wails 原生窗口环境中运行
const isWails = () => typeof window !== 'undefined' && !!window.go?.main?.App

export function useApi() {
  const getConfig = async (): Promise<AppConfig> => {
    if (isWails()) return window.go!.main.App.GetConfig()
    return {
      gameDir: 'E:\\Rusted Warfare\\Rusted Warfare',
      launchConfig: {
        executableType: 'exe64',
        maxMemoryMb: 4096,
        fullscreen: false,
        noMods: false,
        safeMode: false,
        extraArgs: ''
      }
    }
  }

  const saveConfig = async (cfg: AppConfig): Promise<void> => {
    if (isWails()) return window.go!.main.App.SaveConfig(cfg)
    console.log('Mock saveConfig:', cfg)
  }

  const selectGameDir = async (): Promise<string> => {
    if (isWails()) return window.go!.main.App.SelectGameDir()
    return 'E:\\Rusted Warfare\\Rusted Warfare'
  }

  const setGameDir = async (dir: string): Promise<GameStatus> => {
    if (isWails()) return window.go!.main.App.SetGameDir(dir)
    return {
      gameDir: dir,
      isValid: true,
      version: '1.15',
      totalMods: 18,
      enabledMods: 4,
      disabledMods: 14,
      totalMaps: 15
    }
  }

  const getGameStatus = async (): Promise<GameStatus> => {
    if (isWails()) return window.go!.main.App.GetGameStatus()
    return {
      gameDir: 'E:\\Rusted Warfare\\Rusted Warfare',
      isValid: true,
      version: '1.15',
      totalMods: 18,
      enabledMods: 4,
      disabledMods: 14,
      totalMaps: 15
    }
  }

  const scanMods = async (): Promise<ModItem[]> => {
    if (isWails()) return window.go!.main.App.ScanMods()
    return []
  }

  const toggleMod = async (fileName: string, enabled: boolean): Promise<void> => {
    if (isWails()) return window.go!.main.App.ToggleMod(fileName, enabled)
    console.log(`Mock toggleMod: ${fileName} -> ${enabled}`)
  }

  const batchToggleMods = async (enableAll: boolean): Promise<void> => {
    if (isWails()) return window.go!.main.App.BatchToggleMods(enableAll)
    console.log(`Mock batchToggleMods: ${enableAll}`)
  }

  const scanMaps = async (): Promise<MapItem[]> => {
    if (isWails()) return window.go!.main.App.ScanMaps()
    return []
  }

  const launchGame = async (): Promise<void> => {
    if (isWails()) return window.go!.main.App.LaunchGame()
    console.log('Mock launchGame')
  }

  const openFolder = async (folderType: string): Promise<void> => {
    if (isWails()) return window.go!.main.App.OpenFolder(folderType)
    console.log(`Mock openFolder: ${folderType}`)
  }

  return {
    getConfig,
    saveConfig,
    selectGameDir,
    setGameDir,
    getGameStatus,
    scanMods,
    toggleMod,
    batchToggleMods,
    scanMaps,
    launchGame,
    openFolder
  }
}
