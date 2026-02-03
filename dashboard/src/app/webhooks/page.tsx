'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, 
  DialogDescription, DialogFooter
} from '@/components/ui/dialog';
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow
} from '@/components/ui/table';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Webhook, Plus, Trash2, Copy, CheckCircle, XCircle,
  Clock, RefreshCw, Eye, EyeOff, Send, Code
} from 'lucide-react';

interface WebhookConfig {
  id: string;
  url: string;
  secret: string;
  events: string[];
  active: boolean;
  created_at: string;
  last_triggered?: string;
  success_rate?: number;
}

interface DeliveryLog {
  id: string;
  event_id: string;
  event_type: string;
  status_code: number;
  duration_ms: number;
  error?: string;
  created_at: string;
}

const AVAILABLE_EVENTS = [
  { id: 'node.online', name: 'Node Online', description: 'When a node comes online' },
  { id: 'node.offline', name: 'Node Offline', description: 'When a node goes offline' },
  { id: 'request.completed', name: 'Request Completed', description: 'When a proxy request completes' },
  { id: 'request.failed', name: 'Request Failed', description: 'When a proxy request fails' },
  { id: 'quota.warning', name: 'Quota Warning', description: 'When usage reaches 80%' },
  { id: 'quota.exceeded', name: 'Quota Exceeded', description: 'When usage limit is reached' },
  { id: 'api_key.created', name: 'API Key Created', description: 'When a new API key is created' },
  { id: 'api_key.deleted', name: 'API Key Deleted', description: 'When an API key is deleted' },
  { id: 'payment.success', name: 'Payment Success', description: 'When a payment succeeds' },
  { id: 'payment.failed', name: 'Payment Failed', description: 'When a payment fails' },
];

export default function WebhooksPage() {
  const [webhooks, setWebhooks] = useState<WebhookConfig[]>([]);
  const [deliveryLogs, setDeliveryLogs] = useState<DeliveryLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [showSecretFor, setShowSecretFor] = useState<string | null>(null);
  const [testingWebhook, setTestingWebhook] = useState<string | null>(null);

  useEffect(() => {
    fetchWebhooks();
    fetchDeliveryLogs();
  }, []);

  const fetchWebhooks = async () => {
    try {
      const res = await fetch('/api/webhooks');
      const data = await res.json();
      setWebhooks(data.webhooks || []);
    } catch (error) {
      console.error('Failed to fetch webhooks:', error);
    } finally {
      setLoading(false);
    }
  };

  const fetchDeliveryLogs = async () => {
    try {
      const res = await fetch('/api/webhooks/logs');
      const data = await res.json();
      setDeliveryLogs(data.logs || []);
    } catch (error) {
      console.error('Failed to fetch delivery logs:', error);
    }
  };

  const handleToggle = async (webhookId: string, active: boolean) => {
    try {
      await fetch(`/api/webhooks/${webhookId}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ active })
      });
      fetchWebhooks();
    } catch (error) {
      console.error('Failed to toggle webhook:', error);
    }
  };

  const handleDelete = async (webhookId: string) => {
    if (!confirm('Are you sure you want to delete this webhook?')) return;
    
    try {
      await fetch(`/api/webhooks/${webhookId}`, { method: 'DELETE' });
      fetchWebhooks();
    } catch (error) {
      console.error('Failed to delete webhook:', error);
    }
  };

  const handleTest = async (webhookId: string) => {
    setTestingWebhook(webhookId);
    try {
      await fetch(`/api/webhooks/${webhookId}/test`, { method: 'POST' });
      fetchDeliveryLogs();
    } catch (error) {
      console.error('Failed to test webhook:', error);
    } finally {
      setTestingWebhook(null);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  const formatDate = (date: string) => {
    return new Date(date).toLocaleString();
  };

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Webhook className="w-6 h-6" />
            Webhooks
          </h1>
          <p className="text-muted-foreground">
            Receive real-time notifications for events
          </p>
        </div>
        <Button onClick={() => setShowCreateDialog(true)}>
          <Plus className="w-4 h-4 mr-2" />
          Add Webhook
        </Button>
      </div>

      <Tabs defaultValue="endpoints" className="space-y-6">
        <TabsList>
          <TabsTrigger value="endpoints">Endpoints</TabsTrigger>
          <TabsTrigger value="logs">Delivery Logs</TabsTrigger>
          <TabsTrigger value="docs">Documentation</TabsTrigger>
        </TabsList>

        {/* Endpoints Tab */}
        <TabsContent value="endpoints" className="space-y-4">
          {loading ? (
            <Card>
              <CardContent className="py-8 text-center text-muted-foreground">
                Loading webhooks...
              </CardContent>
            </Card>
          ) : webhooks.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center">
                <Webhook className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
                <h3 className="text-lg font-medium mb-2">No webhooks configured</h3>
                <p className="text-muted-foreground mb-4">
                  Create a webhook to receive real-time event notifications
                </p>
                <Button onClick={() => setShowCreateDialog(true)}>
                  <Plus className="w-4 h-4 mr-2" />
                  Create Webhook
                </Button>
              </CardContent>
            </Card>
          ) : (
            webhooks.map((webhook) => (
              <Card key={webhook.id}>
                <CardHeader className="pb-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-4">
                      <div className={`w-3 h-3 rounded-full ${webhook.active ? 'bg-green-500' : 'bg-gray-400'}`} />
                      <div>
                        <CardTitle className="text-base font-mono break-all">
                          {webhook.url}
                        </CardTitle>
                        <CardDescription>
                          Created {formatDate(webhook.created_at)}
                        </CardDescription>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Switch
                        checked={webhook.active}
                        onCheckedChange={(checked) => handleToggle(webhook.id, checked)}
                      />
                      <Button 
                        variant="outline" 
                        size="sm"
                        onClick={() => handleTest(webhook.id)}
                        disabled={testingWebhook === webhook.id}
                      >
                        {testingWebhook === webhook.id ? (
                          <RefreshCw className="w-4 h-4 animate-spin" />
                        ) : (
                          <Send className="w-4 h-4" />
                        )}
                      </Button>
                      <Button 
                        variant="ghost" 
                        size="sm"
                        onClick={() => handleDelete(webhook.id)}
                      >
                        <Trash2 className="w-4 h-4 text-red-500" />
                      </Button>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  {/* Secret */}
                  <div>
                    <Label className="text-xs text-muted-foreground">Signing Secret</Label>
                    <div className="flex items-center gap-2 mt-1">
                      <code className="flex-1 px-3 py-2 bg-muted rounded text-sm font-mono">
                        {showSecretFor === webhook.id 
                          ? webhook.secret 
                          : '••••••••••••••••••••••••••••••••'}
                      </code>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setShowSecretFor(
                          showSecretFor === webhook.id ? null : webhook.id
                        )}
                      >
                        {showSecretFor === webhook.id ? (
                          <EyeOff className="w-4 h-4" />
                        ) : (
                          <Eye className="w-4 h-4" />
                        )}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => copyToClipboard(webhook.secret)}
                      >
                        <Copy className="w-4 h-4" />
                      </Button>
                    </div>
                  </div>

                  {/* Events */}
                  <div>
                    <Label className="text-xs text-muted-foreground">Subscribed Events</Label>
                    <div className="flex flex-wrap gap-2 mt-2">
                      {webhook.events.map((event) => (
                        <Badge key={event} variant="secondary">
                          {event}
                        </Badge>
                      ))}
                    </div>
                  </div>

                  {/* Stats */}
                  {webhook.success_rate !== undefined && (
                    <div className="flex items-center gap-4 text-sm text-muted-foreground">
                      <span>Success rate: {webhook.success_rate}%</span>
                      {webhook.last_triggered && (
                        <span>Last triggered: {formatDate(webhook.last_triggered)}</span>
                      )}
                    </div>
                  )}
                </CardContent>
              </Card>
            ))
          )}
        </TabsContent>

        {/* Delivery Logs Tab */}
        <TabsContent value="logs">
          <Card>
            <CardHeader>
              <CardTitle>Recent Deliveries</CardTitle>
              <CardDescription>Last 50 webhook delivery attempts</CardDescription>
            </CardHeader>
            <CardContent>
              {deliveryLogs.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  No delivery logs yet
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Event</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Duration</TableHead>
                      <TableHead>Time</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {deliveryLogs.map((log) => (
                      <TableRow key={log.id}>
                        <TableCell>
                          <Badge variant="outline">{log.event_type}</Badge>
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            {log.status_code >= 200 && log.status_code < 300 ? (
                              <CheckCircle className="w-4 h-4 text-green-500" />
                            ) : (
                              <XCircle className="w-4 h-4 text-red-500" />
                            )}
                            <span>{log.status_code || 'Failed'}</span>
                          </div>
                          {log.error && (
                            <div className="text-xs text-red-500 mt-1">
                              {log.error}
                            </div>
                          )}
                        </TableCell>
                        <TableCell>{log.duration_ms}ms</TableCell>
                        <TableCell className="text-muted-foreground">
                          {formatDate(log.created_at)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* Documentation Tab */}
        <TabsContent value="docs">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Code className="w-5 h-5" />
                Webhook Integration
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <div>
                <h3 className="font-medium mb-2">Verifying Signatures</h3>
                <p className="text-sm text-muted-foreground mb-4">
                  Each webhook request includes a signature header for verification.
                </p>
                <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`// Node.js example
const crypto = require('crypto');

function verifyWebhook(payload, signature, secret) {
  const expected = crypto
    .createHmac('sha256', secret)
    .update(payload)
    .digest('hex');
  
  return crypto.timingSafeEqual(
    Buffer.from(signature),
    Buffer.from(expected)
  );
}

// In your webhook handler:
app.post('/webhook', (req, res) => {
  const signature = req.headers['x-signature-256'].replace('sha256=', '');
  const isValid = verifyWebhook(
    JSON.stringify(req.body),
    signature,
    process.env.WEBHOOK_SECRET
  );
  
  if (!isValid) {
    return res.status(401).send('Invalid signature');
  }
  
  // Process the webhook
  console.log('Event:', req.body.type);
  console.log('Data:', req.body.data);
  
  res.status(200).send('OK');
});`}
                </pre>
              </div>

              <div>
                <h3 className="font-medium mb-2">Request Headers</h3>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Header</TableHead>
                      <TableHead>Description</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow>
                      <TableCell className="font-mono">X-Webhook-ID</TableCell>
                      <TableCell>Unique ID of the webhook endpoint</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell className="font-mono">X-Event-ID</TableCell>
                      <TableCell>Unique ID of this event</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell className="font-mono">X-Event-Type</TableCell>
                      <TableCell>Type of event (e.g., node.online)</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell className="font-mono">X-Signature-256</TableCell>
                      <TableCell>HMAC-SHA256 signature of the payload</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell className="font-mono">X-Timestamp</TableCell>
                      <TableCell>ISO 8601 timestamp of the event</TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </div>

              <div>
                <h3 className="font-medium mb-2">Event Payload</h3>
                <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`{
  "id": "evt_abc123",
  "type": "request.completed",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": {
    "request_id": "req_xyz789",
    "node_id": "node_123",
    "country": "US",
    "duration_ms": 150,
    "bytes_transferred": 1024
  }
}`}
                </pre>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Create Webhook Dialog */}
      <CreateWebhookDialog
        open={showCreateDialog}
        onClose={() => setShowCreateDialog(false)}
        onCreated={fetchWebhooks}
      />
    </div>
  );
}

function CreateWebhookDialog({
  open,
  onClose,
  onCreated
}: {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [url, setUrl] = useState('');
  const [selectedEvents, setSelectedEvents] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    
    try {
      await fetch('/api/webhooks', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          url,
          events: selectedEvents.length > 0 ? selectedEvents : ['*']
        })
      });
      onCreated();
      onClose();
      setUrl('');
      setSelectedEvents([]);
    } catch (error) {
      console.error('Failed to create webhook:', error);
    } finally {
      setLoading(false);
    }
  };

  const toggleEvent = (eventId: string) => {
    setSelectedEvents(prev =>
      prev.includes(eventId)
        ? prev.filter(e => e !== eventId)
        : [...prev, eventId]
    );
  };

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Create Webhook</DialogTitle>
          <DialogDescription>
            Configure a new webhook endpoint to receive event notifications
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <Label>Endpoint URL</Label>
            <Input
              type="url"
              placeholder="https://your-server.com/webhook"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              required
            />
          </div>

          <div>
            <Label className="mb-3 block">Events to Subscribe</Label>
            <div className="space-y-2 max-h-64 overflow-y-auto">
              {AVAILABLE_EVENTS.map((event) => (
                <div
                  key={event.id}
                  className="flex items-start gap-3 p-2 rounded hover:bg-muted cursor-pointer"
                  onClick={() => toggleEvent(event.id)}
                >
                  <Checkbox
                    checked={selectedEvents.includes(event.id)}
                    onCheckedChange={() => toggleEvent(event.id)}
                  />
                  <div>
                    <div className="font-medium text-sm">{event.name}</div>
                    <div className="text-xs text-muted-foreground">
                      {event.description}
                    </div>
                  </div>
                </div>
              ))}
            </div>
            <p className="text-xs text-muted-foreground mt-2">
              Leave empty to subscribe to all events
            </p>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={loading || !url}>
              {loading ? 'Creating...' : 'Create Webhook'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
