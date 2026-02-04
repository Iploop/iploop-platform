'use client'

import { useState } from 'react'
import { Layout } from '@/components/layout'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { 
  Book, Code, Key, Globe, Zap, Copy, Check,
  ChevronDown, ChevronRight, Smartphone
} from 'lucide-react'

const API_BASE = 'https://api.iploop.io'
const PROXY_HOST = 'proxy.iploop.com'

export default function DocsPage() {
  const [copiedCode, setCopiedCode] = useState<string | null>(null)
  const [expandedSections, setExpandedSections] = useState<string[]>(['quickstart'])

  const copyCode = (code: string, id: string) => {
    navigator.clipboard.writeText(code)
    setCopiedCode(id)
    setTimeout(() => setCopiedCode(null), 2000)
  }

  const toggleSection = (section: string) => {
    setExpandedSections(prev => 
      prev.includes(section) 
        ? prev.filter(s => s !== section)
        : [...prev, section]
    )
  }

  const CodeBlock = ({ code, language = 'bash', id }: { code: string; language?: string; id: string }) => (
    <div className="relative">
      <pre className="bg-zinc-900 text-zinc-100 p-4 rounded-lg overflow-x-auto text-sm">
        <code>{code}</code>
      </pre>
      <Button
        size="sm"
        variant="ghost"
        className="absolute top-2 right-2 h-8 w-8 p-0"
        onClick={() => copyCode(code, id)}
      >
        {copiedCode === id ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4" />}
      </Button>
    </div>
  )

  const Section = ({ id, title, children }: { id: string; title: string; children: React.ReactNode }) => {
    const isExpanded = expandedSections.includes(id)
    return (
      <div className="border rounded-lg">
        <button
          className="w-full flex items-center justify-between p-4 text-left hover:bg-muted/50"
          onClick={() => toggleSection(id)}
        >
          <h3 className="text-lg font-semibold">{title}</h3>
          {isExpanded ? <ChevronDown className="h-5 w-5" /> : <ChevronRight className="h-5 w-5" />}
        </button>
        {isExpanded && <div className="p-4 pt-0 space-y-4">{children}</div>}
      </div>
    )
  }

  return (
    <Layout>
      <div className="space-y-6 max-w-4xl">
        <div>
          <h1 className="text-3xl font-bold flex items-center gap-2">
            <Book className="h-8 w-8" />
            API Documentation
          </h1>
          <p className="text-muted-foreground mt-2">
            Everything you need to integrate IPLoop residential proxies
          </p>
        </div>

        {/* Quick Links */}
        <div className="grid gap-4 md:grid-cols-4">
          <Card className="cursor-pointer hover:border-primary" onClick={() => toggleSection('quickstart')}>
            <CardHeader className="pb-2">
              <Zap className="h-8 w-8 text-yellow-500" />
              <CardTitle className="text-lg">Quick Start</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">Get started in 5 minutes</p>
            </CardContent>
          </Card>
          
          <Card className="cursor-pointer hover:border-primary" onClick={() => toggleSection('auth')}>
            <CardHeader className="pb-2">
              <Key className="h-8 w-8 text-blue-500" />
              <CardTitle className="text-lg">Authentication</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">API keys & auth methods</p>
            </CardContent>
          </Card>
          
          <Card className="cursor-pointer hover:border-primary" onClick={() => toggleSection('proxy')}>
            <CardHeader className="pb-2">
              <Globe className="h-8 w-8 text-green-500" />
              <CardTitle className="text-lg">Proxy Usage</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">HTTP & SOCKS5 examples</p>
            </CardContent>
          </Card>
          
          <Card className="cursor-pointer hover:border-primary" onClick={() => toggleSection('sdks')}>
            <CardHeader className="pb-2">
              <Smartphone className="h-8 w-8 text-purple-500" />
              <CardTitle className="text-lg">SDKs</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">Android, iOS, Mac, Win</p>
            </CardContent>
          </Card>
        </div>

        {/* Documentation Sections */}
        <div className="space-y-4">
          <Section id="quickstart" title="🚀 Quick Start">
            <p className="text-muted-foreground">
              Start using IPLoop proxies in just a few steps:
            </p>
            
            <div className="space-y-4">
              <div>
                <h4 className="font-medium mb-2">1. Get your API Key</h4>
                <p className="text-sm text-muted-foreground mb-2">
                  Go to <a href="/api-keys" className="text-primary hover:underline">API Keys</a> page and create a new key.
                </p>
              </div>
              
              <div>
                <h4 className="font-medium mb-2">2. Make your first request</h4>
                <CodeBlock 
                  id="quickstart-curl"
                  code={`curl -x http://YOUR_API_KEY:@${PROXY_HOST}:7777 https://httpbin.org/ip`}
                />
              </div>
              
              <div>
                <h4 className="font-medium mb-2">3. Target specific countries</h4>
                <CodeBlock 
                  id="quickstart-country"
                  code={`# Add country code to your API key
curl -x http://YOUR_API_KEY-country-US:@${PROXY_HOST}:7777 https://httpbin.org/ip

# Available: US, IL, DE, UK, FR, etc.`}
                />
              </div>
            </div>
          </Section>

          <Section id="auth" title="🔐 Authentication">
            <p className="text-muted-foreground mb-4">
              All requests require authentication via API key.
            </p>
            
            <h4 className="font-medium mb-2">API Key Format</h4>
            <CodeBlock 
              id="auth-format"
              code={`# Basic format
username: YOUR_API_KEY
password: (empty or any value)

# With targeting options
username: YOUR_API_KEY-country-US-session-abc123
password: (empty)`}
            />
            
            <h4 className="font-medium mt-4 mb-2">Targeting Options</h4>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b">
                    <th className="text-left py-2 px-3">Parameter</th>
                    <th className="text-left py-2 px-3">Example</th>
                    <th className="text-left py-2 px-3">Description</th>
                  </tr>
                </thead>
                <tbody>
                  <tr className="border-b">
                    <td className="py-2 px-3"><code>country</code></td>
                    <td className="py-2 px-3"><code>-country-US</code></td>
                    <td className="py-2 px-3">Target specific country (ISO 3166-1)</td>
                  </tr>
                  <tr className="border-b">
                    <td className="py-2 px-3"><code>city</code></td>
                    <td className="py-2 px-3"><code>-city-newyork</code></td>
                    <td className="py-2 px-3">Target specific city</td>
                  </tr>
                  <tr className="border-b">
                    <td className="py-2 px-3"><code>session</code></td>
                    <td className="py-2 px-3"><code>-session-abc123</code></td>
                    <td className="py-2 px-3">Sticky session (same IP)</td>
                  </tr>
                  <tr className="border-b">
                    <td className="py-2 px-3"><code>rotate</code></td>
                    <td className="py-2 px-3"><code>-rotate-10</code></td>
                    <td className="py-2 px-3">Rotate IP every N requests</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </Section>

          <Section id="proxy" title="🌐 Proxy Endpoints">
            <div className="grid gap-4 md:grid-cols-2 mb-4">
              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-base">HTTP/HTTPS Proxy</CardTitle>
                </CardHeader>
                <CardContent>
                  <code className="text-sm bg-muted px-2 py-1 rounded">{PROXY_HOST}:7777</code>
                </CardContent>
              </Card>
              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-base">SOCKS5 Proxy</CardTitle>
                </CardHeader>
                <CardContent>
                  <code className="text-sm bg-muted px-2 py-1 rounded">{PROXY_HOST}:1080</code>
                </CardContent>
              </Card>
            </div>

            <h4 className="font-medium mb-2">cURL</h4>
            <CodeBlock 
              id="proxy-curl"
              code={`# HTTP Proxy
curl -x http://API_KEY:@${PROXY_HOST}:7777 https://httpbin.org/ip

# With country targeting
curl -x http://API_KEY-country-IL:@${PROXY_HOST}:7777 https://httpbin.org/ip

# SOCKS5 Proxy
curl --socks5 API_KEY:@${PROXY_HOST}:1080 https://httpbin.org/ip`}
            />

            <h4 className="font-medium mt-4 mb-2">Python</h4>
            <CodeBlock 
              id="proxy-python"
              language="python"
              code={`import requests

API_KEY = "your_api_key"
PROXY = f"http://{API_KEY}:@${PROXY_HOST}:7777"

# Basic request
response = requests.get(
    "https://httpbin.org/ip",
    proxies={"http": PROXY, "https": PROXY}
)
print(response.json())

# With country targeting
PROXY_US = f"http://{API_KEY}-country-US:@${PROXY_HOST}:7777"
response = requests.get(
    "https://httpbin.org/ip",
    proxies={"http": PROXY_US, "https": PROXY_US}
)`}
            />

            <h4 className="font-medium mt-4 mb-2">Node.js</h4>
            <CodeBlock 
              id="proxy-nodejs"
              language="javascript"
              code={`const axios = require('axios');
const HttpsProxyAgent = require('https-proxy-agent');

const API_KEY = 'your_api_key';
const proxy = \`http://\${API_KEY}:@${PROXY_HOST}:7777\`;
const agent = new HttpsProxyAgent(proxy);

axios.get('https://httpbin.org/ip', { httpsAgent: agent })
  .then(res => console.log(res.data))
  .catch(err => console.error(err));`}
            />
          </Section>

          <Section id="sessions" title="🔄 Sessions & IP Rotation">
            <p className="text-muted-foreground mb-4">
              Control IP persistence and rotation behavior.
            </p>

            <h4 className="font-medium mb-2">Sticky Sessions</h4>
            <p className="text-sm text-muted-foreground mb-2">
              Use the same IP for multiple requests by specifying a session ID:
            </p>
            <CodeBlock 
              id="session-sticky"
              code={`# All requests with same session ID will use same IP
curl -x http://API_KEY-session-mysession123:@${PROXY_HOST}:7777 https://httpbin.org/ip

# Sessions expire after 30 minutes of inactivity`}
            />

            <h4 className="font-medium mt-4 mb-2">Auto Rotation</h4>
            <p className="text-sm text-muted-foreground mb-2">
              Automatically rotate IP after N requests:
            </p>
            <CodeBlock 
              id="session-rotate"
              code={`# Rotate IP every 10 requests
curl -x http://API_KEY-session-abc-rotate-10:@${PROXY_HOST}:7777 https://httpbin.org/ip`}
            />
          </Section>

          <Section id="errors" title="⚠️ Error Codes">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b">
                    <th className="text-left py-2 px-3">Code</th>
                    <th className="text-left py-2 px-3">Meaning</th>
                    <th className="text-left py-2 px-3">Solution</th>
                  </tr>
                </thead>
                <tbody>
                  <tr className="border-b">
                    <td className="py-2 px-3"><Badge variant="destructive">407</Badge></td>
                    <td className="py-2 px-3">Authentication Required</td>
                    <td className="py-2 px-3">Check your API key</td>
                  </tr>
                  <tr className="border-b">
                    <td className="py-2 px-3"><Badge variant="destructive">429</Badge></td>
                    <td className="py-2 px-3">Rate Limited</td>
                    <td className="py-2 px-3">Slow down requests</td>
                  </tr>
                  <tr className="border-b">
                    <td className="py-2 px-3"><Badge variant="destructive">502</Badge></td>
                    <td className="py-2 px-3">No Available Nodes</td>
                    <td className="py-2 px-3">Try different country/city</td>
                  </tr>
                  <tr className="border-b">
                    <td className="py-2 px-3"><Badge variant="destructive">503</Badge></td>
                    <td className="py-2 px-3">Service Unavailable</td>
                    <td className="py-2 px-3">Retry after a moment</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </Section>

          <Section id="api" title="📡 REST API">
            <p className="text-muted-foreground mb-4">
              Manage your account programmatically.
            </p>

            <h4 className="font-medium mb-2">Base URL</h4>
            <CodeBlock id="api-base" code={`${API_BASE}/api/v1`} />

            <h4 className="font-medium mt-4 mb-2">Get Usage Stats</h4>
            <CodeBlock 
              id="api-usage"
              code={`curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \\
  ${API_BASE}/api/v1/usage/summary?days=30`}
            />

            <h4 className="font-medium mt-4 mb-2">List API Keys</h4>
            <CodeBlock 
              id="api-keys"
              code={`curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \\
  ${API_BASE}/api/v1/auth/api-keys`}
            />

            <h4 className="font-medium mt-4 mb-2">Create API Key</h4>
            <CodeBlock 
              id="api-create-key"
              code={`curl -X POST -H "Authorization: Bearer YOUR_JWT_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"name": "Production Key"}' \\
  ${API_BASE}/api/v1/auth/api-keys`}
            />
          </Section>
        </div>

          <Section id="sdks" title="📱 SDKs - Node Integration">
            <p className="text-muted-foreground mb-4">
              Integrate your apps into the IPLoop network as proxy nodes.
            </p>

            <div className="grid gap-4 md:grid-cols-2">
              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-base">🤖 Android SDK</CardTitle>
                </CardHeader>
                <CardContent className="text-sm">
                  <p className="text-muted-foreground mb-2">Kotlin/Java library</p>
                  <CodeBlock id="sdk-android" code={`// build.gradle
implementation 'io.iploop:sdk:1.0.0'

// Usage
IPLoopSDK.initialize(context, "API_KEY")
IPLoopSDK.start()`} />
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-base">🍎 iOS SDK</CardTitle>
                </CardHeader>
                <CardContent className="text-sm">
                  <p className="text-muted-foreground mb-2">Swift Package</p>
                  <CodeBlock id="sdk-ios" code={`// Package.swift
.package(url: "github.com/iploop/ios-sdk", from: "1.0.0")

// Usage
IPLoopSDK.shared.initialize(apiKey: "API_KEY")
try await IPLoopSDK.shared.start()`} />
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-base">🖥️ macOS SDK</CardTitle>
                </CardHeader>
                <CardContent className="text-sm">
                  <p className="text-muted-foreground mb-2">Swift + CLI Daemon</p>
                  <CodeBlock id="sdk-macos" code={`# As daemon
IPLOOP_API_KEY=your_key ./iploop-daemon

# As library
IPLoopSDK.shared.initialize(apiKey: "API_KEY")
try await IPLoopSDK.shared.start()`} />
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-base">🪟 Windows SDK (DLL)</CardTitle>
                </CardHeader>
                <CardContent className="text-sm">
                  <p className="text-muted-foreground mb-2">.NET / Native DLL</p>
                  <CodeBlock id="sdk-windows" code={`// C# (.NET)
IPLoopSDK.Shared.Initialize("API_KEY");
await IPLoopSDK.Shared.StartAsync();

// C++ / Delphi / VB6
IPLoop_Initialize("API_KEY");
IPLoop_Start();`} />
                </CardContent>
              </Card>
            </div>

            <div className="mt-4 p-4 bg-muted rounded-lg">
              <h4 className="font-medium mb-2">SDK Features</h4>
              <ul className="text-sm text-muted-foreground space-y-1">
                <li>✅ Auto device registration</li>
                <li>✅ Heartbeat & health monitoring</li>
                <li>✅ Bandwidth tracking</li>
                <li>✅ GDPR consent management</li>
                <li>✅ Background service support</li>
              </ul>
            </div>
          </Section>

        {/* Support */}
        <Card>
          <CardHeader>
            <CardTitle>Need Help?</CardTitle>
            <CardDescription>Contact our support team</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">
              Email: <a href="mailto:support@iploop.io" className="text-primary hover:underline">support@iploop.io</a>
            </p>
          </CardContent>
        </Card>
      </div>
    </Layout>
  )
}
