'use client'

import { useState } from 'react'
import { Layout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { 
  Globe, 
  Copy, 
  Search, 
  MapPin, 
  Zap, 
  Shield, 
  Filter,
  Code,
  Download,
  ExternalLink
} from 'lucide-react'

interface ProxyEndpoint {
  id: string
  country: string
  countryCode: string
  city: string
  hostname: string
  port: number
  protocol: 'HTTP' | 'HTTPS' | 'SOCKS5'
  status: 'online' | 'offline' | 'maintenance'
  uptime: number
  latency: number
  load: number
  premium: boolean
}

const endpoints: ProxyEndpoint[] = [
  {
    id: '1',
    country: 'United States',
    countryCode: 'US',
    city: 'New York',
    hostname: 'us-ny-01.iploop.com',
    port: 8080,
    protocol: 'HTTP',
    status: 'online',
    uptime: 99.9,
    latency: 12,
    load: 45,
    premium: true
  },
  {
    id: '2',
    country: 'United States',
    countryCode: 'US',
    city: 'Los Angeles',
    hostname: 'us-la-01.iploop.com',
    port: 8080,
    protocol: 'HTTPS',
    status: 'online',
    uptime: 99.8,
    latency: 18,
    load: 32,
    premium: true
  },
  {
    id: '3',
    country: 'United Kingdom',
    countryCode: 'GB',
    city: 'London',
    hostname: 'uk-lon-01.iploop.com',
    port: 8080,
    protocol: 'HTTP',
    status: 'online',
    uptime: 99.7,
    latency: 28,
    load: 67,
    premium: false
  },
  {
    id: '4',
    country: 'Germany',
    countryCode: 'DE',
    city: 'Frankfurt',
    hostname: 'de-fra-01.iploop.com',
    port: 8080,
    protocol: 'SOCKS5',
    status: 'online',
    uptime: 99.9,
    latency: 15,
    load: 23,
    premium: true
  },
  {
    id: '5',
    country: 'Japan',
    countryCode: 'JP',
    city: 'Tokyo',
    hostname: 'jp-tok-01.iploop.com',
    port: 8080,
    protocol: 'HTTP',
    status: 'maintenance',
    uptime: 98.5,
    latency: 89,
    load: 0,
    premium: false
  },
  {
    id: '6',
    country: 'Singapore',
    countryCode: 'SG',
    city: 'Singapore',
    hostname: 'sg-sin-01.iploop.com',
    port: 8080,
    protocol: 'HTTPS',
    status: 'online',
    uptime: 99.6,
    latency: 45,
    load: 78,
    premium: true
  },
  {
    id: '7',
    country: 'Australia',
    countryCode: 'AU',
    city: 'Sydney',
    hostname: 'au-syd-01.iploop.com',
    port: 8080,
    protocol: 'HTTP',
    status: 'online',
    uptime: 99.4,
    latency: 95,
    load: 56,
    premium: false
  },
  {
    id: '8',
    country: 'Canada',
    countryCode: 'CA',
    city: 'Toronto',
    hostname: 'ca-tor-01.iploop.com',
    port: 8080,
    protocol: 'SOCKS5',
    status: 'offline',
    uptime: 95.2,
    latency: 0,
    load: 0,
    premium: false
  }
]

const configExamples = {
  curl: (endpoint: ProxyEndpoint) => `curl -x ${endpoint.hostname}:${endpoint.port} \\
  -U "username:password" \\
  https://httpbin.org/ip`,
  
  python: (endpoint: ProxyEndpoint) => `import requests

proxies = {
    'http': 'http://username:password@${endpoint.hostname}:${endpoint.port}',
    'https': 'https://username:password@${endpoint.hostname}:${endpoint.port}'
}

response = requests.get('https://httpbin.org/ip', proxies=proxies)
print(response.json())`,

  javascript: (endpoint: ProxyEndpoint) => `const axios = require('axios');

const config = {
  proxy: {
    host: '${endpoint.hostname}',
    port: ${endpoint.port},
    auth: {
      username: 'username',
      password: 'password'
    }
  }
};

axios.get('https://httpbin.org/ip', config)
  .then(response => console.log(response.data));`
}

export default function EndpointsPage() {
  const [searchTerm, setSearchTerm] = useState('')
  const [selectedProtocol, setSelectedProtocol] = useState<string>('all')
  const [selectedStatus, setSelectedStatus] = useState<string>('all')
  const [copiedText, setCopiedText] = useState('')
  const [selectedEndpoint, setSelectedEndpoint] = useState<ProxyEndpoint | null>(null)
  const [selectedCodeExample, setSelectedCodeExample] = useState('curl')

  const filteredEndpoints = endpoints.filter(endpoint => {
    const matchesSearch = endpoint.country.toLowerCase().includes(searchTerm.toLowerCase()) ||
                         endpoint.city.toLowerCase().includes(searchTerm.toLowerCase()) ||
                         endpoint.hostname.toLowerCase().includes(searchTerm.toLowerCase())
    
    const matchesProtocol = selectedProtocol === 'all' || endpoint.protocol === selectedProtocol
    const matchesStatus = selectedStatus === 'all' || endpoint.status === selectedStatus
    
    return matchesSearch && matchesProtocol && matchesStatus
  })

  const copyToClipboard = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedText(label)
      setTimeout(() => setCopiedText(''), 2000)
    } catch (err) {
      console.error('Failed to copy: ', err)
    }
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'online': return 'success'
      case 'offline': return 'destructive'
      case 'maintenance': return 'warning'
      default: return 'secondary'
    }
  }

  const getLoadColor = (load: number) => {
    if (load < 30) return 'text-green-500'
    if (load < 70) return 'text-yellow-500'
    return 'text-red-500'
  }

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold">Proxy Endpoints</h1>
            <p className="text-muted-foreground">Global proxy network endpoints and configurations</p>
          </div>
          <div className="flex gap-2">
            <Button variant="outline">
              <Download className="w-4 h-4 mr-2" />
              Export List
            </Button>
            <Button variant="outline">
              <ExternalLink className="w-4 h-4 mr-2" />
              Network Status
            </Button>
          </div>
        </div>

        {/* Filters */}
        <Card>
          <CardContent className="pt-6">
            <div className="flex flex-col lg:flex-row gap-4">
              <div className="flex-1">
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-muted-foreground w-4 h-4" />
                  <Input
                    placeholder="Search by country, city, or hostname..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="pl-10"
                  />
                </div>
              </div>
              <div className="flex gap-2">
                <select
                  value={selectedProtocol}
                  onChange={(e) => setSelectedProtocol(e.target.value)}
                  className="px-3 py-2 border border-input bg-background rounded-md text-sm"
                >
                  <option value="all">All Protocols</option>
                  <option value="HTTP">HTTP</option>
                  <option value="HTTPS">HTTPS</option>
                  <option value="SOCKS5">SOCKS5</option>
                </select>
                <select
                  value={selectedStatus}
                  onChange={(e) => setSelectedStatus(e.target.value)}
                  className="px-3 py-2 border border-input bg-background rounded-md text-sm"
                >
                  <option value="all">All Status</option>
                  <option value="online">Online</option>
                  <option value="offline">Offline</option>
                  <option value="maintenance">Maintenance</option>
                </select>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Endpoints Grid */}
        <div className="grid gap-4 lg:grid-cols-2">
          {filteredEndpoints.map((endpoint) => (
            <Card key={endpoint.id} className="hover:shadow-md transition-shadow">
              <CardContent className="pt-6">
                <div className="space-y-4">
                  <div className="flex items-start justify-between">
                    <div className="flex items-center gap-3">
                      <div className="flex items-center gap-2">
                        <MapPin className="w-4 h-4 text-muted-foreground" />
                        <span className="font-medium">{endpoint.city}, {endpoint.country}</span>
                        {endpoint.premium && (
                          <Badge variant="secondary" className="text-xs">Premium</Badge>
                        )}
                      </div>
                    </div>
                    <Badge variant={getStatusColor(endpoint.status)}>
                      {endpoint.status}
                    </Badge>
                  </div>

                  <div className="bg-muted/50 p-3 rounded-lg">
                    <div className="flex items-center justify-between">
                      <code className="text-sm font-mono">
                        {endpoint.hostname}:{endpoint.port}
                      </code>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => copyToClipboard(`${endpoint.hostname}:${endpoint.port}`, `${endpoint.id}-endpoint`)}
                      >
                        {copiedText === `${endpoint.id}-endpoint` ? (
                          <span className="text-green-500 text-xs">✓</span>
                        ) : (
                          <Copy className="w-4 h-4" />
                        )}
                      </Button>
                    </div>
                    <div className="mt-2 text-xs text-muted-foreground">
                      Protocol: {endpoint.protocol}
                    </div>
                  </div>

                  <div className="grid grid-cols-3 gap-4 text-sm">
                    <div>
                      <div className="text-muted-foreground">Uptime</div>
                      <div className="font-medium">{endpoint.uptime}%</div>
                    </div>
                    <div>
                      <div className="text-muted-foreground">Latency</div>
                      <div className="font-medium">{endpoint.latency}ms</div>
                    </div>
                    <div>
                      <div className="text-muted-foreground">Load</div>
                      <div className={`font-medium ${getLoadColor(endpoint.load)}`}>
                        {endpoint.load}%
                      </div>
                    </div>
                  </div>

                  <div className="flex gap-2">
                    <Button 
                      variant="outline" 
                      size="sm" 
                      onClick={() => setSelectedEndpoint(endpoint)}
                    >
                      <Code className="w-4 h-4 mr-2" />
                      View Config
                    </Button>
                    <Button 
                      variant="outline" 
                      size="sm"
                      onClick={() => copyToClipboard(
                        `${endpoint.hostname}:${endpoint.port}`,
                        `${endpoint.id}-full`
                      )}
                    >
                      <Copy className="w-4 h-4 mr-2" />
                      Copy
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>

        {/* Configuration Modal/Card */}
        {selectedEndpoint && (
          <Card className="border-2 border-primary">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>Configuration Examples</CardTitle>
                  <CardDescription>
                    {selectedEndpoint.city}, {selectedEndpoint.country} - {selectedEndpoint.hostname}:{selectedEndpoint.port}
                  </CardDescription>
                </div>
                <Button variant="ghost" onClick={() => setSelectedEndpoint(null)}>
                  ×
                </Button>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex gap-1">
                {Object.keys(configExamples).map((lang) => (
                  <Button
                    key={lang}
                    variant={selectedCodeExample === lang ? 'default' : 'outline'}
                    size="sm"
                    onClick={() => setSelectedCodeExample(lang)}
                  >
                    {lang.charAt(0).toUpperCase() + lang.slice(1)}
                  </Button>
                ))}
              </div>
              
              <div className="relative">
                <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
                  <code>{configExamples[selectedCodeExample as keyof typeof configExamples](selectedEndpoint)}</code>
                </pre>
                <Button
                  variant="ghost"
                  size="icon"
                  className="absolute top-2 right-2"
                  onClick={() => copyToClipboard(
                    configExamples[selectedCodeExample as keyof typeof configExamples](selectedEndpoint),
                    'config-example'
                  )}
                >
                  {copiedText === 'config-example' ? (
                    <span className="text-green-500 text-xs">✓</span>
                  ) : (
                    <Copy className="w-4 h-4" />
                  )}
                </Button>
              </div>
            </CardContent>
          </Card>
        )}

        {/* Network Stats Summary */}
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardContent className="pt-6 text-center">
              <Globe className="w-8 h-8 text-primary mx-auto mb-2" />
              <div className="text-2xl font-bold">{endpoints.length}</div>
              <div className="text-sm text-muted-foreground">Total Endpoints</div>
            </CardContent>
          </Card>
          
          <Card>
            <CardContent className="pt-6 text-center">
              <Zap className="w-8 h-8 text-green-500 mx-auto mb-2" />
              <div className="text-2xl font-bold">{endpoints.filter(e => e.status === 'online').length}</div>
              <div className="text-sm text-muted-foreground">Online Now</div>
            </CardContent>
          </Card>
          
          <Card>
            <CardContent className="pt-6 text-center">
              <Shield className="w-8 h-8 text-blue-500 mx-auto mb-2" />
              <div className="text-2xl font-bold">{endpoints.filter(e => e.premium).length}</div>
              <div className="text-sm text-muted-foreground">Premium Tier</div>
            </CardContent>
          </Card>
          
          <Card>
            <CardContent className="pt-6 text-center">
              <MapPin className="w-8 h-8 text-purple-500 mx-auto mb-2" />
              <div className="text-2xl font-bold">{new Set(endpoints.map(e => e.country)).size}</div>
              <div className="text-sm text-muted-foreground">Countries</div>
            </CardContent>
          </Card>
        </div>
      </div>
    </Layout>
  )
}