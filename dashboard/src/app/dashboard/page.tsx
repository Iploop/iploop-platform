'use client'

import { useEffect, useState } from 'react'
import dynamic from 'next/dynamic'
import { Layout } from '@/components/layout'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { BarChart3, Globe, Zap, Activity, TrendingUp, Users, Server, Clock, Smartphone, MapPin, Wifi } from 'lucide-react'

const WorldMap = dynamic(() => import('@/components/world-map').then(mod => mod.WorldMap), { 
  ssr: false,
  loading: () => <div className="w-full flex items-center justify-center" style={{ aspectRatio: '2/1' }}><span className="text-muted-foreground">Loading map...</span></div>
})

interface NodeData {
  nodes: any[]
  nodeCount: number
  stats: any
  health: any
}

export default function DashboardPage() {
  const [data, setData] = useState<NodeData | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchData = async () => {
      try {
        const res = await fetch('/api/nodes')
        const json = await res.json()
        setData(json)
      } catch (err) {
        console.error('Failed to fetch:', err)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
    const interval = setInterval(fetchData, 30000)
    return () => clearInterval(interval)
  }, [])

  const activeNodes = data?.nodes?.filter(n => n.status === 'available') || []
  const connectedCount = data?.health?.connected_nodes || data?.stats?.active_nodes || activeNodes.length
  const countryCount = data?.stats?.country_breakdown ? Object.keys(data.stats.country_breakdown).filter((k: string) => k !== '').length : new Set(data?.nodes?.map((n: any) => n.country) || []).size

  const statsData = [
    {
      title: "Active Nodes",
      value: connectedCount.toString(),
      icon: Smartphone,
      description: "Currently online"
    },
    {
      title: "Total Nodes",
      value: data?.stats?.total_nodes?.toString() || '0',
      icon: Users,
      description: "Registered devices"
    },
    {
      title: "Countries",
      value: countryCount.toString(),
      icon: Globe,
      description: "Available regions"
    },
    {
      title: "System Status",
      value: data?.health?.status === 'healthy' ? 'Healthy' : 'Unknown',
      icon: Activity,
      description: "Service health"
    }
  ]

  return (
    <Layout>
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold">Dashboard Overview</h1>
          <p className="text-muted-foreground">IPLoop Residential Proxy Network Status</p>
        </div>

        {/* Stats Grid */}
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          {statsData.map((stat) => (
            <Card key={stat.title}>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">{stat.title}</CardTitle>
                <stat.icon className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{loading ? '...' : stat.value}</div>
                <p className="text-xs text-muted-foreground">{stat.description}</p>
              </CardContent>
            </Card>
          ))}
        </div>

        <div className="grid gap-4 lg:grid-cols-2">
          {/* Active Nodes */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Smartphone className="h-5 w-5" />
                Active Nodes
              </CardTitle>
              <CardDescription>Currently connected devices</CardDescription>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="text-center py-4 text-muted-foreground">Loading...</div>
              ) : activeNodes.length === 0 ? (
                <div className="text-center py-4 text-muted-foreground">No active nodes</div>
              ) : (
                <div className="space-y-3">
                  {activeNodes.map((node) => (
                    <div key={node.id} className="flex items-center justify-between p-3 border rounded-lg">
                      <div className="flex items-center gap-3">
                        <div className="w-2 h-2 bg-green-500 rounded-full" />
                        <div>
                          <div className="font-medium">{node.ip_address}</div>
                          <div className="text-sm text-muted-foreground flex items-center gap-2">
                            <MapPin className="h-3 w-3" />
                            {node.city}, {node.country}
                          </div>
                        </div>
                      </div>
                      <div className="text-right">
                        <Badge variant="outline">{node.device_type}</Badge>
                        <div className="text-xs text-muted-foreground mt-1 flex items-center gap-1">
                          <Wifi className="h-3 w-3" />
                          {node.connection_type}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Quick Stats */}
          <Card>
            <CardHeader>
              <CardTitle>Network Statistics</CardTitle>
              <CardDescription>Real-time network metrics</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Server className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm">Service Status</span>
                </div>
                <Badge variant={data?.health?.status === 'healthy' ? 'success' : 'destructive'}>
                  {data?.health?.status || 'Unknown'}
                </Badge>
              </div>
              
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Users className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm">Connected Nodes</span>
                </div>
                <Badge variant="secondary">{data?.health?.connected_nodes || 0}</Badge>
              </div>
              
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Activity className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm">Average Quality</span>
                </div>
                <Badge variant="secondary">{data?.stats?.average_quality || 0}%</Badge>
              </div>
              
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <BarChart3 className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm">Bandwidth Used</span>
                </div>
                <Badge variant="secondary">{data?.stats?.total_bandwidth_mb || 0} MB</Badge>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Global Node Map */}
        {data?.stats?.country_breakdown && Object.keys(data.stats.country_breakdown).length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Globe className="h-5 w-5" />
                Global Node Distribution
              </CardTitle>
              <CardDescription>
                {countryCount} countries • {connectedCount.toLocaleString()} active nodes worldwide
              </CardDescription>
            </CardHeader>
            <CardContent>
              <WorldMap countryData={data.stats.country_breakdown} />
              {/* Top countries summary */}
              <div className="mt-4 flex flex-wrap gap-2">
                {Object.entries(data.stats.country_breakdown)
                  .sort(([,a], [,b]) => (b as number) - (a as number))
                  .slice(0, 10)
                  .map(([country, count]) => (
                    <div key={country} className="flex items-center gap-1.5 px-3 py-1.5 bg-secondary/50 rounded-full text-sm">
                      <span>{getCountryFlag(country)}</span>
                      <span className="font-medium">{country}</span>
                      <span className="text-muted-foreground">{(count as number).toLocaleString()}</span>
                    </div>
                  ))}
                {Object.keys(data.stats.country_breakdown).length > 10 && (
                  <div className="flex items-center px-3 py-1.5 text-sm text-muted-foreground">
                    +{Object.keys(data.stats.country_breakdown).length - 10} more
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        )}

        {/* Quick Actions */}
        <Card>
          <CardHeader>
            <CardTitle>Quick Actions</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-3">
              <a href="/nodes" className="px-4 py-2 bg-primary text-primary-foreground rounded-md hover:bg-primary/90">
                View All Nodes
              </a>
              <a href="/api-keys" className="px-4 py-2 bg-secondary text-secondary-foreground rounded-md hover:bg-secondary/90">
                Manage API Keys
              </a>
              <a href="/endpoints" className="px-4 py-2 bg-secondary text-secondary-foreground rounded-md hover:bg-secondary/90">
                Proxy Endpoints
              </a>
            </div>
          </CardContent>
        </Card>
      </div>
    </Layout>
  )
}

function getCountryFlag(code: string): string {
  const flags: Record<string, string> = {
    IL: '🇮🇱',
    US: '🇺🇸',
    UK: '🇬🇧',
    GB: '🇬🇧',
    DE: '🇩🇪',
    FR: '🇫🇷',
    CA: '🇨🇦',
    AU: '🇦🇺',
    JP: '🇯🇵',
    KR: '🇰🇷',
    BR: '🇧🇷',
    IN: '🇮🇳',
  }
  return flags[code] || '🌍'
}
