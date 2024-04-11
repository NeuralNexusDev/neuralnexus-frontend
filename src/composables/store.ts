import { defineStore } from 'pinia'

interface Session {
    session_id: string
    user_id: string
    permissions: string[]
    iat: number
    lua: number
    exp: number
}

const defaultSession: Session = {
    session_id: '',
    user_id: '',
    permissions: [],
    iat: 0,
    lua: 0,
    exp: 0
}

export const sessionStore = defineStore({
    id: 'session',
    state: () => ({
        session: { ...defaultSession }
    }),
    actions: {
        setSession(sessionData: Session) {
            this.session = sessionData
        },
        getSession() {
            return this.session
        },
        deleteSession() {
            this.session = { ...defaultSession }
        }
    }
})
