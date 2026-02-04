'use client'

import { useState, useEffect } from 'react'
import { Layout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Key, Copy, Eye, EyeOff, Plus, Trash2, Calendar, Shield, Loader2, AlertCircle, Check } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface ApiKey {
  id: string
  name: string
  keyPrefix: string
  apiKey?: string // Only shown once when created
  isActive: boolean
  createdAt: string
  lastUsedAt: string | null
}

export default function ApiKeysPage() {
  const [apiKeys, setApiKeys] = useState<ApiKey[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [creating, setCreating] = useState(false)
  const [newlyCreatedKey, setNewlyCreatedKey] = useState<string | null>(null)
  const [copiedKey, setCopiedKey] = useState<string>('')

  useEffect(() => {
    fetchKeys()
  }, [])

  const fetchKeys = async () => {
    try {
      const res = await fetch('/api/proxy/keys')
      const data = await res.json()
      if (res.ok) {
        setApiKeys(data.keys || [])
      } else {
        setError(data.error || 'Failed to load API keys')
      }
    } catch (err) {
      setError('Failed to connect to server')
    } finally {
      setLoading(false)
    }
  }

  const createKey = async () => {
    if (!newKeyName.trim()) return

    setCreating(true)
    setError('')
    
    try {
      const res = await fetch('/api/proxy/keys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: newKeyName })
      })

      const data = await res.json()
      
      if (res.ok) {
        setNewlyCreatedKey(data.key.apiKey)
        setApiKeys([data.key, ...apiKeys])
        setNewKeyName('')
        setShowCreateForm(false)
      } else {
        setError(data.error?.message || data.error || 'Failed to create API key')
      }
    } catch (err) {
      setError('Failed to create API key')
    } finally {
      setCreating(false)
    }
  }

  const deleteKey = async (id: string) => {
    if (!confirm('Are you sure you want to delete this API key? This cannot be undone.')) return

    try {
      const res = await fetch(`/api/proxy/keys/${id}`, { method: 'DELETE' })
      if (res.ok) {
        setApiKeys(apiKeys.filter(key => key.id !== id))
      } else {
        const data = await res.json()
        setError(data.error || 'Failed to delete API key')
      }
    } catch (err) {
      setError('Failed to delete API key')
    }
  }

  const toggleKeyStatus = async (id: string, isActive: boolean) => {
    try {
      const res = await fetch(`/api/proxy/keys/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ isActive: !isActive })
      })

      if (res.ok) {
        setApiKeys(apiKeys.map(key => 
          key.id === id ? { ...key, isActive: !isActive } : key
        ))
      } else {
        const data = await res.json()
        setError(data.error || 'Failed to update API key')
      }
    } catch (err) {
      setError('Failed to update API key')
    }
  }

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text)
    setCopiedKey(id)
    setTimeout(() => setCopiedKey(''), 2000)
  }

  const formatDate = (dateStr: string | null) => {
    if (!dateStr) return 'Never'
    return new Date(dateStr).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    })
  }

  if (loading) {
    return (
      <Layout>
        <div className="flex items-center justify-center h-64">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </Layout>
    )
  }

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <div>
            <h1 className="text-3xl font-bold flex items-center gap-2">
              <Key className="h-8 w-8" />
              API Keys
            </h1>
            <p className="text-muted-foreground mt-1">
              Manage your API keys for proxy access
            </p>
          </div>
          <Button onClick={() => setShowCreateForm(true)} disabled={apiKeys.length >= 5}>
            <Plus className="h-4 w-4 mr-2" />
            Create API Key
          </Button>
        </div>

        {error && (
          <Alert variant="destructive">
            <AlertCircle className="h-4 w-4" />
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        {newlyCreatedKey && (
          <Alert className="bg-green-500/10 border-green-500">
            <Check className="h-4 w-4 text-green-500" />
            <AlertDescription className="flex items-center justify-between">
              <div>
                <strong>API Key created!</strong> Copy it now - it won't be shown again:
                <code className="ml-2 px-2 py-1 bg-muted rounded text-sm">
                  {newlyCreatedKey}
                </code>
              </div>
              <Button
                size="sm"
                variant="outline"
                onClick={() => {
                  copyToClipboard(newlyCreatedKey, 'new')
                  setTimeout(() => setNewlyCreatedKey(null), 2000)
                }}
              >
                {copiedKey === 'new' ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
              </Button>
            </AlertDescription>
          </Alert>
        )}

        {showCreateForm && (
          <Card>
            <CardHeader>
              <CardTitle>Create New API Key</CardTitle>
              <CardDescription>
                Give your API key a descriptive name
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex gap-4">
                <Input
                  placeholder="Key name (e.g., Production, Mobile App)"
                  value={newKeyName}
                  onChange={(e) => setNewKeyName(e.target.value)}
                  disabled={creating}
                />
                <Button onClick={createKey} disabled={creating || !newKeyName.trim()}>
                  {creating ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
                  Create
                </Button>
                <Button variant="outline" onClick={() => setShowCreateForm(false)} disabled={creating}>
                  Cancel
                </Button>
              </div>
            </CardContent>
          </Card>
        )}

        {apiKeys.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center">
              <Key className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
              <h3 className="text-lg font-semibold mb-2">No API Keys</h3>
              <p className="text-muted-foreground mb-4">
                Create your first API key to start using the proxy
              </p>
              <Button onClick={() => setShowCreateForm(true)}>
                <Plus className="h-4 w-4 mr-2" />
                Create Your First Key
              </Button>
            </CardContent>
          </Card>
        ) : (
          <div className="grid gap-4">
            {apiKeys.map((key) => (
              <Card key={key.id}>
                <CardContent className="p-6">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-4">
                      <div className="p-3 bg-primary/10 rounded-lg">
                        <Key className="h-6 w-6 text-primary" />
                      </div>
                      <div>
                        <h3 className="font-semibold">{key.name}</h3>
                        <div className="flex items-center gap-2 mt-1">
                          <code className="text-sm text-muted-foreground bg-muted px-2 py-1 rounded">
                            {key.keyPrefix}
                          </code>
                          <Badge variant={key.isActive ? "default" : "secondary"}>
                            {key.isActive ? 'Active' : 'Inactive'}
                          </Badge>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <div className="text-right text-sm text-muted-foreground mr-4">
                        <div className="flex items-center gap-1">
                          <Calendar className="h-3 w-3" />
                          Created: {formatDate(key.createdAt)}
                        </div>
                        <div>Last used: {formatDate(key.lastUsedAt)}</div>
                      </div>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => toggleKeyStatus(key.id, key.isActive)}
                      >
                        {key.isActive ? 'Disable' : 'Enable'}
                      </Button>
                      <Button
                        size="sm"
                        variant="destructive"
                        onClick={() => deleteKey(key.id)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Shield className="h-5 w-5" />
              API Key Security
            </CardTitle>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground space-y-2">
            <p>• API keys are only shown once when created. Store them securely.</p>
            <p>• You can have up to 5 active API keys at a time.</p>
            <p>• Disable or delete keys that are no longer in use.</p>
            <p>• Never share your API keys in public repositories or client-side code.</p>
          </CardContent>
        </Card>
      </div>
    </Layout>
  )
}
