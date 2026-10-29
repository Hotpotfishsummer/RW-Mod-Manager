/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Material Design 3 (M3) Dark Theme Color System
        m3: {
          // Surface tiers (层次系统：由深到底依次抬升)
          surface: '#0f1319',
          'surface-dim': '#0a0d12',
          'surface-bright': '#232b36',
          'surface-low': '#131821',
          'surface-container': '#18202b',
          'surface-high': '#1e2837',
          'surface-highest': '#253245',

          // Outlines & Borders
          outline: '#2e3d52',
          'outline-variant': '#1f2a38',

          // Typography / On Surfaces
          'on-surface': '#f1f5f9',
          'on-surface-variant': '#94a3b8',
          'on-surface-dim': '#64748b',

          // Primary (Material 3 电竞/工具蓝：清爽深邃、高识别度)
          primary: '#4d88ff',
          'primary-hover': '#6398ff',
          'on-primary': '#ffffff',
          'primary-container': '#1b3252',
          'on-primary-container': '#d3e4ff',

          // Secondary (平衡冷灰蓝)
          secondary: '#90a2b8',
          'on-secondary': '#111a24',
          'secondary-container': '#202b3a',
          'on-secondary-container': '#d5e1ee',

          // Tertiary (青冷强调色)
          tertiary: '#5bc0de',
          'tertiary-container': '#163140',
          'on-tertiary-container': '#d0f0fd',

          // Semantic Roles
          success: '#34d399',
          'success-container': '#102e21',
          'on-success-container': '#a7f3d0',

          warning: '#fbbf24',
          'warning-container': '#30260e',
          'on-warning-container': '#fef08a',

          error: '#f87171',
          'error-container': '#38171b',
          'on-error-container': '#fecaca',
        },
        // 保留 rw 别名以平滑过渡
        rw: {
          dark: '#0f1319',
          card: '#18202b',
          border: '#2e3d52',
          primary: '#4d88ff',
          accent: '#34d399',
          warning: '#fbbf24',
          danger: '#f87171',
        }
      },
      fontFamily: {
        sans: [
          '"Google Sans"',
          '"Segoe UI"',
          'Roboto',
          '-apple-system',
          'BlinkMacSystemFont',
          '"Microsoft YaHei"',
          'sans-serif'
        ],
      },
      boxShadow: {
        'm3-1': '0 1px 3px 1px rgba(0, 0, 0, 0.25), 0 1px 2px 0 rgba(0, 0, 0, 0.3)',
        'm3-2': '0 2px 6px 2px rgba(0, 0, 0, 0.25), 0 1px 2px 0 rgba(0, 0, 0, 0.35)',
        'm3-3': '0 4px 12px 3px rgba(0, 0, 0, 0.3), 0 2px 4px 0 rgba(0, 0, 0, 0.4)',
      }
    },
  },
  plugins: [],
}
