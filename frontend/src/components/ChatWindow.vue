<script setup lang="ts">
import { ref, nextTick, watch, onMounted, onUnmounted, computed } from 'vue'
import { useChatStore } from '../store/chat'
import type { Message, MessageElement } from '../types'
// @ts-ignore
import { RecallMessage, OpenImage, SendImageMessage } from '../../wailsjs/go/backend/Backend'
// @ts-ignore
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'


const props = defineProps<{ isMobile?: boolean }>()
const emit = defineEmits<{ (e: 'back'): void }>()

const store = useChatStore()
const inputContent = ref('')
const msgListRef = ref<HTMLElement | null>(null)
const contextMenu = ref<{ visible: boolean; x: number; y: number; msg: Message | null }>({ visible: false, x: 0, y: 0, msg: null })
const replyTo = ref<Message | null>(null)
const atList = ref<number[]>([]) // Stores UIDs of people we officially @-ed via context menu or click
const searchQuery = ref('')
const isComposing = ref(false)
const pendingImages = ref<string[]>([]) // List of base64 strings to send
const mentionPopup = ref({ visible: false, x: 0, y: 0, filter: '' })
const activeMentionIndex = ref(0)

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

// Derive active members from recent messages for mention list
const activeMembers = computed(() => {
    const members = new Map<number, { id: number, name: string, avatar: string }>()
    // Add self? maybe not necessary
    store.messages.forEach(m => {
        if (m.sender && m.sender_id !== 0 && !members.has(m.sender_id)) {
            members.set(m.sender_id, {
                id: m.sender_id,
                name: m.sender.nickname || String(m.sender_id),
                avatar: m.sender.avatar_url
            })
        }
    })
    // Also include friend list if private chat? No, mentions usually for group.
    return Array.from(members.values())
})

const filteredMembers = computed(() => {
    const term = mentionPopup.value.filter.toLowerCase()
    return activeMembers.value.filter(m => 
        m.name.toLowerCase().includes(term) || String(m.id).includes(term)
    )
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
    pendingImages.value = []
    scrollToBottom()
})

// Watch input for @ mention trigger
watch(inputContent, (newVal) => {
    const lastChar = newVal.slice(-1)
    if (lastChar === '@') {
        mentionPopup.value.visible = true
        mentionPopup.value.filter = ''
        activeMentionIndex.value = 0
        // Position? ideally relative to caret, but simplified: fixed above input
    } else if (mentionPopup.value.visible) {
        // Update filter based on text after last @
        const match = newVal.match(/@([^\s]*)$/)
        if (match) {
            mentionPopup.value.filter = match[1]
        } else {
            mentionPopup.value.visible = false
        }
    }
})

const confirmMention = (member: { id: number, name: string }) => {
    const match = inputContent.value.match(/@([^\s]*)$/)
    if (match) {
        const prefix = inputContent.value.slice(0, match.index)
        inputContent.value = prefix + `@${member.name} `
        if (!atList.value.includes(member.id)) {
            atList.value.push(member.id)
        }
    }
    mentionPopup.value.visible = false
    document.querySelector('.chat-input')?.classList.remove('suggest-open'); // Optional UI hint
    // Focus back
    (document.querySelector('.chat-input') as HTMLInputElement)?.focus()
}

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
    return escaped.replace(urlRegex, '<a href="$1">$1</a>')
}

const handleContentClick = (e: MouseEvent) => {
    const target = e.target as HTMLElement
    if (target.tagName === 'A') {
        const href = target.getAttribute('href')
        if (href) {
             e.preventDefault()
             BrowserOpenURL(href)
        }
    }
}


const formatTime = (ts: number) => {
    if (!ts) return ''
    const d = new Date(ts * 1000)
    return `${d.toLocaleDateString()} ${d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
}

const handleEnter = (e: KeyboardEvent) => {
    if (isComposing.value) return
    if (mentionPopup.value.visible) {
        e.preventDefault()
        if (filteredMembers.value.length > 0) {
            confirmMention(filteredMembers.value[activeMentionIndex.value])
        }
        return
    }
    if (e.shiftKey) return // Allow multiline
    e.preventDefault()
    send()
}

const navigateMention = (step: number) => {
    if (!mentionPopup.value.visible) return
    const len = filteredMembers.value.length
    if (len === 0) return
    activeMentionIndex.value = (activeMentionIndex.value + step + len) % len
}

const send = async () => {
    // Send pending images first
    for (const b64 of pendingImages.value) {
        try {
            // Helper to upload
             // @ts-ignore
             const path = await window.go.backend.Backend.UploadImage(b64)
             if (path) {
                 await SendImageMessage(store.currentChat?.id || 0, store.currentChat?.isGroup || false, path)
             }
        } catch (e) {
            console.error(e)
        }
    }
    pendingImages.value = []

    if (!inputContent.value.trim()) {
        scrollToBottom()
        return
    }

    let content = inputContent.value
    // Cleanup pseudo-at text if needed, similar to before
    if (atList.value.length) {
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
            const reader = new FileReader()
            reader.onload = async (evt) => {
                const base64 = (evt.target?.result as string).split(',')[1]
                pendingImages.value.push(base64)
            }
            reader.readAsDataURL(file)
            e.preventDefault() 
            return
        }
    }
}

const handleDrop = async (e: DragEvent) => {
    const files = e.dataTransfer?.files
    if (!files || files.length === 0) return
    
    for (const file of Array.from(files)) {
        if (file.type.startsWith('image/')) {
            const reader = new FileReader()
            reader.onload = async (evt) => {
                const base64 = (evt.target?.result as string).split(',')[1]
                pendingImages.value.push(base64)
            }
             reader.readAsDataURL(file)
        }
    }
}

const removePendingImage = (index: number) => {
    pendingImages.value.splice(index, 1)
}

const showContextMenu = (e: MouseEvent, msg: Message) => {
    e.preventDefault()
    contextMenu.value = {
        visible: true,
        x: e.clientX,
        y: e.clientY,
        msg,
    }
    // Adjust if off screen
    nextTick(() => {
        if (contextMenu.value.y + 150 > window.innerHeight) {
            contextMenu.value.y = window.innerHeight - 160
        }
    })
}

const closeContextMenu = () => {
    contextMenu.value.visible = false
}

const handleDocumentClick = (event: MouseEvent) => {
    if (event.button === 2) return
    closeContextMenu()
    mentionPopup.value.visible = false
}

const recallMsg = async () => {
    if (contextMenu.value.msg?.is_send) {
        await RecallMessage(contextMenu.value.msg.message_id)
    }
    closeContextMenu()
}

const setReply = (msg: Message) => {
    replyTo.value = msg
    closeContextMenu();
    // Focus input
    (document.querySelector('.chat-input') as HTMLInputElement)?.focus()
}

const mention = (msg: Message) => {
    if (msg.is_send) return
    if (!atList.value.includes(msg.sender_id)) {
        atList.value.push(msg.sender_id)
    }
    const name = msg.sender?.nickname || String(msg.sender_id)
    inputContent.value = `@${name} ` + inputContent.value
    closeContextMenu()
    ;(document.querySelector('.chat-input') as HTMLInputElement)?.focus()
}

const forwardMsg = (msg: Message) => {
    // Basic implementation for now, ideally UI dialog
    if (!msg.database_id) return
    const target = prompt('输入转发目标ID (好友ID 或 g群号):')
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
    const jump = () => {
        const el = document.getElementById(`msg-${messageId}`)
        if (el) {
            el.scrollIntoView({ behavior: 'smooth', block: 'center' })
            el.classList.add('highlight')
            setTimeout(() => el.classList.remove('highlight'), 2000)
        }
    }

    const existing = store.messages.find((m) => m.message_id === messageId)
    if (existing) {
        jump()
    } else {
        // Try fetch or check search results? 
        // If not loaded, we might need to load history logic (not implemented yet)
        console.warn("Message not in view")
    }
}

const handleScroll = async () => {
    if (!msgListRef.value) return
    if (msgListRef.value.scrollTop === 0 && store.hasMoreMessages && !store.loadingMessages) {
        const oldHeight = msgListRef.value.scrollHeight
        await store.loadHistory()
        nextTick(() => {
            if (msgListRef.value) {
                const newHeight = msgListRef.value.scrollHeight
                msgListRef.value.scrollTop = newHeight - oldHeight
            }
        })
    }
}

onMounted(() => {
    document.addEventListener('click', handleDocumentClick)
})

onUnmounted(() => {
    document.removeEventListener('click', handleDocumentClick)
})

const handleImgError = (e: Event) => {
    const img = e.target as HTMLImageElement
    img.style.display = 'none'
    // append text node?
    img.parentElement?.insertAdjacentText('beforeend', '[图片加载失败]')
}
</script>

<template>
  <div class="chat-window">
      <div class="header">
          <button v-if="props.isMobile" @click="emit('back')" class="icon-btn back-btn" aria-label="Back">
             ←
          </button>
          <div class="title-block">
              <h2>{{ store.currentChat?.name }}</h2>
              <span v-if="store.currentChat?.isGroup" class="pill">群聊</span>
          </div>
          <div class="tools">
              <input 
                v-model="searchQuery" 
                @keyup.enter="performSearch"
                class="nb-input search-input" 
                placeholder="搜索消息"
              />
          </div>
      </div>

      <div v-if="store.searchResults.length" class="search-results nb-box">
          <div 
            v-for="item in store.searchResults" 
            :key="item.message_id" 
            class="search-item"
            @click="scrollToMessage(item.message_id)"
          >
              <div class="search-meta">{{ formatTime(item.time) }} - {{ item.sender?.nickname }}</div>
              <div class="search-text">{{ item.content }}</div>
          </div>
      </div>
      
      <div class="messages" ref="msgListRef" @scroll="handleScroll">
          <div v-if="store.loadingMessages && store.messages.length === 0" class="loading">Loading...</div>
          <div v-if="store.loadingMessages && store.messages.length > 0" class="loading-more">Loading history...</div>
          
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
                      <span class="sender">{{ msg.is_send ? '我' : (msg.sender?.nickname || 'Ta') }}</span>
                      <span class="time">{{ formatTime(msg.time) }}</span>
                  </div>
                  <div v-if="msg.recall" class="recall-text">消息已撤回</div>
                  <template v-else>
                      <div v-if="msg.reply_to" class="reply-preview nb-box" @click="scrollToMessage(msg.reply_to)">
                          <span class="label">回复</span>
                          <div class="reply-body">{{ messageMap[msg.reply_to]?.content || '...' }}</div>
                      </div>
                      <div class="content" @click="handleContentClick">
                          <template v-for="(el, idx) in elementList(msg)" :key="idx">
                              <span v-if="el.type === 'text'" class="text" v-html="formatText(el.text || '')"></span>
                              <span v-else-if="el.type === 'at'" class="at-tag">@{{ store.getMemberNameSync(store.currentChat?.isGroup ? store.currentChat.id : 0, Number(el.qq)) }}</span>
                              <img 
                                v-else-if="el.type === 'image'" 
                                :src="el.url || el.file" 
                                class="msg-image nb-box" 
                                @load="scrollToBottom" 
                                @error="handleImgError"
                              />
                              <div v-else-if="el.type === 'voice'" class="voice-msg">
                                  <audio controls :src="el.url || el.file"></audio>
                              </div>
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
           <!-- Mention Popup -->
           <div v-if="mentionPopup.visible" class="mention-popup nb-box">
                <div 
                  v-for="(m, idx) in filteredMembers" 
                  :key="m.id" 
                  class="mention-item"
                  :class="{ active: idx === activeMentionIndex }"
                  @click="confirmMention(m)"
                >
                    <img :src="m.avatar" class="tiny-avatar"/>
                    <span>{{ m.name }}</span>
                </div>
                <div v-if="filteredMembers.length === 0" class="mention-empty">No match</div>
           </div>

           <!-- Pending Images -->
           <div v-if="pendingImages.length" class="pending-images">
               <div v-for="(img, idx) in pendingImages" :key="idx" class="pending-img-wrap">
                   <img :src="`data:image/jpeg;base64,${img}`" class="pending-img" />
                   <button class="remove-btn" @click="removePendingImage(idx)">×</button>
               </div>
           </div>

           <div v-if="replyTo" class="replying nb-box">
               <div class="replying-text">回复 {{ replyTo.sender?.nickname || '...' }}</div>
               <button class="link-btn" @click="replyTo = null">取消</button>
           </div>
           
           <div class="toolbar">
              <!-- <button class="icon-btn">😊</button> -->
           </div>

           <div class="input-row">
                <textarea 
                    v-model="inputContent" 
                    @keydown.enter="handleEnter"
                    @keydown.up.prevent="navigateMention(-1)"
                    @keydown.down.prevent="navigateMention(1)"
                    @compositionstart="isComposing = true"
                    @compositionend="isComposing = false"
                    class="nb-input chat-input" 
                    placeholder="输入消息，Enter发送，Shift+Enter换行"
                    rows="1"
                ></textarea>
                <button @click="send" class="nb-button">Send</button>
           </div>
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
}

.header {
    padding: 10px 15px;
    border-bottom: var(--border-width) solid var(--border-color);
    background: #fefefe;
    display: flex;
    align-items: center;
    gap: 10px;
    height: 50px;
    box-sizing: border-box;
}
.title-block {
    display: flex;
    align-items: center;
    gap: 10px;
    flex: 1;
    overflow: hidden;
}
.header h2 {
    margin: 0;
    font-size: 1.1rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.tools {
    display: flex;
    gap: 8px;
}
.search-input {
    width: 150px;
    font-size: 0.8rem;
    padding: 4px 8px;
}
.back-btn {
    font-size: 1.2rem;
    background: none;
    border: none;
    cursor: pointer;
    padding: 0 5px;
}

.search-results {
    max-height: 150px;
    overflow-y: auto;
    margin: 8px 16px 0;
    padding: 8px;
    background: #fff;
    border-bottom: 2px solid #eee;
}
.search-item {
    padding: 6px 8px;
    cursor: pointer;
    border-bottom: 1px dashed var(--border-color);
}
.search-item:hover {
    background: #f0f9ff;
}

.messages {
    flex: 1;
    overflow-y: auto;
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 15px;
}

.msg-row {
    display: flex;
    align-items: flex-end;
    gap: 10px;
    max-width: 80%; /* Increased width */
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
    padding: 8px 12px;
    background: #fff;
    border-radius: 12px;
    border: var(--border-width) solid var(--border-color);
    box-shadow: 2px 2px 0 var(--border-color);
    min-width: 60px;
    position: relative;
}
.msg-row.mine .bubble {
    background: var(--secondary-color);
    color: white;
    border-bottom-right-radius: 2px;
}
.msg-row:not(.mine) .bubble {
     border-bottom-left-radius: 2px;
}

.bubble.recalled {
    background: #f5f5f5;
    color: #777;
}

.msg-row.highlight .bubble {
    animation: flash 1s;
}
@keyframes flash {
    0% { background: yellow; }
    100% { background: #fff; }
}

.meta {
    display: flex;
    justify-content: space-between;
    font-size: 0.7rem;
    margin-bottom: 4px;
    color: #888;
    gap: 10px;
}
.msg-row.mine .meta {
    color: #e5e5e5;
}

.content {
    display: flex;
    flex-direction: column;
    gap: 4px;
    word-break: break-all;
}

.text a {
    color: var(--accent-color);
    text-decoration: underline;
    cursor: pointer;
}
.msg-row.mine .text a {
    color: #fff;
}

.msg-image {
    max-width: 100%;
    max-height: 300px;
    border-radius: 8px;
    cursor: pointer;
}

.reply-preview {
    padding: 4px 8px;
    margin-bottom: 6px;
    background: rgba(0,0,0,0.05);
    border-left: 3px solid var(--accent-color);
    font-size: 0.85rem;
    cursor: pointer;
}
.reply-preview .label {
    font-weight: bold;
    display: block;
}
.reply-preview .reply-body {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.input-area {
    padding: 10px;
    border-top: var(--border-width) solid var(--border-color);
    display: flex;
    gap: 8px;
    background: #fafafa;
    flex-direction: column;
    position: relative;
}

.replying {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 4px 8px;
    background: #eef2ff;
    border-left: 3px solid var(--accent-color);
}
.link-btn {
    background: none;
    border: none;
    cursor: pointer;
    color: var(--accent-color);
}

.input-row {
    display: flex;
    gap: 10px;
}

.chat-input {
    flex: 1;
    resize: none;
    min-height: 40px;
    max-height: 100px;
    padding: 8px;
    font-family: inherit;
}

.pending-images {
    display: flex;
    gap: 10px;
    overflow-x: auto;
    padding-bottom: 5px;
}
.pending-img-wrap {
    position: relative;
    width: 60px;
    height: 60px;
    flex-shrink: 0;
}
.pending-img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border: 1px solid #ccc;
    border-radius: 4px;
}
.remove-btn {
    position: absolute;
    top: -5px;
    right: -5px;
    background: red;
    color: white;
    border: none;
    border-radius: 50%;
    width: 16px;
    height: 16px;
    font-size: 10px;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
}

.mention-popup {
    position: absolute;
    bottom: 100%;
    left: 10px;
    background: white;
    width: 200px;
    max-height: 200px;
    overflow-y: auto;
    border: 2px solid black;
    z-index: 100;
}
.mention-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    cursor: pointer;
}
.mention-item.active {
    background: #e0f2fe;
}
.mention-item:hover {
    background: #f0f9ff;
}
.tiny-avatar {
    width: 20px;
    height: 20px;
    border-radius: 50%;
}
.mention-empty {
    padding: 10px;
    color: #888;
    text-align: center;
}

.context-menu {
    position: fixed;
    z-index: 1000;
    background: white;
    min-width: 120px;
    box-shadow: 2px 2px 5px rgba(0,0,0,0.2);
}
.menu-item {
    padding: 8px 12px;
    cursor: pointer;
    border-bottom: 1px solid #eee;
}
.menu-item:hover {
    background: #f5f5f5;
}

.pill {
    display: inline-block;
    padding: 1px 6px;
    border: 1px solid #000;
    font-size: 0.7rem;
    margin-left: 5px;
    background: #fff;
}

.at-tag {
    background: #fef3c7;
    padding: 0 4px;
    border-radius: 4px;
    font-weight: 500;
    margin: 0 2px;
}
</style>
