import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { User, Group, Message, Session } from '../types'
// @ts-ignore
import { GetFriends, GetGroups, GetMessages, SendMessage, ForwardMessage, SearchMessages, MarkChatRead, UnreadSummary } from '../../wailsjs/go/backend/Backend'
// @ts-ignore
import { EventsOn } from '../../wailsjs/runtime/runtime'

export const useChatStore = defineStore('chat', () => {
    const friends = ref<User[]>([])
    const groups = ref<Group[]>([])
    const currentChat = ref<{ id: number; isGroup: boolean; name: string; avatar: string } | null>(null)
    const messages = ref<Message[]>([])
    const loadingMessages = ref(false)
    const searchResults = ref<Message[]>([])
    const unreadMap = ref<Record<string, number>>({})
    const sessions = ref<Session[]>([])

    const chatKey = (id: number, isGroup: boolean) => `${isGroup ? 'g' : 'f'}-${id}`

    const refreshUnread = async () => {
        try {
            const summary = await UnreadSummary()
            const map: Record<string, number> = {}
            summary.forEach((row: any) => {
                map[chatKey(Number(row.chat_id), !!row.is_group)] = Number(row.count)
            })
            unreadMap.value = map
        } catch (e) {
            console.error('Failed to load unread summary', e)
        }
    }

    // Load initial data
    const loadContacts = async () => {
        try {
            friends.value = await GetFriends()
            groups.value = await GetGroups()
            await refreshUnread()
        } catch (e) {
            console.error("Failed to load contacts", e)
        }
    }

    const selectChat = async (id: number, isGroup: boolean, name: string, avatar: string) => {
        currentChat.value = { id, isGroup, name, avatar }
        loadingMessages.value = true
        messages.value = [] // clear current
        searchResults.value = []
        try {
            const res = await GetMessages(id, isGroup)
            messages.value = (res as Message[]).reverse()
            await MarkChatRead(id, isGroup)
            unreadMap.value[chatKey(id, isGroup)] = 0

            // update session unread
            const s = sessions.value.find(s => s.id === id && s.isGroup === isGroup)
            if (s) s.unread = 0
        } catch (e) {
            console.error("Failed to load messages", e)
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
            options.atList.forEach(id => parts.push(`[CQ:at,qq=${id}]`))
        }
        parts.push(content)

        try {
            const msg = await SendMessage(currentChat.value.id, currentChat.value.isGroup, parts.join(''))
            if (msg) {
                // Backend sends back the message object, push it
                messages.value.push(msg as Message)

                // Update session
                touchSession(
                    currentChat.value.id,
                    currentChat.value.isGroup,
                    content, // preview
                    msg.time,
                    currentChat.value.name,
                    currentChat.value.avatar,
                    false // don't increase unread for own message
                )
            }
        } catch (e) {
            console.error("Failed to send", e)
        }
    }

    // Helper to find or create session
    const touchSession = (id: number, isGroup: boolean, content: string, time: number, senderName: string, avatar: string, increaseUnread: boolean) => {
        const idx = sessions.value.findIndex(s => s.id === id && s.isGroup === isGroup)
        if (idx !== -1) {
            const s = sessions.value[idx]
            s.lastMessage = content
            s.time = time
            if (increaseUnread && (!currentChat.value || currentChat.value.id !== id || currentChat.value.isGroup !== isGroup)) {
                s.unread++
            }
            // Move to top
            sessions.value.splice(idx, 1)
            sessions.value.unshift(s)
        } else {
            // New session
            sessions.value.unshift({
                id,
                isGroup,
                name: isGroup ? `Group ${id}` : senderName,
                avatar: avatar || (isGroup ? `https://p.qlogo.cn/gh/${id}/${id}/640` : `https://q1.qlogo.cn/g?b=qq&nk=${id}&s=640`),
                lastMessage: content,
                time: time,
                unread: increaseUnread ? 1 : 0
            })
        }
    }

    const initListeners = () => {
        EventsOn("MessageReceived", (msg: Message) => {
            if (belongsToCurrentChat(msg)) {
                messages.value.push(msg)
                if (currentChat.value) {
                    MarkChatRead(currentChat.value.id, currentChat.value.isGroup)
                    // Reset unread for this session in unreadMap (backup) and session list
                    unreadMap.value[chatKey(currentChat.value.id, currentChat.value.isGroup)] = 0
                }
            } else {
                const key = msg.message_type === 'group'
                    ? chatKey(msg.group_id, true)
                    : chatKey(msg.is_send ? msg.target_id : msg.sender_id, false)
                unreadMap.value[key] = (unreadMap.value[key] || 0) + 1
            }

            // Update session list
            const isGroup = msg.message_type === 'group'
            const id = isGroup ? msg.group_id : (msg.is_send ? msg.target_id : msg.sender_id)
            const name = isGroup ? (msg.group_name || `Group ${id}`) : (msg.sender?.nickname || `User ${id}`)
            const avatar = isGroup ? `https://p.qlogo.cn/gh/${id}/${id}/640` : `https://q1.qlogo.cn/g?b=qq&nk=${id}&s=640`

            // Format content preview
            let preview = msg.content
            if (msg.elements?.some(e => e.type === 'image')) preview = '[图片]'
            if (msg.elements?.some(e => e.type === 'voice')) preview = '[语音]'
            if (msg.elements?.some(e => e.type === 'Record')) preview = '[语音]' // Case sensitivity check

            touchSession(id, isGroup, preview, msg.time, name, avatar, !msg.is_send && !belongsToCurrentChat(msg))
        })

        EventsOn("MessageRecalled", (payload: any) => {
            const targetId = typeof payload === 'number' ? payload : payload?.message_id
            const idx = messages.value.findIndex(m => m.message_id === targetId)
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

    // Initialize sessions from friends/groups
    const initSessions = async () => {
        await loadContacts()
        const newSessions: Session[] = []

        groups.value.forEach(g => {
            newSessions.push({
                id: g.group_id,
                isGroup: true,
                name: g.group_name,
                avatar: g.avatar_url,
                lastMessage: '',
                time: 0,
                unread: unreadMap.value[`g-${g.group_id}`] || 0
            })
        })
        friends.value.forEach(f => {
            newSessions.push({
                id: f.user_id,
                isGroup: false,
                name: f.nickname,
                avatar: f.avatar_url,
                lastMessage: '',
                time: 0,
                unread: unreadMap.value[`f-${f.user_id}`] || 0
            })
        })
        sessions.value = newSessions
    }

    return {
        friends,
        groups,
        sessions,
        currentChat,
        messages,
        loadingMessages,
        searchResults,
        unreadMap,
        loadContacts,
        selectChat,
        sendMessage,
        initListeners,
        search,
        forward,
        refreshUnread,
        initSessions
    }
})
