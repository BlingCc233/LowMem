import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { User, Group, Message, Session } from '../types'
// @ts-ignore
import { GetFriends, GetGroups, GetMessages, SendMessage, ForwardMessage, SearchMessages, MarkChatRead, GetSessions, GetOneBotMessage } from '../../wailsjs/go/backend/Backend'
// @ts-ignore
import { EventsOn } from '../../wailsjs/runtime/runtime'

const PINNED_KEY = 'lagrangeqq-pinned'

const loadPinnedChats = (): string[] => {
    if (typeof window === 'undefined') return []
    try {
        const raw = localStorage.getItem(PINNED_KEY)
        if (!raw) return []
        const parsed = JSON.parse(raw)
        if (Array.isArray(parsed)) {
            return parsed.filter((item) => typeof item === 'string')
        }
    } catch (e) {
        console.error('Failed to load pinned chats', e)
    }
    return []
}

const persistPinnedChats = (keys: string[]) => {
    if (typeof window === 'undefined') return
    try {
        localStorage.setItem(PINNED_KEY, JSON.stringify(keys))
    } catch (e) {
        console.error('Failed to persist pinned chats', e)
    }
}

const defaultName = (id: number, isGroup: boolean) => (isGroup ? `Group ${id}` : `User ${id}`)
const defaultAvatar = (id: number, isGroup: boolean) =>
    isGroup ? `https://p.qlogo.cn/gh/${id}/${id}/640` : `https://q1.qlogo.cn/g?b=qq&nk=${id}&s=640`

type ChatRef = { id: number; isGroup: boolean; name: string; avatar: string } | null
type SessionUpdate = Partial<Session> & { id: number; isGroup: boolean }

export const useChatStore = defineStore('chat', () => {
    const friends = ref<User[]>([])
    const groups = ref<Group[]>([])
    const currentChat = ref<ChatRef>(null)
    const messages = ref<Message[]>([])
    const loadingMessages = ref(false)
    const searchResults = ref<Message[]>([])
    const referencedMessages = ref<Record<number, Message>>({})
    const sessions = ref<Record<string, Session>>({})
    const sessionSearch = ref('')
    const pinnedChats = ref<string[]>(loadPinnedChats())

    const chatKey = (id: number, isGroup: boolean) => `${isGroup ? 'g' : 'f'}-${id}`
    const pinnedSet = computed(() => new Set(pinnedChats.value))

    const isPinned = (id: number, isGroup: boolean) => pinnedSet.value.has(chatKey(id, isGroup))

    const persistPinned = () => {
        persistPinnedChats(pinnedChats.value)
    }

    const togglePin = (id: number, isGroup: boolean) => {
        const key = chatKey(id, isGroup)
        const idx = pinnedChats.value.indexOf(key)
        if (idx !== -1) {
            pinnedChats.value.splice(idx, 1)
        } else {
            pinnedChats.value.unshift(key)
        }
        persistPinned()
    }

    const sessionEntries = computed(() => Object.values(sessions.value))

    const orderedSessions = computed(() => {
        const term = sessionSearch.value.trim().toLowerCase()
        const filtered = term
            ? sessionEntries.value.filter((session) => session.name.toLowerCase().includes(term))
            : sessionEntries.value.slice()
        const pinned = filtered.filter((session) => isPinned(session.id, session.isGroup))
        const others = filtered.filter((session) => !isPinned(session.id, session.isGroup))
        const sorter = (a: Session, b: Session) => {
            if (b.time !== a.time) return b.time - a.time
            return a.name.localeCompare(b.name)
        }
        pinned.sort(sorter)
        others.sort(sorter)
        return [...pinned, ...others]
    })

    const updateSession = (update: SessionUpdate) => {
        const key = chatKey(update.id, update.isGroup)
        const existing = sessions.value[key]
        const merged: Session = {
            id: update.id,
            isGroup: update.isGroup,
            name: update.name || existing?.name || defaultName(update.id, update.isGroup),
            avatar: update.avatar || existing?.avatar || defaultAvatar(update.id, update.isGroup),
            lastMessage: existing?.lastMessage || '',
            time: existing?.time || 0,
            unread: existing?.unread || 0,
        }

        if (update.lastMessage !== undefined) {
            merged.lastMessage = update.lastMessage
        }
        if (update.time !== undefined) {
            merged.time = existing ? Math.max(existing.time, update.time) : update.time
        }
        if (update.unread !== undefined) {
            merged.unread = update.unread
        }

        sessions.value = { ...sessions.value, [key]: merged }
        return merged
    }

    const touchSession = (
        id: number,
        isGroup: boolean,
        content: string,
        time: number,
        senderName: string,
        avatar: string,
        increaseUnread: boolean
    ) => {
        const key = chatKey(id, isGroup)
        const existing = sessions.value[key]
        const incremented = increaseUnread ? (existing?.unread ?? 0) + 1 : 0
        updateSession({
            id,
            isGroup,
            name: senderName,
            avatar,
            lastMessage: content,
            time,
            unread: incremented,
        })
    }

    const markSessionRead = (id: number, isGroup: boolean) => {
        updateSession({ id, isGroup, unread: 0 })
    }

    const loadContacts = async () => {
        try {
            friends.value = await GetFriends()
            groups.value = await GetGroups()
            friends.value.forEach((friend) => {
                updateSession({
                    id: friend.user_id,
                    isGroup: false,
                    name: friend.nickname,
                    avatar: friend.avatar_url,
                })
            })
            groups.value.forEach((group) => {
                updateSession({
                    id: group.group_id,
                    isGroup: true,
                    name: group.group_name,
                    avatar: group.avatar_url,
                })
            })
        } catch (e) {
            console.error('Failed to load contacts', e)
        }
    }

    const loadSessionSummaries = async () => {
        try {
            const list = await GetSessions()
                ; (list as any[]).forEach((item) => {
                    updateSession({
                        id: item.chat_id,
                        isGroup: item.is_group,
                        name: item.name,
                        avatar: item.avatar,
                        lastMessage: item.last_message,
                        time: item.time || 0,
                        unread: item.unread ?? 0,
                    })
                })
        } catch (e) {
            console.error('Failed to load chat summaries', e)
        }
    }

    const initSessions = async () => {
        await loadContacts()
        await loadSessionSummaries()
    }

    const selectChat = async (id: number, isGroup: boolean, name: string, avatar: string) => {
        currentChat.value = { id, isGroup, name, avatar }
        loadingMessages.value = true
        messages.value = []
        searchResults.value = []
        try {
            const res = await GetMessages(id, isGroup)
            messages.value = (res as Message[]).reverse()
            await MarkChatRead(id, isGroup)
            markSessionRead(id, isGroup)
        } catch (e) {
            console.error('Failed to load messages', e)
        } finally {
            loadingMessages.value = false
        }
    }

    const belongsToCurrentChat = (msg: Message) => {
        if (!currentChat.value) return false
        if (currentChat.value.isGroup) {
            return msg.message_type === 'group' && msg.group_id === currentChat.value.id
        }
        if (msg.message_type !== 'private') return false
        const peer = msg.is_send ? msg.target_id : msg.sender_id
        return peer === currentChat.value.id
    }

    const sendMessage = async (content: string, options?: { replyTo?: number; atList?: number[] }) => {
        if (!currentChat.value || !content.trim()) return
        const parts: string[] = []
        if (options?.replyTo) {
            parts.push(`[CQ:reply,id=${options.replyTo}]`)
        }
        if (options?.atList?.length) {
            options.atList.forEach((id) => parts.push(`[CQ:at,qq=${id}]`))
        }
        parts.push(content)

        try {
            const msg = await SendMessage(currentChat.value.id, currentChat.value.isGroup, parts.join(''))
            if (msg) {
                messages.value.push(msg as Message)
                touchSession(
                    currentChat.value.id,
                    currentChat.value.isGroup,
                    content,
                    msg.time,
                    currentChat.value.name,
                    currentChat.value.avatar,
                    false
                )
            }
        } catch (e) {
            console.error('Failed to send', e)
        }
    }

    const previewForMessage = (msg: Message) => {
        let preview = msg.content
        if (msg.elements?.some((el) => el.type?.toLowerCase() === 'image')) {
            preview = '[图片]'
        } else if (msg.elements?.some((el) => {
            const low = el.type?.toLowerCase()
            return low === 'voice' || low === 'record'
        })) {
            preview = '[语音]'
        } else if (msg.elements?.some((el) => el.type?.toLowerCase() === 'face')) {
            preview = '[表情]'
        }
        return preview
    }

    const initListeners = () => {
        EventsOn('MessageReceived', (msg: Message) => {
            const isGroup = msg.message_type === 'group'
            const id = isGroup ? msg.group_id : (msg.is_send ? msg.target_id : msg.sender_id)
            const name = isGroup ? `Group ${id}` : (msg.sender?.nickname || `User ${id}`)
            const avatar = isGroup
                ? `https://p.qlogo.cn/gh/${id}/${id}/640`
                : msg.sender?.avatar_url || `https://q1.qlogo.cn/g?b=qq&nk=${id}&s=640`
            const preview = previewForMessage(msg)
            const increase = !msg.is_send && !belongsToCurrentChat(msg)

            if (belongsToCurrentChat(msg)) {
                messages.value.push(msg)
                if (currentChat.value) {
                    MarkChatRead(currentChat.value.id, currentChat.value.isGroup)
                    markSessionRead(currentChat.value.id, currentChat.value.isGroup)
                }
            }

            touchSession(id, isGroup, preview, msg.time, name, avatar, increase)
        })

        EventsOn('MessageRecalled', (payload: any) => {
            const targetId = typeof payload === 'number' ? payload : payload?.message_id
            const idx = messages.value.findIndex((m) => m.message_id === targetId)
            if (idx !== -1) {
                messages.value[idx].recall = true
                messages.value[idx].content = '[已撤回]'
            }
        })
    }

    const search = async (keyword: string) => {
        if (!currentChat.value || !keyword.trim()) {
            searchResults.value = []
            return
        }
        try {
            const res = await SearchMessages(keyword, currentChat.value.id, currentChat.value.isGroup)
            searchResults.value = res as Message[]
        } catch (e) {
            console.error('search failed', e)
        }
    }

    const forward = async (messageDBID: number, targetID: number, isGroup: boolean) => {
        try {
            await ForwardMessage(messageDBID, targetID, isGroup)
        } catch (e) {
            console.error('forward failed', e)
        }
    }

    const clearSessionSearch = () => {
        sessionSearch.value = ''
    }

    const fetchMessage = async (messageID: number) => {
        if (referencedMessages.value[messageID]) return referencedMessages.value[messageID]
        // Check if in main messages
        const found = messages.value.find(m => m.message_id === messageID)
        if (found) return found

        try {
            const res = await GetOneBotMessage(messageID)
            if (res) {
                referencedMessages.value[messageID] = res as unknown as Message
                return res as unknown as Message
            }
        } catch (e) {
            console.error('Failed to fetch message', messageID, e)
        }
        return null
    }

    return {
        friends,
        groups,
        currentChat,
        messages,
        loadingMessages,
        searchResults,
        referencedMessages,
        sessionSearch,
        orderedSessions,
        initSessions,
        initListeners,
        selectChat,
        sendMessage,
        search,
        forward,
        togglePin,
        isPinned,
        clearSessionSearch,
        fetchMessage,
    }
})
