import { createContext, useContext, useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { api, type DashboardUser } from './api'

interface AuthContextType {
  user: DashboardUser | null
  login: (username: string, password: string) => Promise<void>
  logout: () => void
  isAuthenticated: boolean
  isLoading: boolean
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<DashboardUser | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  useEffect(() => {
    // Check if user has a token on mount
    const token = localStorage.getItem('mqtt_token')
    const userStr = localStorage.getItem('mqtt_user')

    if (token && userStr) {
      try {
        setUser(JSON.parse(userStr))
      } catch (e) {
        localStorage.removeItem('mqtt_token')
        localStorage.removeItem('mqtt_user')
      }
    }
    setIsLoading(false)

    // Listen for unauthorized events from API calls
    const handleUnauthorized = () => {
      // Clear state and redirect to login
      setUser(null)
      localStorage.removeItem('mqtt_token')
      localStorage.removeItem('mqtt_user')
      queryClient.clear()
      navigate('/login', { replace: true })
    }

    window.addEventListener('unauthorized', handleUnauthorized)
    return () => window.removeEventListener('unauthorized', handleUnauthorized)
  }, [navigate, queryClient])

  const login = async (username: string, password: string) => {
    const { user } = await api.login(username, password)
    setUser(user)
    localStorage.setItem('mqtt_user', JSON.stringify(user))
    navigate('/dashboard')
  }

  const logout = () => {
    api.removeToken()
    localStorage.removeItem('mqtt_user')
    setUser(null)
    // Clear all cached queries to prevent stale data
    queryClient.clear()
    navigate('/login')
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        login,
        logout,
        isAuthenticated: !!user,
        isLoading,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
