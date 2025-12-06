<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
// @ts-ignore
import { StartLagrange } from '../../wailsjs/go/backend/Backend'
// @ts-ignore
import { EventsOn } from '../../wailsjs/runtime/runtime'

const router = useRouter()
const logs = ref<string[]>([])
const qrCode = ref<string>('')

const addLog = (msg: string) => {
  logs.value.push(`[${new Date().toLocaleTimeString()}] ${msg}`)
  if (logs.value.length > 100) logs.value.shift()
}

onMounted(() => {
  // Listen for logs
  EventsOn("Log", (msg: string) => {
    addLog(msg)
  })

  // Listen for QR Code
  EventsOn("LoginQRCode", (data: string) => {
    // data is base64 encoded PNG
    qrCode.value = `data:image/png;base64,${data}`
    addLog("QR Code received - please scan with mobile QQ")
  })

  // Listen for login success
  EventsOn("LoginSuccess", (uin: number) => {
    addLog(`Login successful! UIN: ${uin}`)
    setTimeout(() => {
      router.push('/chat')
    }, 500)
  })

  // Listen for disconnect
  EventsOn("Disconnected", () => {
    addLog("Disconnected from server")
    qrCode.value = ''
  })

  // Listen for message received (navigate to chat)
  EventsOn("MessageReceived", () => {
    if (router.currentRoute.value.path === '/') {
      router.push('/chat')
    }
  })

  // Start backend
  addLog("Starting Lagrange client...")
  StartLagrange()
})
</script>

<template>
  <div class="login-container">
    <div class="nb-box login-box">
      <h1 class="title">LagrangeQQ</h1>
      <p class="subtitle">NTQQ Protocol Client</p>
      
      <div v-if="!qrCode" class="loading-section">
        <div class="spinner"></div>
        <p class="status-text">Initializing...</p>
      </div>
      
      <div v-if="qrCode" class="qr-section">
        <p class="qr-hint">Scan with Mobile QQ</p>
        <img :src="qrCode" class="qr-code nb-box" />
        <p class="qr-tip">Open QQ → Scan → Confirm Login</p>
      </div>

      <div class="logs-section">
        <div class="logs-header">
          <span class="logs-title">Logs</span>
          <span class="logs-count">{{ logs.length }}</span>
        </div>
        <div class="logs">
          <div v-for="(log, i) in logs" :key="i" class="log-item">{{ log }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 20px;
}

.login-box {
  padding: 40px;
  width: 100%;
  max-width: 420px;
  text-align: center;
  background: #fff;
}

.title {
  font-size: 2.5rem;
  margin-bottom: 5px;
  font-weight: 900;
  text-transform: uppercase;
  letter-spacing: 2px;
}

.subtitle {
  color: #666;
  margin-bottom: 30px;
  font-size: 0.9rem;
}

.loading-section {
  padding: 40px 0;
}

.spinner {
  width: 50px;
  height: 50px;
  border: 5px solid #eee;
  border-top-color: var(--accent-color, #6366f1);
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin: 0 auto 20px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.status-text {
  color: #666;
  font-weight: 600;
}

.qr-section {
  margin-bottom: 30px;
}

.qr-hint {
  font-weight: 700;
  margin-bottom: 15px;
  font-size: 1.1rem;
}

.qr-code {
  width: 220px;
  height: 220px;
  margin: 0 auto 15px;
  display: block;
  background: #fff;
  padding: 10px;
}

.qr-tip {
  color: #888;
  font-size: 0.85rem;
}

.logs-section {
  margin-top: 20px;
}

.logs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #f5f5f5;
  border: var(--border-width) solid var(--border-color);
  border-bottom: none;
}

.logs-title {
  font-weight: 700;
  text-transform: uppercase;
  font-size: 0.8rem;
}

.logs-count {
  background: var(--accent-color, #6366f1);
  color: #fff;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 0.75rem;
  font-weight: 700;
}

.logs {
  height: 180px;
  overflow-y: auto;
  text-align: left;
  background: #1a1a2e;
  color: #00ff9d;
  padding: 12px;
  border: var(--border-width) solid var(--border-color);
  font-size: 0.75rem;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  line-height: 1.6;
}

.log-item {
  word-break: break-all;
  margin-bottom: 4px;
}

.log-item:last-child {
  margin-bottom: 0;
}
</style>
