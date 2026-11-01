/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Material Design 3 (M3) Color System (Dynamic via CSS Variables)
        m3: {
          // Surface tiers (层次系统：由深到底依次抬升)
          surface: 'rgb(var(--m3-surface) / <alpha-value>)',
          'surface-dim': 'rgb(var(--m3-surface-dim) / <alpha-value>)',
          'surface-bright': 'rgb(var(--m3-surface-bright) / <alpha-value>)',
          'surface-low': 'rgb(var(--m3-surface-low) / <alpha-value>)',
          'surface-container': 'rgb(var(--m3-surface-container) / <alpha-value>)',
          'surface-high': 'rgb(var(--m3-surface-high) / <alpha-value>)',
          'surface-highest': 'rgb(var(--m3-surface-highest) / <alpha-value>)',

          // Outlines & Borders
          outline: 'rgb(var(--m3-outline) / <alpha-value>)',
          'outline-variant': 'rgb(var(--m3-outline-variant) / <alpha-value>)',

          // Typography / On Surfaces
          'on-surface': 'rgb(var(--m3-on-surface) / <alpha-value>)',
          'on-surface-variant': 'rgb(var(--m3-on-surface-variant) / <alpha-value>)',
          'on-surface-dim': 'rgb(var(--m3-on-surface-dim) / <alpha-value>)',

          // Primary (Material 3 电竞/工具蓝：清爽深邃、高识别度)
          primary: 'rgb(var(--m3-primary) / <alpha-value>)',
          'primary-hover': 'rgb(var(--m3-primary-hover) / <alpha-value>)',
          'on-primary': 'rgb(var(--m3-on-primary) / <alpha-value>)',
          'primary-container': 'rgb(var(--m3-primary-container) / <alpha-value>)',
          'on-primary-container': 'rgb(var(--m3-on-primary-container) / <alpha-value>)',

          // Secondary (平衡冷灰蓝)
          secondary: 'rgb(var(--m3-secondary) / <alpha-value>)',
          'on-secondary': 'rgb(var(--m3-on-secondary) / <alpha-value>)',
          'secondary-container': 'rgb(var(--m3-secondary-container) / <alpha-value>)',
          'on-secondary-container': 'rgb(var(--m3-on-secondary-container) / <alpha-value>)',

          // Tertiary (青冷强调色)
          tertiary: 'rgb(var(--m3-tertiary) / <alpha-value>)',
          'tertiary-container': 'rgb(var(--m3-tertiary-container) / <alpha-value>)',
          'on-tertiary-container': 'rgb(var(--m3-on-tertiary-container) / <alpha-value>)',

          // Semantic Roles
          success: 'rgb(var(--m3-success) / <alpha-value>)',
          'success-container': 'rgb(var(--m3-success-container) / <alpha-value>)',
          'on-success-container': 'rgb(var(--m3-on-success-container) / <alpha-value>)',

          warning: 'rgb(var(--m3-warning) / <alpha-value>)',
          'warning-container': 'rgb(var(--m3-warning-container) / <alpha-value>)',
          'on-warning-container': 'rgb(var(--m3-on-warning-container) / <alpha-value>)',

          error: 'rgb(var(--m3-error) / <alpha-value>)',
          'error-container': 'rgb(var(--m3-error-container) / <alpha-value>)',
          'on-error-container': 'rgb(var(--m3-on-error-container) / <alpha-value>)',
        },
        // 保留 rw 别名以平滑过渡
        rw: {
          dark: 'rgb(var(--m3-surface) / <alpha-value>)',
          card: 'rgb(var(--m3-surface-container) / <alpha-value>)',
          border: 'rgb(var(--m3-outline) / <alpha-value>)',
          primary: 'rgb(var(--m3-primary) / <alpha-value>)',
          accent: 'rgb(var(--m3-success) / <alpha-value>)',
          warning: 'rgb(var(--m3-warning) / <alpha-value>)',
          danger: 'rgb(var(--m3-error) / <alpha-value>)',
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
        'm3-1': 'var(--m3-shadow-1)',
        'm3-2': 'var(--m3-shadow-2)',
        'm3-3': 'var(--m3-shadow-3)',
      }
    },
  },
  plugins: [],
}
