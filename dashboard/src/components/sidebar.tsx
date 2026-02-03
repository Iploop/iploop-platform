'use client'

import { useState } from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import {
  BarChart3,
  Key,
  CreditCard,
  Settings,
  Shield,
  Globe,
  Menu,
  X,
  LogOut,
  User,
  Smartphone
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from './ui/button'

const navigation = [
  { name: 'Dashboard', href: '/dashboard', icon: BarChart3 },
  { name: 'Nodes', href: '/nodes', icon: Smartphone },
  { name: 'API Keys', href: '/api-keys', icon: Key },
  { name: 'Usage Stats', href: '/usage', icon: BarChart3 },
  { name: 'Proxy Endpoints', href: '/endpoints', icon: Globe },
  { name: 'Billing', href: '/billing', icon: CreditCard },
  { name: 'Settings', href: '/settings', icon: Settings },
]

export function Sidebar() {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const pathname = usePathname()

  return (
    <>
      {/* Mobile sidebar overlay */}
      <div className={cn(
        "fixed inset-0 flex z-40 md:hidden",
        sidebarOpen ? "pointer-events-auto" : "pointer-events-none"
      )}>
        <div className={cn(
          "fixed inset-0 bg-gray-600 bg-opacity-75",
          sidebarOpen ? "opacity-100" : "opacity-0"
        )} onClick={() => setSidebarOpen(false)} />
        
        <div className={cn(
          "relative flex-1 flex flex-col max-w-xs w-full bg-card",
          sidebarOpen ? "translate-x-0" : "-translate-x-full"
        )}>
          <div className="absolute top-0 right-0 -mr-12 pt-2">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setSidebarOpen(false)}
              className="text-white"
            >
              <X className="h-6 w-6" />
            </Button>
          </div>
          
          <div className="flex-1 h-0 pt-5 pb-4 overflow-y-auto">
            <div className="flex-shrink-0 flex items-center px-4">
              <Shield className="h-8 w-8 text-primary" />
              <span className="ml-2 text-xl font-bold">IPLoop</span>
            </div>
            <nav className="mt-5 px-2 space-y-1">
              {navigation.map((item) => {
                const isActive = pathname === item.href
                return (
                  <Link
                    key={item.name}
                    href={item.href}
                    className={cn(
                      "group flex items-center px-2 py-2 text-base font-medium rounded-md",
                      isActive 
                        ? "bg-primary text-primary-foreground" 
                        : "text-foreground hover:bg-accent hover:text-accent-foreground"
                    )}
                  >
                    <item.icon className="mr-4 h-6 w-6" />
                    {item.name}
                  </Link>
                )
              })}
            </nav>
          </div>
          
          <div className="flex-shrink-0 flex border-t border-border p-4">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <User className="h-8 w-8 text-muted-foreground" />
              </div>
              <div className="ml-3">
                <p className="text-sm font-medium">Demo User</p>
                <p className="text-xs text-muted-foreground">demo@iploop.com</p>
              </div>
              <Button variant="ghost" size="icon" className="ml-auto">
                <LogOut className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      </div>

      {/* Static sidebar for desktop */}
      <div className="hidden md:flex md:w-64 md:flex-col md:fixed md:inset-y-0">
        <div className="flex-1 flex flex-col min-h-0 border-r border-border bg-card">
          <div className="flex-1 flex flex-col pt-5 pb-4 overflow-y-auto">
            <div className="flex items-center flex-shrink-0 px-4">
              <Shield className="h-8 w-8 text-primary" />
              <span className="ml-2 text-xl font-bold">IPLoop</span>
            </div>
            <nav className="mt-5 flex-1 px-2 space-y-1">
              {navigation.map((item) => {
                const isActive = pathname === item.href
                return (
                  <Link
                    key={item.name}
                    href={item.href}
                    className={cn(
                      "group flex items-center px-2 py-2 text-sm font-medium rounded-md",
                      isActive 
                        ? "bg-primary text-primary-foreground" 
                        : "text-foreground hover:bg-accent hover:text-accent-foreground"
                    )}
                  >
                    <item.icon className="mr-3 h-6 w-6" />
                    {item.name}
                  </Link>
                )
              })}
            </nav>
          </div>
          
          <div className="flex-shrink-0 flex border-t border-border p-4">
            <div className="flex items-center w-full">
              <div className="flex-shrink-0">
                <User className="h-8 w-8 text-muted-foreground" />
              </div>
              <div className="ml-3 flex-1">
                <p className="text-sm font-medium">Demo User</p>
                <p className="text-xs text-muted-foreground">demo@iploop.com</p>
              </div>
              <Button variant="ghost" size="icon">
                <LogOut className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      </div>

      {/* Mobile menu button */}
      <div className="md:hidden">
        <div className="fixed top-0 left-0 right-0 z-30 flex items-center justify-between bg-card border-b border-border px-4 py-2">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setSidebarOpen(true)}
          >
            <Menu className="h-6 w-6" />
          </Button>
          <div className="flex items-center">
            <Shield className="h-6 w-6 text-primary" />
            <span className="ml-2 text-lg font-bold">IPLoop</span>
          </div>
          <div className="w-10" />
        </div>
      </div>
    </>
  )
}