'use client'

import { useState, useEffect, Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import Link from 'next/link'
import { Eye, EyeOff, Loader2, ArrowLeft } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

export type PortalType = 'ssp' | 'dsp'

interface PortalConfig {
  type: PortalType
  title: string
  subtitle: string
  description: string
  icon: string
  accentColor: string      // tailwind bg class
  accentText: string       // tailwind text class
  accentGradient: string   // gradient CSS for the icon circle
  badgeLabel: string
  badgeVariant: string     // tailwind classes for badge
}

export const PORTAL_CONFIGS: Record<PortalType, PortalConfig> = {
  ssp: {
    type: 'ssp',
    title: 'Publisher Portal',
    subtitle: 'Supply Side Platform',
    description: 'Monetize your network with IPLoop\'s proxy infrastructure. Earn revenue by sharing idle bandwidth.',
    icon: '📡',
    accentColor: 'bg-emerald-600',
    accentText: 'text-emerald-400',
    accentGradient: 'linear-gradient(135deg, #059669, #0d9488)',
    badgeLabel: 'SSP',
    badgeVariant: 'bg-emerald-500/20 text-emerald-400 border-emerald-500/30',
  },
  dsp: {
    type: 'dsp',
    title: 'Advertiser Portal',
    subtitle: 'Demand Side Platform',
    description: 'Access premium residential proxies worldwide. Enterprise-grade targeting and session management.',
    icon: '🎯',
    accentColor: 'bg-violet-600',
    accentText: 'text-violet-400',
    accentGradient: 'linear-gradient(135deg, #7c3aed, #6366f1)',
    badgeLabel: 'DSP',
    badgeVariant: 'bg-violet-500/20 text-violet-400 border-violet-500/30',
  },
}

function PortalLoginFormInner({ portalType }: { portalType: PortalType }) {
  const config = PORTAL_CONFIGS[portalType]
  const router = useRouter()
  const searchParams = useSearchParams()
  const [isLogin, setIsLogin] = useState(!searchParams.get('signup'))
  const [showPassword, setShowPassword] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const [formData, setFormData] = useState({
    email: '',
    password: '',
    confirmPassword: '',
    firstName: '',
    lastName: '',
    company: '',
  })

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const endpoint = isLogin ? '/api/auth/login' : '/api/auth/register'
      const base = isLogin
        ? { email: formData.email, password: formData.password, portalType }
        : {
            email: formData.email,
            password: formData.password,
            firstName: formData.firstName,
            lastName: formData.lastName,
            company: formData.company,
            portalType,
          }

      if (!isLogin && formData.password !== formData.confirmPassword) {
        setError('Passwords do not match')
        setLoading(false)
        return
      }

      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(base),
      })

      const data = await res.json()

      if (!res.ok) {
        const errMsg =
          typeof data.error === 'string'
            ? data.error
            : typeof data.error === 'object' && data.error?.message
            ? data.error.message
            : data.message || 'Authentication failed'
        throw new Error(errMsg)
      }

      // Store token + portal context
      localStorage.setItem('token', data.token)
      localStorage.setItem('user', JSON.stringify(data.user))
      localStorage.setItem('portalType', portalType)

      router.push('/dashboard')
    } catch (err: any) {
      const errorMsg =
        typeof err.message === 'string'
          ? err.message
          : typeof err === 'string'
          ? err
          : 'An error occurred'
      setError(errorMsg)
    } finally {
      setLoading(false)
    }
  }

  // Gradient background per portal
  const bgGradient =
    portalType === 'ssp'
      ? 'from-gray-950 via-emerald-950/30 to-gray-950'
      : 'from-gray-950 via-violet-950/30 to-gray-950'

  const buttonClass =
    portalType === 'ssp'
      ? 'bg-emerald-600 hover:bg-emerald-700 text-white'
      : 'bg-violet-600 hover:bg-violet-700 text-white'

  const linkClass =
    portalType === 'ssp' ? 'text-emerald-400 hover:text-emerald-300' : 'text-violet-400 hover:text-violet-300'

  return (
    <div className={`min-h-screen flex items-center justify-center bg-gradient-to-br ${bgGradient} p-4`}>
      {/* Back to home */}
      <div className="absolute top-4 left-4">
        <a
          href="https://iploop.io"
          className="flex items-center gap-2 text-gray-400 hover:text-white transition-colors text-sm"
        >
          <ArrowLeft className="h-4 w-4" /> Back to Home
        </a>
      </div>

      {/* Other portal link */}
      <div className="absolute top-4 right-4">
        <a
          href={portalType === 'ssp' ? '/dsp/login' : '/ssp/login'}
          className={`text-sm ${linkClass} transition-colors`}
        >
          {portalType === 'ssp' ? 'Advertiser Portal →' : '← Publisher Portal'}
        </a>
      </div>

      <Card className="w-full max-w-md border-gray-800 bg-gray-900/80 backdrop-blur-sm shadow-2xl">
        <CardHeader className="text-center pb-4">
          {/* Icon */}
          <div className="flex justify-center mb-3">
            <div
              className="w-16 h-16 rounded-2xl flex items-center justify-center text-3xl shadow-lg"
              style={{ background: config.accentGradient }}
            >
              {config.icon}
            </div>
          </div>

          {/* Badge */}
          <div className="flex justify-center mb-2">
            <span
              className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold border ${config.badgeVariant}`}
            >
              {config.badgeLabel}
            </span>
          </div>

          <CardTitle className="text-2xl text-white">{config.title}</CardTitle>
          <CardDescription className="text-gray-400">
            {isLogin ? config.description : `Create your ${config.subtitle} account`}
          </CardDescription>
        </CardHeader>

        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {!isLogin && (
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <label className="text-sm font-medium text-gray-300">First Name</label>
                  <Input
                    placeholder="John"
                    value={formData.firstName}
                    onChange={(e) => setFormData({ ...formData, firstName: e.target.value })}
                    required={!isLogin}
                    className="bg-gray-800 border-gray-700 text-white placeholder:text-gray-500"
                  />
                </div>
                <div className="space-y-1.5">
                  <label className="text-sm font-medium text-gray-300">Last Name</label>
                  <Input
                    placeholder="Doe"
                    value={formData.lastName}
                    onChange={(e) => setFormData({ ...formData, lastName: e.target.value })}
                    required={!isLogin}
                    className="bg-gray-800 border-gray-700 text-white placeholder:text-gray-500"
                  />
                </div>
              </div>
            )}

            <div className="space-y-1.5">
              <label className="text-sm font-medium text-gray-300">Email</label>
              <Input
                type="email"
                placeholder="you@example.com"
                value={formData.email}
                onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                required
                className="bg-gray-800 border-gray-700 text-white placeholder:text-gray-500"
              />
            </div>

            <div className="space-y-1.5">
              <label className="text-sm font-medium text-gray-300">Password</label>
              <div className="relative">
                <Input
                  type={showPassword ? 'text' : 'password'}
                  placeholder="••••••••"
                  value={formData.password}
                  onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                  required
                  className="bg-gray-800 border-gray-700 text-white placeholder:text-gray-500"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="absolute right-0 top-0 h-full text-gray-400 hover:text-white"
                  onClick={() => setShowPassword(!showPassword)}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </Button>
              </div>
              {isLogin && (
                <div className="flex justify-end">
                  <Link href="/forgot-password" className={`text-sm ${linkClass}`}>
                    Forgot password?
                  </Link>
                </div>
              )}
            </div>

            {!isLogin && (
              <>
                <div className="space-y-1.5">
                  <label className="text-sm font-medium text-gray-300">Confirm Password</label>
                  <Input
                    type="password"
                    placeholder="••••••••"
                    value={formData.confirmPassword}
                    onChange={(e) => setFormData({ ...formData, confirmPassword: e.target.value })}
                    required={!isLogin}
                    className="bg-gray-800 border-gray-700 text-white placeholder:text-gray-500"
                  />
                </div>
                <div className="space-y-1.5">
                  <label className="text-sm font-medium text-gray-300">Company (Optional)</label>
                  <Input
                    placeholder="Acme Inc."
                    value={formData.company}
                    onChange={(e) => setFormData({ ...formData, company: e.target.value })}
                    className="bg-gray-800 border-gray-700 text-white placeholder:text-gray-500"
                  />
                </div>
              </>
            )}

            {error && (
              <div className="p-3 text-sm bg-red-500/10 text-red-400 rounded-md border border-red-500/20">
                {error}
              </div>
            )}

            <Button type="submit" className={`w-full ${buttonClass}`} disabled={loading}>
              {loading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Please wait
                </>
              ) : isLogin ? (
                'Sign In'
              ) : (
                'Create Account'
              )}
            </Button>
          </form>

          <div className="mt-5 text-center">
            <button
              onClick={() => {
                setIsLogin(!isLogin)
                setError('')
              }}
              className="text-sm text-gray-400 hover:text-white transition-colors"
            >
              {isLogin ? "Don't have an account? Sign up" : 'Already have an account? Sign in'}
            </button>
          </div>

          {/* Separator */}
          <div className="mt-4 pt-4 border-t border-gray-800 text-center">
            <p className="text-xs text-gray-500">
              IPLoop &mdash; {config.subtitle}
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

export default function PortalLoginForm({ portalType }: { portalType: PortalType }) {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen flex items-center justify-center bg-gray-950">
          <Loader2 className="h-8 w-8 animate-spin text-gray-400" />
        </div>
      }
    >
      <PortalLoginFormInner portalType={portalType} />
    </Suspense>
  )
}
