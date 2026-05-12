export type AuthProvider = 'github' | 'google'

export interface AuthEvent {
  provider: AuthProvider
  action: 'login' | 'register'
  timestamp: number
}

export interface AuthState {
  isAuthenticated: boolean
  provider: AuthProvider | null
  events: AuthEvent[]
  loading: boolean
}
