export interface User {
    user_id: number;
    nickname: string;
    avatar_url: string;
}

export interface Group {
    group_id: number;
    group_name: string;
    avatar_url: string;
    member_count: number;
}

export type MessageElementType = 'text' | 'image' | 'face' | 'voice' | 'at' | 'reply' | 'json' | string

export interface MessageElement {
    type: MessageElementType
    text?: string
    url?: string
    file?: string
    qq?: string
    name?: string
    reply_id?: number
    id?: number
}

export interface Message {
    database_id: number;
    message_id: number; // OneBot ID
    message_type: 'private' | 'group';
    sub_type: string;
    sender_id: number;
    sender: User;
    target_id: number; // For private
    group_id: number; // 0 for private
    content: string; // Display text
    raw_message: string;
    elements?: MessageElement[];
    reply_to?: number;
    time: number;
    is_send: boolean;
    is_read: boolean;
    recall?: boolean;
}

export interface ContactListResponse {
    friends: User[];
    groups: Group[];
}

export interface UnreadSummary {
    chat_id: number
    is_group: boolean
    count: number
}
export interface Session {
    id: number
    isGroup: boolean
    name: string
    avatar: string
    lastMessage: string
    time: number
    unread: number
}
