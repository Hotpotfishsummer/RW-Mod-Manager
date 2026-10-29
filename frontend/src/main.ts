import { createApp } from 'vue'
import './style.css'
import App from './App.vue'

const app = createApp(App)
app.config.errorHandler = (err, instance, info) => {
  console.error('Captured Global Vue Error:', err, info)
}
app.mount('#app')

