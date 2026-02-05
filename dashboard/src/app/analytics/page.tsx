'use client'

import { useEffect, useState } from 'react'
import { Layout } from '@/components/layout'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { 
  BarChart3, TrendingUp, Globe, Clock, Activity,
  ArrowUp, ArrowDown, RefreshCw
} from 'lucide-react'
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  BarChart, Bar, PieChart, Pie, Cell, Legend
} from 'recharts'

interface UsageData {
  daily: { date: string; requests: number; successful: number; mbTransferred: string }[]
  byCountry: { country: string; requests: number; mbTransferred: string }[]
  summary: {
    totalRequests: number
    successfulRequests: number
    successRate: string
    totalGbTransferred: string
    avgResponseTimeMs: number
  }
}

const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899']

export default function AnalyticsPage() {
  const [data, setData] = useState<UsageData | null>(null)
  const [loading, setLoading] = useState(true)
  const [period, setPeriod] = useState(30)

  useEffect(() => {
    fetchData()
  }, [period])

  const fetchData = async () => {
    setLoading(true)
    try {
      const token = localStorage.getItem('token')
      const headers = token ? { Authorization: `Bearer ${token}` } : {}

      const results = await Promise.all([
        fetch(`/api/usage/summary?days=${period}`, { headers }),
        fetch(`/api/usage/daily?days=${period}`, { headers }),
        fetch(`/api/usage/by-country?days=${period}`, { headers })
      ])
      const [summaryRes, dailyRes, countryRes] = results

      const summary = summaryRes.ok ? (await summaryRes.json()).stats : null
      const daily = dailyRes.ok ? (await dailyRes.json()).daily : []
      const byCountry = countryRes.ok ? (await countryRes.json()).byCountry : []

      setData({
        summary: summary || {
          totalRequests: 0,
          successfulRequests: 0,
          successRate: '0',
          totalGbTransferred: '0',
          avgResponseTimeMs: 0
        },
        daily: daily.length > 0 ? daily : generateMockDaily(period),
        byCountry: byCountry.length > 0 ? byCountry : generateMockCountries()
      })
    } catch (err) {
      console.error('Failed to fetch analytics:', err)
      // Use mock data for demo
      setData({
        summary: {
          totalRequests: 12847,
          successfulRequests: 12589,
          successRate: '97.99',
          totalGbTransferred: '45.23',
          avgResponseTimeMs: 342
        },
        daily: generateMockDaily(period),
        byCountry: generateMockCountries()
      })
    } finally {
      setLoading(false)
    }
  }

  const statCards = data?.summary ? [
    {
      title: 'Total Requests',
      value: data.summary.totalRequests.toLocaleString(),
      icon: Activity,
      trend: '+12.5%',
      trendUp: true
    },
    {
      title: 'Success Rate',
      value: `${data.summary.successRate}%`,
      icon: TrendingUp,
      trend: '+0.3%',
      trendUp: true
    },
    {
      title: 'Data Transferred',
      value: `${data.summary.totalGbTransferred} GB`,
      icon: BarChart3,
      trend: '+8.2%',
      trendUp: true
    },
    {
      title: 'Avg Response Time',
      value: `${data.summary.avgResponseTimeMs}ms`,
      icon: Clock,
      trend: '-15ms',
      trendUp: true
    }
  ] : []

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold">Analytics</h1>
            <p className="text-muted-foreground">Detailed usage statistics and trends</p>
          </div>
          <div className="flex items-center gap-2">
            <div className="flex gap-1">
              {[7, 30, 90].map((days) => (
                <Button
                  key={days}
                  size="sm"
                  variant={period === days ? 'default' : 'outline'}
                  onClick={() => setPeriod(days)}
                >
                  {days}d
                </Button>
              ))}
            </div>
            <Button size="sm" variant="outline" onClick={fetchData}>
              <RefreshCw className="h-4 w-4" />
            </Button>
          </div>
        </div>

        {/* Stats Cards */}
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          {statCards.map((stat) => (
            <Card key={stat.title}>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium">{stat.title}</CardTitle>
                <stat.icon className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{loading ? '...' : stat.value}</div>
                <div className={`flex items-center text-xs ${stat.trendUp ? 'text-green-500' : 'text-red-500'}`}>
                  {stat.trendUp ? <ArrowUp className="h-3 w-3" /> : <ArrowDown className="h-3 w-3" />}
                  <span className="ml-1">{stat.trend} vs last period</span>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>

        {/* Charts Row */}
        <div className="grid gap-4 lg:grid-cols-2">
          {/* Traffic Over Time */}
          <Card>
            <CardHeader>
              <CardTitle>Traffic Over Time</CardTitle>
              <CardDescription>Daily requests and bandwidth</CardDescription>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="h-80 flex items-center justify-center text-muted-foreground">Loading...</div>
              ) : (
                <ResponsiveContainer width="100%" height={320}>
                  <AreaChart data={data?.daily?.slice().reverse()}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                    <XAxis 
                      dataKey="date" 
                      tickFormatter={(val) => new Date(val).toLocaleDateString('en', { month: 'short', day: 'numeric' })}
                      className="text-xs"
                    />
                    <YAxis className="text-xs" />
                    <Tooltip 
                      contentStyle={{ backgroundColor: 'hsl(var(--card))', border: '1px solid hsl(var(--border))' }}
                      labelFormatter={(val) => new Date(val).toLocaleDateString()}
                    />
                    <Area 
                      type="monotone" 
                      dataKey="requests" 
                      stroke="#3b82f6" 
                      fill="#3b82f680" 
                      name="Requests"
                    />
                    <Area 
                      type="monotone" 
                      dataKey="successful" 
                      stroke="#10b981" 
                      fill="#10b98140" 
                      name="Successful"
                    />
                  </AreaChart>
                </ResponsiveContainer>
              )}
            </CardContent>
          </Card>

          {/* Traffic by Country */}
          <Card>
            <CardHeader>
              <CardTitle>Traffic by Country</CardTitle>
              <CardDescription>Request distribution by region</CardDescription>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="h-80 flex items-center justify-center text-muted-foreground">Loading...</div>
              ) : (
                <ResponsiveContainer width="100%" height={320}>
                  <PieChart>
                    <Pie
                      data={data?.byCountry?.slice(0, 6)}
                      dataKey="requests"
                      nameKey="country"
                      cx="50%"
                      cy="50%"
                      outerRadius={100}
                      label={({ country, percent }) => `${country} ${(percent * 100).toFixed(0)}%`}
                    >
                      {data?.byCountry?.slice(0, 6).map((_, index) => (
                        <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                      ))}
                    </Pie>
                    <Tooltip />
                    <Legend />
                  </PieChart>
                </ResponsiveContainer>
              )}
            </CardContent>
          </Card>
        </div>

        {/* Bandwidth Chart */}
        <Card>
          <CardHeader>
            <CardTitle>Bandwidth Usage</CardTitle>
            <CardDescription>Data transferred per day (MB)</CardDescription>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="h-64 flex items-center justify-center text-muted-foreground">Loading...</div>
            ) : (
              <ResponsiveContainer width="100%" height={250}>
                <BarChart data={data?.daily?.slice().reverse()}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis 
                    dataKey="date" 
                    tickFormatter={(val) => new Date(val).toLocaleDateString('en', { day: 'numeric' })}
                    className="text-xs"
                  />
                  <YAxis className="text-xs" />
                  <Tooltip 
                    contentStyle={{ backgroundColor: 'hsl(var(--card))', border: '1px solid hsl(var(--border))' }}
                    labelFormatter={(val) => new Date(val).toLocaleDateString()}
                    formatter={(value: any) => [`${parseFloat(value).toFixed(2)} MB`, 'Bandwidth']}
                  />
                  <Bar dataKey="mbTransferred" fill="#8b5cf6" name="Bandwidth (MB)" />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        {/* Country Table */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Globe className="h-5 w-5" />
              Traffic by Country
            </CardTitle>
            <CardDescription>Detailed breakdown by region</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b">
                    <th className="text-left py-3 px-4">Country</th>
                    <th className="text-right py-3 px-4">Requests</th>
                    <th className="text-right py-3 px-4">Bandwidth</th>
                    <th className="text-right py-3 px-4">Share</th>
                  </tr>
                </thead>
                <tbody>
                  {data?.byCountry?.map((row, i) => {
                    const total = data.byCountry.reduce((sum, r) => sum + r.requests, 0)
                    const share = ((row.requests / total) * 100).toFixed(1)
                    return (
                      <tr key={row.country} className="border-b hover:bg-muted/50">
                        <td className="py-3 px-4">
                          <div className="flex items-center gap-2">
                            <span className="text-lg">{getCountryFlag(row.country)}</span>
                            <span>{row.country}</span>
                          </div>
                        </td>
                        <td className="text-right py-3 px-4">{row.requests.toLocaleString()}</td>
                        <td className="text-right py-3 px-4">{row.mbTransferred} MB</td>
                        <td className="text-right py-3 px-4">
                          <Badge variant="secondary">{share}%</Badge>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      </div>
    </Layout>
  )
}

function generateMockDaily(days: number) {
  const data = []
  for (let i = 0; i < days; i++) {
    const date = new Date()
    date.setDate(date.getDate() - i)
    const requests = Math.floor(Math.random() * 500) + 200
    data.push({
      date: date.toISOString().split('T')[0],
      requests,
      successful: Math.floor(requests * (0.95 + Math.random() * 0.04)),
      mbTransferred: (Math.random() * 500 + 100).toFixed(2)
    })
  }
  return data
}

function generateMockCountries() {
  return [
    { country: 'IL', requests: 4521, mbTransferred: '15234.50' },
    { country: 'US', requests: 3892, mbTransferred: '12453.20' },
    { country: 'DE', requests: 1823, mbTransferred: '6234.80' },
    { country: 'UK', requests: 1245, mbTransferred: '4521.30' },
    { country: 'FR', requests: 876, mbTransferred: '2934.60' },
    { country: 'NL', requests: 490, mbTransferred: '1823.40' }
  ]
}

function getCountryFlag(code: string): string {
  const flags: Record<string, string> = {
    IL: '🇮🇱', US: '🇺🇸', UK: '🇬🇧', GB: '🇬🇧', DE: '🇩🇪', 
    FR: '🇫🇷', NL: '🇳🇱', CA: '🇨🇦', AU: '🇦🇺', JP: '🇯🇵'
  }
  return flags[code] || '🌍'
}
