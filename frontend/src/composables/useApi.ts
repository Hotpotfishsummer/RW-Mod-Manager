import { ModItem, MapItem, GameStatus, AppConfig, GamePreferences } from '../types/api'

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
          ActivateMapParentMod(parentModFile: string): Promise<void>
          GenerateMapThumbnail(tmxPath: string): Promise<string>
          BatchGenerateThumbnails(): Promise<number>
          GetGamePreferences(): Promise<GamePreferences>
          SaveGamePreferences(prefs: GamePreferences): Promise<void>
          LaunchGame(): Promise<void>
          OpenFolder(folderType: string): Promise<void>
        }
      }
    }
  }
}

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
      },
      gamePreferences: {
        highRefreshRate: true,
        renderVsync: true,
        showFps: true,
        renderSmoothDelta: false,
        renderAntiAlias: true,
        renderFancyWater: false,
        renderClouds: true,
        renderExtraShadows: true,
        teamShaders: true,
        newRender: false,
        teamUnitCapSinglePlayer: 5000,
        teamUnitCapHostedGame: 2000,
        uiRenderScale: 1.0
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
      totalMods: 19,
      enabledMods: 3,
      disabledMods: 16,
      totalMaps: 306,
      activeMaps: 163
    }
  }

  const getGameStatus = async (): Promise<GameStatus> => {
    if (isWails()) return window.go!.main.App.GetGameStatus()
    return {
      gameDir: 'E:\\Rusted Warfare\\Rusted Warfare',
      isValid: true,
      version: '1.15',
      totalMods: 19,
      enabledMods: 3,
      disabledMods: 16,
      totalMaps: 306,
      activeMaps: 163
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

  const activateMapParentMod = async (parentModFile: string): Promise<void> => {
    if (isWails()) return window.go!.main.App.ActivateMapParentMod(parentModFile)
    console.log(`Mock activateMapParentMod: ${parentModFile}`)
  }

  const generateMapThumbnail = async (tmxPath: string): Promise<string> => {
    if (isWails()) return window.go!.main.App.GenerateMapThumbnail(tmxPath)
    return ''
  }

  const batchGenerateThumbnails = async (): Promise<number> => {
    if (isWails()) return window.go!.main.App.BatchGenerateThumbnails()
    return 0
  }

  const getGamePreferences = async (): Promise<GamePreferences> => {
    if (isWails()) return window.go!.main.App.GetGamePreferences()
    return {
      highRefreshRate: true,
      renderVsync: true,
      showFps: true,
      renderSmoothDelta: false,
      renderAntiAlias: true,
      renderFancyWater: false,
      renderClouds: true,
      renderExtraShadows: true,
      teamShaders: true,
      newRender: false,
      teamUnitCapSinglePlayer: 5000,
      teamUnitCapHostedGame: 2000,
      uiRenderScale: 1.0
    }
  }

  const saveGamePreferences = async (prefs: GamePreferences): Promise<void> => {
    if (isWails()) return window.go!.main.App.SaveGamePreferences(prefs)
    console.log('Mock saveGamePreferences:', prefs)
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
    activateMapParentMod,
    generateMapThumbnail,
    batchGenerateThumbnails,
    getGamePreferences,
    saveGamePreferences,
    launchGame,
    openFolder
  }
}
