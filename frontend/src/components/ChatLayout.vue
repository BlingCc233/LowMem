<script setup lang="ts">
import { onMounted } from 'vue'
import { useChatStore } from '../store/chat'
import ChatWindow from './ChatWindow.vue'

const store = useChatStore()

onMounted(() => {
  store.initSessions()
  store.initListeners()
})

const formatTime = (ts: number) => {
    if (!ts) return ''
    const date = new Date(ts * 1000)
    const now = new Date()
    if (date.toDateString() === now.toDateString()) {
        return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    }
    return date.toLocaleDateString()
}
</script>

<template>
  <div class="layout">
    <div class="sidebar nb-box">
        <div class="sidebar-header">
            <input 
              v-model="store.sessionSearch" 
              class="nb-input search-field" 
              placeholder="搜索会话或好友"
            />
            <button 
              v-if="store.sessionSearch" 
              class="nb-button ghost small" 
              @click="store.clearSessionSearch"
            >
                清除
            </button>
        </div>
        <div class="list-container">
            <div 
              v-for="session in store.orderedSessions" 
              :key="`${session.id}-${session.isGroup}`" 
              class="list-item"
              :class="{ active: store.currentChat?.id === session.id && store.currentChat?.isGroup === session.isGroup }"
              @click="store.selectChat(session.id, session.isGroup, session.name, session.avatar)"
            >
                <img :src="session.avatar" class="avatar" />
                <div class="info">
                    <div class="top-row">
                        <span class="name">{{ session.name }}</span>
                        <span class="time" v-if="session.time">{{ formatTime(session.time) }}</span>
                    </div>
                    <div class="bottom-row">
                        <span class="preview">{{ session.lastMessage || '暂无消息' }}</span>
                        <span v-if="session.unread > 0" class="badge nb-box">{{ session.unread }}</span>
                    </div>
                </div>
                <button 
                  class="pin-btn" 
                  :class="{ pinned: store.isPinned(session.id, session.isGroup) }"
                  @click.stop="store.togglePin(session.id, session.isGroup)"
                  aria-label="Pin chat"
                >
                    <span v-if="store.isPinned(session.id, session.isGroup)">★</span>
                    <span v-else>☆</span>
                </button>
            </div>
        </div>
    </div>
    
    <div class="main-area nb-box">
        <ChatWindow v-if="store.currentChat" />
        <div v-else class="empty-state">
            <p>Select a contact to start chatting</p>
        </div>
    </div>
  </div>
</template>

<style scoped>
.layout {
  display: flex;
  height: 100vh;
  padding: 20px;
  gap: 20px;
  background-color: var(--primary-color);
  box-sizing: border-box;
}

.sidebar {
  width: 300px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.sidebar-header {
  padding: 12px;
  display: flex;
  gap: 10px;
}

.search-field {
  flex: 1;
}

.nb-button.small {
  padding: 6px 12px;
  font-size: 0.75rem;
  text-transform: none;
}

.list-container {
  flex: 1;
  overflow-y: auto;
}

.list-item {
  display: flex;
  align-items: center;
  padding: 12px;
  border-bottom: 2px solid #eee;
  cursor: pointer;
  gap: 12px;
  transition: background 0.1s, border 0.1s;
}
.list-item:hover {
  background: #f9f9f9;
}
.list-item.active {
  background: #e0e7ff;
  border-color: var(--accent-color);
}

.pin-btn {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 1rem;
  margin-left: auto;
  color: #bbb;
  padding: 4px 6px;
}
.pin-btn.pinned {
  color: var(--accent-color);
}

.avatar {
  width: 48px;
  height: 48px;
  border: 2px solid var(--border-color);
  border-radius: 50%;
  object-fit: cover;
}

.info {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.top-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.name {
  font-weight: bold;
  font-size: 1rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.time {
  font-size: 0.75rem;
  color: #666;
  min-width: fit-content;
}

.bottom-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.preview {
  font-size: 0.85rem;
  color: #888;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.badge {
  margin-left: auto;
  min-width: 26px;
  text-align: center;
  padding: 4px 8px;
  background: var(--accent-color);
  color: #fff;
  font-size: 0.8rem;
  font-weight: 800;
}

.main-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.empty-state {
  flex: 1;
  display: flex;
  justify-content: center;
  align-items: center;
  font-size: 1.5rem;
  color: #888;
}
</style>
