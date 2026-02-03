'use client'

import { ReactNode } from 'react'
import { Sidebar } from './sidebar'

interface LayoutProps {
  children: ReactNode
}

export function Layout({ children }: LayoutProps) {
  return (
    <div className="min-h-screen bg-background">
      <Sidebar />
      
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