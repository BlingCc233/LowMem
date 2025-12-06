<script setup lang="ts">
import { ref, nextTick, watch, onMounted, onUnmounted, computed } from 'vue'
import { useChatStore } from '../store/chat'
import type { Message, MessageElement } from '../types'
// @ts-ignore
import { RecallMessage, OpenImage, SendImageMessage } from '../../wailsjs/go/backend/Backend'

const store = useChatStore()
const inputContent = ref('')
const msgListRef = ref<HTMLElement | null>(null)
const contextMenu = ref<{ visible: boolean; x: number; y: number; msg: Message | null }>({ visible: false, x: 0, y: 0, msg: null })
const replyTo = ref<Message | null>(null)
const atList = ref<number[]>([])
const searchQuery = ref('')

const messageMap = computed(() => {
    const map: Record<number, Message> = {}
    store.messages.forEach((m) => {
        map[m.message_id] = m
    })
    store.searchResults.forEach((m) => {
        map[m.message_id] = m
    })
    for (const k in store.referencedMessages) {
        const id = Number(k)
        map[id] = store.referencedMessages[id]
    }
    return map
})

const scrollToBottom = () => {
    nextTick(() => {
        if (msgListRef.value) {
            msgListRef.value.scrollTop = msgListRef.value.scrollHeight
        }
    })
}

watch(() => store.messages.length, () => {
    scrollToBottom()
})

watch(() => store.messages, (msgs) => {
    msgs.forEach(msg => {
        if (msg.reply_to && !messageMap.value[msg.reply_to]) {
            store.fetchMessage(msg.reply_to)
        }
    })
}, { deep: true, immediate: true })

watch(() => store.currentChat, () => {
    replyTo.value = null
    atList.value = []
    scrollToBottom()
})

const elementList = (msg: Message): MessageElement[] => {
    if (msg.elements && msg.elements.length) return msg.elements
    return [{ type: 'text', text: msg.content }]
}

const escapeHtml = (text: string) => text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')

const formatText = (text: string) => {
    const escaped = escapeHtml(text)
    const urlRegex = /(https?:\/\/[\w.-]+(?:\/[\w+%.-]*)?[^\s<]*)/g
    return escaped.replace(urlRegex, '<a href="$1" target="_blank">$1</a>')
}

const formatTime = (ts: number) => {
    if (!ts) return ''
    const d = new Date(ts * 1000)
    return `${d.toLocaleDateString()} ${d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
}

const send = () => {
    if (!inputContent.value.trim()) return
    // Remove the @UserName part if it exists at the start to prevent double mentions
    let content = inputContent.value
    if (atList.value.length > 0) {
        // Simple heuristic: remove leading @Names
        // In a real app we might want structured input, but for text input:
        const tokens = content.split(' ')
        // Remove tokens that look like @Mention if we have atList
        // Or deeper: NapCat/OneBot might append @User when we send [CQ:at], so we just send the content part if we want
        // But the user sees "@User content". 
        // If we send "[CQ:at] @User content", it shows "@@User content" or two bubbles.
        // We will strip the literal "@User " string from the start if we are strictly using [CQ:at]
        // But actually the store appends [CQ:at], so we should remove the text representation.
        // We'll just trim left for now if it starts with @
    }
    
    // Better approach: When `mention` is clicked, we appended `@Name `.
    // If we detect `atList` is populated, we should probably strip that specific string from `content`.
    // However, user might have deleted it. 
    // Let's rely on the store to handle [CQ:at] and let's try to NOT double-send deeply.
    // Actually the user reported "double at", meaning [CQ:at] + text "@User".
    // We should remove the text part if we are sending the CQ code.
    
    // We will clean manual @ mentions from text if they match our atList
    // Ideally we would parse the input, but let's just send what the user typed w/o explicit CQ codes for now?
    // No, `store.sendMessage` appends CQ codes.
    
    // Correct fix: Scan `atList` and remove corresponding names from `content`
     if (atList.value.length) {
         // This is tricky without the original name map. 
         // Let's just trust the user input string and NOT send the `atList` to store if the string already contains it?
         // Or better: Remove the text "@Name " from content and let store add [CQ:at].
         // Since we don't have the names easily here, we'll try to find keys starting with @.
         // Simpler fix for "Double At": If we have atList, remove the "@Name " prefix if present.
         // But we don't know the Name easily from ID here without looking up.
         
         // Alternative: if atList is present, do NOT append it in `store.ts` and just let the text be?
         // But then it might not be a real mention (just text).
         // The issue is likely `store.ts` doing `parts.push([CQ:at])` AND the content having `@Name`.
         // We will remove the regex `^@\S+\s+` from content.
         content = content.replace(/^@\S+\s+/, '')
    }

    store.sendMessage(content, { replyTo: replyTo.value?.message_id, atList: atList.value })
    inputContent.value = ''
    replyTo.value = null
    atList.value = []
    scrollToBottom()
}

const handlePaste = async (e: ClipboardEvent) => {
    const items = e.clipboardData?.items
    if (!items) return
    for (const item of Array.from(items)) {
        if (item.type.indexOf('image') !== -1) {
            const file = item.getAsFile()
            if (!file) continue
            // We need to upload this file or save it to send.
            // Since we can't easily upload from frontend JS to backend via Wails without a method,
            // we will read as DataURL and pass to backend to save & send.
            const reader = new FileReader()
            reader.onload = async (evt) => {
                const base64 = (evt.target?.result as string).split(',')[1]
                try {
                    // @ts-ignore
                    const path = await window.go.backend.Backend.UploadImage(base64)
                    if (path) {
                        await SendImageMessage(store.currentChat?.id || 0, store.currentChat?.isGroup || false, path)
                         scrollToBottom()
                    }
                } catch (err) {
                    console.error("Paste image failed", err)
                }
            }
            reader.readAsDataURL(file)
            e.preventDefault() // User handled paste
            return
        }
    }
}

const handleDrop = async (e: DragEvent) => {
    const files = e.dataTransfer?.files
    if (!files || files.length === 0) return
    
    for (const file of Array.from(files)) {
        if (file.type.startsWith('image/')) {
             // For drag and drop from OS, we might get actual file path if we were in Electron, but in Browser/Wails we get a File object.
             // Wails 3 might handle native drag easier, but wails 2 usually gives File object.
             // We use the same UploadImage trick.
            const reader = new FileReader()
            reader.onload = async (evt) => {
                const base64 = (evt.target?.result as string).split(',')[1]
                try {
                    // @ts-ignore
                     const path = await window.go.backend.Backend.UploadImage(base64)
                    if (path) {
                        await SendImageMessage(store.currentChat?.id || 0, store.currentChat?.isGroup || false, path)
                        scrollToBottom()
                    }
                } catch (err) {
                     console.error("Drop image failed", err)
                }
            }
             reader.readAsDataURL(file)
        }
    }
}

const sendImage = async () => {
    try {
        const path = await OpenImage()
        if (path) {
            // Send image message with CQ code
            // Note: NapCat accepts file:// absolute path
            const content = `[CQ:image,file=file://${path}]`
            store.sendMessage(content, { replyTo: replyTo.value?.message_id, atList: atList.value })
        }
    } catch(e) {
        console.error(e)
    }
}

const showContextMenu = (e: MouseEvent, msg: Message) => {
    e.preventDefault()
    contextMenu.value = {
        visible: true,
        x: e.clientX,
        y: e.clientY,
        msg,
    }
}

const closeContextMenu = () => {
    contextMenu.value.visible = false
}

const handleDocumentClick = (event: MouseEvent) => {
    if (event.button === 2) return
    closeContextMenu()
}

const recallMsg = async () => {
    if (contextMenu.value.msg?.is_send) {
        await RecallMessage(contextMenu.value.msg.message_id)
    }
    closeContextMenu()
}

const setReply = (msg: Message) => {
    replyTo.value = msg
    closeContextMenu()
}

const mention = (msg: Message) => {
    if (msg.is_send) return
    if (!atList.value.includes(msg.sender_id)) {
        atList.value.push(msg.sender_id)
    }
    inputContent.value = `@${msg.sender?.nickname || msg.sender_id} ` + inputContent.value
    closeContextMenu()
}

const forwardMsg = (msg: Message) => {
    if (!msg.database_id) return
    const target = prompt('输入聊天ID，群聊请以 g 前缀，例如 g123456')
    if (!target) return
    const isGroup = target.startsWith('g') || target.startsWith('G')
    const id = Number(isGroup ? target.slice(1) : target)
    if (Number.isNaN(id)) return
    store.forward(msg.database_id, id, isGroup)
    closeContextMenu()
}

const performSearch = () => {
    store.search(searchQuery.value)
}

const scrollToMessage = (messageId: number) => {
    const existing = store.messages.find((m) => m.message_id === messageId)
    if (!existing) {
        const found = store.searchResults.find((m) => m.message_id === messageId)
        if (found) {
            store.messages.push(found)
            store.messages.sort((a, b) => a.time - b.time)
        }
    }

    nextTick(() => {
        const el = document.getElementById(`msg-${messageId}`)
        if (el) {
            el.scrollIntoView({ behavior: 'smooth', block: 'center' })
        }
    })
}

onMounted(() => {
    document.addEventListener('click', handleDocumentClick)
})

onUnmounted(() => {
    document.removeEventListener('click', handleDocumentClick)
})
</script>

<template>
  <div class="chat-window">
      <div class="header">
          <div class="title-block">
              <h2>{{ store.currentChat?.name }}</h2>
              <span v-if="store.currentChat?.isGroup" class="pill">群聊</span>
          </div>
          <div class="tools">
              <input 
                v-model="searchQuery" 
                @keyup.enter="performSearch"
                class="nb-input search-input" 
                placeholder="搜索聊天记录"
              />
              <button @click="performSearch" class="nb-button ghost">Search</button>
          </div>
      </div>

      <div v-if="store.searchResults.length" class="search-results nb-box">
          <div 
            v-for="item in store.searchResults" 
            :key="item.message_id" 
            class="search-item"
            @click="scrollToMessage(item.message_id)"
          >
              <div class="search-meta">{{ formatTime(item.time) }}</div>
              <div class="search-text">{{ item.content }}</div>
          </div>
      </div>
      
      <div class="messages" ref="msgListRef">
          <div v-if="store.loadingMessages" class="loading">Loading...</div>
          
          <div 
            v-for="msg in store.messages" 
            :key="msg.database_id || msg.time" 
            :id="`msg-${msg.message_id}`"
            class="msg-row"
            :class="{ 'mine': msg.sender_id === 0 || msg.is_send }"
            @contextmenu="showContextMenu($event, msg as Message)"
          >
              <img v-if="msg.sender_id !== 0 && !msg.is_send" :src="msg.sender?.avatar_url" class="avatar" />
              <div class="bubble nb-box" :class="{ recalled: msg.recall }">
                  <div class="meta">
                      <span class="sender">{{ msg.is_send ? '我' : (msg.sender?.nickname || '好友') }}</span>
                      <span class="time">{{ formatTime(msg.time) }}</span>
                  </div>
                  <div v-if="msg.recall" class="recall-text">消息已撤回</div>
                  <template v-else>
                      <div v-if="msg.reply_to" class="reply-preview nb-box">
                          <span class="label">回复</span>
                          <div class="reply-body">{{ messageMap[msg.reply_to]?.content || '引用的消息' }}</div>
                      </div>
                      <div class="content">
                          <template v-for="(el, idx) in elementList(msg)" :key="idx">
                              <span v-if="el.type === 'text'" class="text" v-html="formatText(el.text || '')"></span>
                              <span v-else-if="el.type === 'at'" class="at-tag">@{{ el.name || el.qq }}</span>
                              <img v-else-if="el.type === 'image'" :src="el.url || el.file" class="msg-image nb-box" @load="scrollToBottom" />
                              <audio v-else-if="el.type === 'voice'" controls :src="el.url || el.file" class="voice"></audio>
                              <span v-else-if="el.type === 'face'">
                                  <img v-if="el.id" :src="`https://raw.githubusercontent.com/kyubotics/coolq-http-api/master/docs/face/${el.id}.png`" style="width:24px;vertical-align:middle" :alt="`[表情${el.id}]`" @error="(e:Event)=>(e.target as HTMLImageElement).style.display='none'" />
                                  <span v-else>[表情]</span>
                              </span>
                              <span v-else class="pill">{{ el.type }}</span>
                          </template>
                      </div>
                  </template>
              </div>
          </div>
      </div>
      
      <div class="input-area" 
        @paste="handlePaste" 
        @drop.prevent="handleDrop" 
        @dragover.prevent
      >
          <div v-if="replyTo" class="replying nb-box">
              <div class="replying-text">回复 {{ replyTo.sender?.nickname || '我' }}</div>
              <button class="link-btn" @click="replyTo = null">取消</button>
          </div>
          <div class="toolbar">
             <!-- <button @click="sendImage" class="icon-btn">📷</button> -->
          </div>
          <input 
            v-model="inputContent" 
            @keyup.enter="send"
            class="nb-input chat-input" 
            placeholder="输入消息，可@或回复"
          />
          <button @click="send" class="nb-button">Send</button>
      </div>

      <div 
        v-if="contextMenu.visible" 
        class="context-menu nb-box"
        :style="{ top: contextMenu.y + 'px', left: contextMenu.x + 'px' }"
        @click.stop
      >
          <div class="menu-item" @click="setReply(contextMenu.msg as Message)">回复</div>
          <div class="menu-item" v-if="contextMenu.msg && !contextMenu.msg.is_send" @click="mention(contextMenu.msg as Message)">@Ta</div>
          <div class="menu-item" @click="forwardMsg(contextMenu.msg as Message)">转发</div>
          <div class="menu-item" v-if="contextMenu.msg?.is_send" @click="recallMsg">撤回</div>
      </div>
  </div>
 </template>

<style scoped>
.chat-window {
    display: flex;
    flex-direction: column;
    height: 100%;
    position: relative;
    background: linear-gradient(135deg, #fef3c7 0%, #e0f2fe 100%);
    border: var(--border-width) solid var(--border-color);
}

.header {
    padding: 15px;
    border-bottom: var(--border-width) solid var(--border-color);
    background: #fefefe;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
}
.title-block {
    display: flex;
    align-items: center;
    gap: 10px;
}
.header h2 {
    margin: 0;
    font-size: 1.2rem;
}
.tools {
    display: flex;
    align-items: center;
    gap: 8px;
}
.search-input {
    width: 220px;
}

.search-results {
    max-height: 150px;
    overflow-y: auto;
    margin: 8px 16px 0;
    padding: 8px;
    background: #fff;
}
.search-item {
    padding: 6px 8px;
    cursor: pointer;
    border-bottom: 1px dashed var(--border-color);
}
.search-item:last-child {
    border-bottom: none;
}
.search-item:hover {
    background: #f0f9ff;
}
.search-meta {
    font-size: 0.75rem;
    color: #555;
}
.search-text {
    font-weight: 700;
}

.messages {
    flex: 1;
    overflow-y: auto;
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 15px;
    background: transparent;
}

.msg-row {
    display: flex;
    align-items: flex-end;
    gap: 10px;
    max-width: 70%;
}

.msg-row.mine {
    align-self: flex-end;
    flex-direction: row-reverse;
}

.avatar {
    width: 36px;
    height: 36px;
    border: 2px solid black;
    border-radius: 50%;
}

.bubble {
    padding: 10px 15px;
    background: #fff;
    border-radius: 10px;
    border: var(--border-width) solid var(--border-color);
    box-shadow: var(--shadow-X) var(--shadow-Y) 0 var(--border-color);
    min-width: 200px;
}
.bubble.recalled {
    background: #f5f5f5;
    color: #777;
}

.msg-row.mine .bubble {
    background: var(--secondary-color);
    color: white;
    border-bottom-left-radius: 10px;
    border-bottom-right-radius: 0;
}

.meta {
    display: flex;
    justify-content: space-between;
    font-size: 0.75rem;
    margin-bottom: 6px;
    color: #555;
}
.msg-row.mine .meta {
    color: #e5e5e5;
}

.content {
    display: flex;
    flex-direction: column;
    gap: 6px;
    word-break: break-all;
}

.text a {
    color: var(--accent-color);
    text-decoration: underline;
    word-break: break-all;
}

.msg-image {
    max-width: 260px;
    border: var(--border-width) solid var(--border-color);
}

.reply-preview {
    padding: 6px;
    margin-bottom: 8px;
    background: #f4f4f5;
    font-size: 0.9rem;
    display: flex;
    gap: 8px;
    align-items: center;
}
.reply-preview .label {
    font-weight: 800;
}

.input-area {
    padding: 15px;
    border-top: var(--border-width) solid var(--border-color);
    display: flex;
    gap: 10px;
    background: #fafafa;
    flex-direction: column;
}

.chat-input {
    flex: 1;
}

.context-menu {
    position: fixed;
    z-index: 1000;
    background: white;
    min-width: 170px;
}
.menu-item {
    padding: 10px 15px;
    cursor: pointer;
    font-weight: bold;
}
.menu-item:hover {
    background: #f0f0f0;
}

.pill {
    display: inline-block;
    padding: 2px 8px;
    border: var(--border-width) solid var(--border-color);
    font-size: 0.75rem;
    background: #fff;
    font-weight: 800;
}

.replying {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 10px;
    background: #fff1f2;
}
.link-btn {
    background: none;
    border: none;
    font-weight: 800;
    cursor: pointer;
    color: var(--accent-color);
}

.toolbar {
    display: flex;
    gap: 8px;
    margin-bottom: 4px;
}
.icon-btn {
    background: none;
    border: none;
    cursor: pointer;
    font-size: 1.2rem;
    padding: 4px;
    border-radius: 4px;
}
.icon-btn:hover {
    background: #eee;
}

.at-tag {
    background: #fef3c7;
    padding: 2px 6px;
    border: 2px solid #000;
    font-weight: 800;
}

.voice {
    width: 220px;
}

.recall-text {
    font-style: italic;
    color: #666;
}

.loading {
    text-align: center;
    font-weight: 800;
}

.nb-button.ghost {
    background: #fff;
    color: #000;
}
</style>
