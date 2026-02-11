'use client'

import { Layout } from '@/components/layout'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Smartphone, Globe, Wifi, Clock, MapPin, RefreshCw } from 'lucide-react'
import { useEffect, useState } from 'react'

interface Node {
  id: string
  device_id: string
  ip_address: string
  country: string
  country_name: string
  city: string
  region: string
  asn: number
  isp: string
  carrier: string
  connection_type: string
  device_type: string
  sdk_version: string
  status: string
  quality_score: number
  bandwidth_used_mb: number
  last_heartbeat: string
  connected_since: string
}

interface NodesData {
  nodes: Node[]
  nodeCount: number
  stats: any
  health: any
  timestamp: string
  error?: string
}

export default function NodesPage() {
  const [data, setData] = useState<NodesData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchNodes = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/nodes')
      const json = await res.json()
      setData(json)
      setError(null)
    } catch (err) {
      setError('Failed to fetch nodes')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchNodes()
    // Auto refresh every 10 seconds
    const interval = setInterval(fetchNodes, 10000)
    return () => clearInterval(interval)
  }, [])

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'available': return 'bg-green-500'
      case 'busy': return 'bg-yellow-500'
      case 'offline': return 'bg-red-500'
      default: return 'bg-gray-500'
    }
  }

  const getConnectionIcon = (type: string) => {
    switch (type?.toLowerCase()) {
      case 'wifi': return <Wifi className="h-4 w-4" />
      case '4g': case 'lte': return <Smartphone className="h-4 w-4" />
      default: return <Globe className="h-4 w-4" />
    }
  }

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold">Connected Nodes</h1>
            <p className="text-muted-foreground">Manage and monitor your proxy nodes</p>
          </div>
          <button 
            onClick={fetchNodes}
            className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-md hover:bg-primary/90"
            disabled={loading}
          >
            <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        </div>

        {/* Stats Cards */}
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">Total Nodes</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{data?.nodeCount || 0}</div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">Online</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-green-500">
                {data?.health?.connected_nodes || data?.stats?.active_nodes || 0}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">Countries</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {data?.stats?.country_breakdown ? Object.keys(data.stats.country_breakdown).filter(k => k !== '').length : 0}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">Service Health</CardTitle>
            </CardHeader>
            <CardContent>
              <Badge variant={data?.health?.status === 'healthy' ? 'success' : 'destructive'}>
                {data?.health?.status || 'Unknown'}
              </Badge>
            </CardContent>
          </Card>
        </div>

        {/* Nodes List */}
        <Card>
          <CardHeader>
            <CardTitle>Active Nodes</CardTitle>
            <CardDescription>
              {data?.nodes?.length || 0} nodes currently registered
            </CardDescription>
          </CardHeader>
          <CardContent>
            {loading && !data ? (
              <div className="text-center py-8 text-muted-foreground">Loading...</div>
            ) : error ? (
              <div className="text-center py-8 text-red-500">{error}</div>
            ) : !data?.nodes?.length ? (
              <div className="text-center py-8 text-muted-foreground">
                No nodes connected. Start the SDK on a device to see it here.
              </div>
            ) : (
              <div className="space-y-4">
                {data.nodes.map((node) => (
                  <div key={node.id} className="flex items-center justify-between p-4 border rounded-lg">
                    <div className="flex items-center gap-4">
                      <div className={`w-3 h-3 rounded-full ${getStatusColor(node.status)}`} />
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-medium">{node.ip_address}</span>
                          <Badge variant="outline" className="text-xs">
                            {node.country}
                          </Badge>
                        </div>
                        <div className="flex items-center gap-4 text-sm text-muted-foreground mt-1">
                          <span className="flex items-center gap-1">
                            <MapPin className="h-3 w-3" />
                            {node.city || 'Unknown'}, {node.country_name || node.country}
                          </span>
                          <span className="flex items-center gap-1">
                            {getConnectionIcon(node.connection_type)}
                            {node.connection_type || 'Unknown'}
                          </span>
                          <span className="flex items-center gap-1">
                            <Smartphone className="h-3 w-3" />
                            {node.device_type || 'Unknown'}
                          </span>
                        </div>
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="text-sm font-medium">
                        Quality: {node.quality_score || 100}%
                      </div>
                      <div className="text-xs text-muted-foreground flex items-center gap-1 justify-end">
                        <Clock className="h-3 w-3" />
                        SDK v{node.sdk_version || '1.0.0'}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

      </div>
    </Layout>
  )
}
