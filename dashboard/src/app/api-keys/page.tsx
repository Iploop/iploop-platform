'use client'

import { useState } from 'react'
import { Layout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Key, Copy, Eye, EyeOff, Plus, Trash2, Calendar, Shield } from 'lucide-react'

interface ApiKey {
  id: string
  name: string
  key: string
  created: string
  lastUsed: string
  status: 'active' | 'inactive'
  permissions: string[]
}

const initialApiKeys: ApiKey[] = [
  {
    id: '1',
    name: 'Production API',
    key: 'ipl_sk_live_1a2b3c4d5e6f7g8h9i0j1k2l3m4n5o6p',
    created: '2024-01-15',
    lastUsed: '2024-02-03',
    status: 'active',
    permissions: ['read', 'write', 'admin']
  },
  {
    id: '2',
    name: 'Development API',
    key: 'ipl_sk_test_9z8y7x6w5v4u3t2s1r0q9p8o7n6m5l4k',
    created: '2024-01-20',
    lastUsed: '2024-02-02',
    status: 'active',
    permissions: ['read', 'write']
  },
  {
    id: '3',
    name: 'Mobile App API',
    key: 'ipl_sk_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6',
    created: '2024-01-25',
    lastUsed: 'Never',
    status: 'inactive',
    permissions: ['read']
  }
]

export default function ApiKeysPage() {
  const [apiKeys, setApiKeys] = useState<ApiKey[]>(initialApiKeys)
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [visibleKeys, setVisibleKeys] = useState<Set<string>>(new Set())
  const [copiedKey, setCopiedKey] = useState<string>('')

  const generateNewKey = () => {
    if (!newKeyName.trim()) return

    const newKey: ApiKey = {
      id: Date.now().toString(),
      name: newKeyName,
      key: `ipl_sk_live_${Math.random().toString(36).substring(2, 40)}`,
      created: new Date().toISOString().split('T')[0],
      lastUsed: 'Never',
      status: 'active',
      permissions: ['read', 'write']
    }

    setApiKeys([...apiKeys, newKey])
    setNewKeyName('')
    setShowCreateForm(false)
  }

  const revokeKey = (id: string) => {
    setApiKeys(apiKeys.filter(key => key.id !== id))
  }

  const toggleKeyVisibility = (id: string) => {
    const newVisible = new Set(visibleKeys)
    if (newVisible.has(id)) {
      newVisible.delete(id)
    } else {
      newVisible.add(id)
    }
    setVisibleKeys(newVisible)
  }

  const copyToClipboard = async (text: string, keyId: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedKey(keyId)
      setTimeout(() => setCopiedKey(''), 2000)
    } catch (err) {
      console.error('Failed to copy: ', err)
    }
  }

  const maskKey = (key: string) => {
    if (key.length <= 8) return key
    return key.substring(0, 12) + '•'.repeat(20) + key.substring(key.length - 8)
  }

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold">API Keys</h1>
            <p className="text-muted-foreground">Manage your API keys for accessing IPLoop services</p>
          </div>
          <Button onClick={() => setShowCreateForm(true)}>
            <Plus className="w-4 h-4 mr-2" />
            Create New Key
          </Button>
        </div>

        {/* Create New Key Form */}
        {showCreateForm && (
          <Card>
            <CardHeader>
              <CardTitle>Create New API Key</CardTitle>
              <CardDescription>Generate a new API key for your applications</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4">
                <div className="space-y-2">
                  <label htmlFor="keyName" className="text-sm font-medium">
                    Key Name
                  </label>
                  <Input
                    id="keyName"
                    placeholder="e.g., Production API, Mobile App"
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                  />
                </div>
                <div className="flex gap-2">
                  <Button onClick={generateNewKey} disabled={!newKeyName.trim()}>
                    Generate Key
                  </Button>
                  <Button variant="outline" onClick={() => setShowCreateForm(false)}>
                    Cancel
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        )}

        {/* Security Notice */}
        <Card className="border-yellow-200 bg-yellow-50 dark:bg-yellow-900/20 dark:border-yellow-800">
          <CardContent className="pt-6">
            <div className="flex items-start gap-3">
              <Shield className="w-5 h-5 text-yellow-600 dark:text-yellow-400 mt-0.5" />
              <div>
                <h3 className="font-semibold text-yellow-800 dark:text-yellow-200">Security Best Practices</h3>
                <p className="text-sm text-yellow-700 dark:text-yellow-300 mt-1">
                  Keep your API keys secure and never share them publicly. Regenerate keys immediately if compromised.
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* API Keys List */}
        <div className="space-y-4">
          {apiKeys.map((apiKey) => (
            <Card key={apiKey.id}>
              <CardContent className="pt-6">
                <div className="flex items-start justify-between">
                  <div className="space-y-3 flex-1">
                    <div className="flex items-center gap-3">
                      <Key className="w-5 h-5 text-muted-foreground" />
                      <h3 className="font-semibold">{apiKey.name}</h3>
                      <Badge variant={apiKey.status === 'active' ? 'success' : 'secondary'}>
                        {apiKey.status}
                      </Badge>
                    </div>
                    
                    <div className="flex items-center gap-2 bg-muted/50 p-3 rounded-lg">
                      <code className="flex-1 text-sm font-mono">
                        {visibleKeys.has(apiKey.id) ? apiKey.key : maskKey(apiKey.key)}
                      </code>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => toggleKeyVisibility(apiKey.id)}
                      >
                        {visibleKeys.has(apiKey.id) ? (
                          <EyeOff className="w-4 h-4" />
                        ) : (
                          <Eye className="w-4 h-4" />
                        )}
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => copyToClipboard(apiKey.key, apiKey.id)}
                      >
                        {copiedKey === apiKey.id ? (
                          <span className="text-green-500 text-xs font-medium">Copied!</span>
                        ) : (
                          <Copy className="w-4 h-4" />
                        )}
                      </Button>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm text-muted-foreground">
                      <div className="flex items-center gap-2">
                        <Calendar className="w-4 h-4" />
                        <span>Created: {apiKey.created}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <Calendar className="w-4 h-4" />
                        <span>Last used: {apiKey.lastUsed}</span>
                      </div>
                      <div>
                        <span>Permissions: {apiKey.permissions.join(', ')}</span>
                      </div>
                    </div>
                  </div>
                  
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => revokeKey(apiKey.id)}
                    className="text-destructive hover:text-destructive"
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>

        {/* API Documentation Link */}
        <Card>
          <CardHeader>
            <CardTitle>API Documentation</CardTitle>
            <CardDescription>Learn how to integrate with IPLoop APIs</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <p className="text-sm text-muted-foreground">
                Get started with our comprehensive API documentation and integration guides.
              </p>
              <div className="flex gap-2">
                <Button variant="outline">View Documentation</Button>
                <Button variant="outline">Download SDK</Button>
                <Button variant="outline">View Examples</Button>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </Layout>
  )
}