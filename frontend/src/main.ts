import { createApp } from 'vue'
import './style.css'
import App from './App.vue'

function reportFrontendError(source: string, err: unknown, extra?: string) {
  const detail = err instanceof Error ? (err.stack || err.message) : String(err)
  const msg = extra ? `${extra} | ${detail}` : detail
  try {
    const appBridge = (window as any)?.go?.main?.App
    if (appBridge?.WriteClientLog) {
      appBridge.WriteClientLog('error', source, msg).catch(() => {})
    }
  } catch {
    // ignore bridge errors
  }
}

const app = createApp(App)
app.config.errorHandler = (err, _instance, info) => {
  console.error('Captured Global Vue Error:', err, info)
  reportFrontendError('VueGlobal', err, info)
}

if (typeof window !== 'undefined') {
  window.addEventListener('error', (event) => {
    reportFrontendError('WindowError', event.error || event.message, `${event.filename}:${event.lineno}:${event.colno}`)
  })
  window.addEventListener('unhandledrejection', (event) => {
    reportFrontendError('UnhandledRejection', event.reason)
  })
}

app.mount('#app')

