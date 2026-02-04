'use client'

import { ReactNode, useEffect, useState } from 'react'
import { Sidebar } from './sidebar'

interface LayoutProps {
  children: ReactNode
}

interface User {
  firstName?: string
  lastName?: string
  email?: string
  role?: string
}

export function Layout({ children }: LayoutProps) {
  const [user, setUser] = useState<User | undefined>(undefined)

  useEffect(() => {
    // Get user from localStorage
    const storedUser = localStorage.getItem('user')
    if (storedUser) {
      try {
        setUser(JSON.parse(storedUser))
      } catch (e) {
        console.error('Failed to parse user from localStorage')
      }
    }
  }, [])

  return (
    <div className="min-h-screen bg-background">
      <Sidebar user={user} />
      
      <div className="md:pl-64 flex flex-col flex-1">
        <main className="flex-1 py-6 px-4 sm:px-6 lg:px-8 mt-16 md:mt-0">
          <div className="max-w-7xl mx-auto">
            {children}
          </div>
        </main>
      </div>
    </div>
  )
}
