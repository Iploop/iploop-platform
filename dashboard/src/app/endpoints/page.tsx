'use client'

import { useState } from 'react'
import { Layout } from '@/components/layout'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Globe, Copy, Terminal, CheckCircle, Server } from 'lucide-react'

const SERVER_IP = 'proxy.iploop.com' // Change to your actual server IP/domain

export default function EndpointsPage() {
  const [copiedItem, setCopiedItem] = useState<string | null>(null)
  const [testResult, setTestResult] = useState<string | null>(null)
  const [testing, setTesting] = useState(false)

  const copyToClipboard = async (text: string, item: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedItem(item)
      setTimeout(() => setCopiedItem(null), 2000)
    } catch (err) {
      console.error('Failed to copy:', err)
    }
  }

  const endpoints = [
    {
      name: 'HTTP Proxy',
      port: 7777,
      protocol: 'HTTP/HTTPS',
      description: 'Standard HTTP proxy with CONNECT support for HTTPS',
      status: 'active',
    },
    {
      name: 'SOCKS5 Proxy',
      port: 1080,
      protocol: 'SOCKS5',
      description: 'SOCKS5 proxy for advanced routing',
      status: 'active',
    },
  ]

  const curlExample = `curl -x http://USERNAME:API_KEY-country-IL@${SERVER_IP}:7777 https://httpbin.org/ip`
  const pythonExample = `import requests

proxies = {
    'http': 'http://USERNAME:API_KEY-country-IL@${SERVER_IP}:7777',
    'https': 'http://USERNAME:API_KEY-country-IL@${SERVER_IP}:7777',
}

response = requests.get('https://httpbin.org/ip', proxies=proxies)
print(response.json())`

  const nodeExample = `const axios = require('axios');

const proxy = {
  host: '${SERVER_IP}',
  port: 7777,
  auth: {
    username: 'USERNAME',
    password: 'API_KEY-country-IL'
  }
};

axios.get('https://httpbin.org/ip', { proxy })
  .then(res => console.log(res.data));`

  return (
    <Layout>
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold">Proxy Endpoints</h1>
          <p className="text-muted-foreground">Connect to IPLoop proxy servers</p>
        </div>

        {/* Active Endpoints */}
        <div className="grid gap-4 md:grid-cols-2">
          {endpoints.map((endpoint) => (
            <Card key={endpoint.name}>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <CardTitle className="flex items-center gap-2">
                    <Server className="h-5 w-5" />
                    {endpoint.name}
                  </CardTitle>
                  <Badge variant="success">{endpoint.status}</Badge>
                </div>
                <CardDescription>{endpoint.description}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between p-3 bg-muted rounded-lg">
                  <code className="text-sm font-mono">
                    {SERVER_IP}:{endpoint.port}
                  </code>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => copyToClipboard(`${SERVER_IP}:${endpoint.port}`, endpoint.name)}
                  >
                    {copiedItem === endpoint.name ? (
                      <CheckCircle className="h-4 w-4 text-green-500" />
                    ) : (
                      <Copy className="h-4 w-4" />
                    )}
                  </Button>
                </div>
                <div className="text-sm text-muted-foreground">
                  Protocol: <Badge variant="outline">{endpoint.protocol}</Badge>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>

        {/* Authentication Format */}
        <Card>
          <CardHeader>
            <CardTitle>Authentication Format</CardTitle>
            <CardDescription>How to authenticate and target specific regions</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="p-4 bg-muted rounded-lg">
              <code className="text-sm">
                USERNAME:API_KEY[-country-XX][-city-CITY][-session-ID]
              </code>
            </div>
            <div className="grid gap-3 text-sm">
              <div className="flex items-start gap-2">
                <Badge variant="outline" className="shrink-0">USERNAME</Badge>
                <span>Any string (not validated, for your reference)</span>
              </div>
              <div className="flex items-start gap-2">
                <Badge variant="outline" className="shrink-0">API_KEY</Badge>
                <span>Your API key from the API Keys page</span>
              </div>
              <div className="flex items-start gap-2">
                <Badge variant="outline" className="shrink-0">-country-XX</Badge>
                <span>Optional: Target country (e.g., IL, US, DE)</span>
              </div>
              <div className="flex items-start gap-2">
                <Badge variant="outline" className="shrink-0">-city-CITY</Badge>
                <span>Optional: Target city (e.g., newyork, london)</span>
              </div>
              <div className="flex items-start gap-2">
                <Badge variant="outline" className="shrink-0">-session-ID</Badge>
                <span>Optional: Session ID for sticky sessions</span>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Code Examples */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Terminal className="h-5 w-5" />
              Quick Start Examples
            </CardTitle>
            <CardDescription>Copy-paste examples to get started</CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {/* cURL */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <h4 className="font-medium">cURL</h4>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => copyToClipboard(curlExample, 'curl')}
                >
                  {copiedItem === 'curl' ? (
                    <CheckCircle className="h-4 w-4 text-green-500" />
                  ) : (
                    <Copy className="h-4 w-4" />
                  )}
                </Button>
              </div>
              <pre className="p-4 bg-muted rounded-lg text-sm overflow-x-auto">
                <code>{curlExample}</code>
              </pre>
            </div>

            {/* Python */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <h4 className="font-medium">Python (requests)</h4>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => copyToClipboard(pythonExample, 'python')}
                >
                  {copiedItem === 'python' ? (
                    <CheckCircle className="h-4 w-4 text-green-500" />
                  ) : (
                    <Copy className="h-4 w-4" />
                  )}
                </Button>
              </div>
              <pre className="p-4 bg-muted rounded-lg text-sm overflow-x-auto">
                <code>{pythonExample}</code>
              </pre>
            </div>

            {/* Node.js */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <h4 className="font-medium">Node.js (axios)</h4>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => copyToClipboard(nodeExample, 'node')}
                >
                  {copiedItem === 'node' ? (
                    <CheckCircle className="h-4 w-4 text-green-500" />
                  ) : (
                    <Copy className="h-4 w-4" />
                  )}
                </Button>
              </div>
              <pre className="p-4 bg-muted rounded-lg text-sm overflow-x-auto">
                <code>{nodeExample}</code>
              </pre>
            </div>
          </CardContent>
        </Card>

        {/* Available Countries */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Globe className="h-5 w-5" />
              Available Regions
            </CardTitle>
            <CardDescription>Currently available proxy locations</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-2">
              <Badge variant="default" className="text-sm">🇮🇱 Israel (IL)</Badge>
              <Badge variant="outline" className="text-sm opacity-50">🇺🇸 United States (US) - Coming Soon</Badge>
              <Badge variant="outline" className="text-sm opacity-50">🇬🇧 United Kingdom (UK) - Coming Soon</Badge>
              <Badge variant="outline" className="text-sm opacity-50">🇩🇪 Germany (DE) - Coming Soon</Badge>
            </div>
            <p className="text-sm text-muted-foreground mt-4">
              More regions are added as new nodes join the network.
            </p>
          </CardContent>
        </Card>
      </div>
    </Layout>
  )
}
