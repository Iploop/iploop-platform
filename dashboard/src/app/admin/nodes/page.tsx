'use client';

import { useState, useEffect } from 'react';
import { 
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow 
} from '@/components/ui/table';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue
} from '@/components/ui/select';
import { 
  Search, RefreshCw, Globe, Smartphone, Monitor, 
  Wifi, Signal, Ban, CheckCircle, AlertTriangle,
  TrendingUp, Users, Database, Zap
} from 'lucide-react';

interface Node {
  id: string;
  device_id: string;
  ip_address: string;
  country: string;
  country_name: string;
  city: string;
  isp: string;
  connection_type: string;
  device_type: string;
  sdk_version: string;
  status: 'available' | 'busy' | 'offline' | 'blacklisted';
  quality_score: number;
  bandwidth_used_mb: number;
  requests_handled: number;
  last_heartbeat: string;
  connected_since: string;
  earnings: number;
}

interface NodeStats {
  total: number;
  available: number;
  busy: number;
  offline: number;
  blacklisted: number;
  countries: Record<string, number>;
  connection_types: Record<string, number>;
}

export default function AdminNodesPage() {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [stats, setStats] = useState<NodeStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [countryFilter, setCountryFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('all');
  const [autoRefresh, setAutoRefresh] = useState(true);

  useEffect(() => {
    fetchNodes();
    fetchStats();

    if (autoRefresh) {
      const interval = setInterval(() => {
        fetchNodes();
        fetchStats();
      }, 10000); // Refresh every 10 seconds
      return () => clearInterval(interval);
    }
  }, [autoRefresh]);

  const fetchNodes = async () => {
    try {
      const res = await fetch('/api/admin/nodes');
      const data = await res.json();
      setNodes(data.nodes || []);
    } catch (error) {
      console.error('Failed to fetch nodes:', error);
    } finally {
      setLoading(false);
    }
  };

  const fetchStats = async () => {
    try {
      const res = await fetch('/api/admin/nodes/stats');
      const data = await res.json();
      setStats(data);
    } catch (error) {
      console.error('Failed to fetch stats:', error);
    }
  };

  const handleBlacklistNode = async (nodeId: string) => {
    try {
      await fetch(`/api/admin/nodes/${nodeId}/blacklist`, { method: 'POST' });
      fetchNodes();
    } catch (error) {
      console.error('Failed to blacklist node:', error);
    }
  };

  const handleUnblacklistNode = async (nodeId: string) => {
    try {
      await fetch(`/api/admin/nodes/${nodeId}/unblacklist`, { method: 'POST' });
      fetchNodes();
    } catch (error) {
      console.error('Failed to unblacklist node:', error);
    }
  };

  const filteredNodes = nodes.filter(node => {
    const matchesSearch = 
      node.id.toLowerCase().includes(searchQuery.toLowerCase()) ||
      node.ip_address.includes(searchQuery) ||
      node.country_name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
      node.city?.toLowerCase().includes(searchQuery.toLowerCase());
    
    const matchesCountry = countryFilter === 'all' || node.country === countryFilter;
    const matchesStatus = statusFilter === 'all' || node.status === statusFilter;
    
    return matchesSearch && matchesCountry && matchesStatus;
  });

  const getStatusBadge = (status: string) => {
    const config: Record<string, { variant: 'default' | 'secondary' | 'destructive' | 'outline'; icon: any }> = {
      available: { variant: 'default', icon: CheckCircle },
      busy: { variant: 'secondary', icon: Zap },
      offline: { variant: 'outline', icon: AlertTriangle },
      blacklisted: { variant: 'destructive', icon: Ban }
    };
    const { variant, icon: Icon } = config[status] || config.offline;
    return (
      <Badge variant={variant} className="flex items-center gap-1">
        <Icon className="w-3 h-3" />
        {status}
      </Badge>
    );
  };

  const getQualityColor = (score: number) => {
    if (score >= 80) return 'text-green-500';
    if (score >= 50) return 'text-yellow-500';
    return 'text-red-500';
  };

  const getDeviceIcon = (type: string) => {
    if (type === 'mobile' || type === 'android' || type === 'ios') {
      return <Smartphone className="w-4 h-4" />;
    }
    return <Monitor className="w-4 h-4" />;
  };

  const getConnectionIcon = (type: string) => {
    if (type === 'wifi') return <Wifi className="w-4 h-4" />;
    return <Signal className="w-4 h-4" />;
  };

  const formatTimeAgo = (date: string) => {
    const seconds = Math.floor((Date.now() - new Date(date).getTime()) / 1000);
    if (seconds < 60) return `${seconds}s ago`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
    if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
    return `${Math.floor(seconds / 86400)}d ago`;
  };

  const uniqueCountries = [...new Set(nodes.map(n => n.country))].sort();

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Node Management</h1>
          <p className="text-muted-foreground">
            Monitor and manage proxy nodes in the network
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant={autoRefresh ? 'default' : 'outline'}
            size="sm"
            onClick={() => setAutoRefresh(!autoRefresh)}
          >
            <RefreshCw className={`w-4 h-4 mr-2 ${autoRefresh ? 'animate-spin' : ''}`} />
            {autoRefresh ? 'Auto-refresh ON' : 'Auto-refresh OFF'}
          </Button>
          <Button size="sm" onClick={() => { fetchNodes(); fetchStats(); }}>
            <RefreshCw className="w-4 h-4 mr-2" />
            Refresh
          </Button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-5 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Total Nodes
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats?.total || 0}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <CheckCircle className="w-4 h-4 text-green-500" />
              Available
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-500">
              {stats?.available || 0}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <Zap className="w-4 h-4 text-blue-500" />
              Busy
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-500">
              {stats?.busy || 0}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <AlertTriangle className="w-4 h-4 text-yellow-500" />
              Offline
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-yellow-500">
              {stats?.offline || 0}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <Globe className="w-4 h-4" />
              Countries
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {Object.keys(stats?.countries || {}).length}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Country Distribution */}
      {stats?.countries && Object.keys(stats.countries).length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium">
              Geographic Distribution
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-2">
              {Object.entries(stats.countries)
                .sort(([,a], [,b]) => b - a)
                .slice(0, 20)
                .map(([country, count]) => (
                  <Badge key={country} variant="outline" className="text-xs">
                    {country}: {count}
                  </Badge>
                ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Filters */}
      <div className="flex items-center gap-4">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input
            placeholder="Search by ID, IP, country, city..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-10"
          />
        </div>
        <Select value={countryFilter} onValueChange={setCountryFilter}>
          <SelectTrigger className="w-40">
            <SelectValue placeholder="Country" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Countries</SelectItem>
            {uniqueCountries.map(country => (
              <SelectItem key={country} value={country}>{country}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className="w-40">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Status</SelectItem>
            <SelectItem value="available">Available</SelectItem>
            <SelectItem value="busy">Busy</SelectItem>
            <SelectItem value="offline">Offline</SelectItem>
            <SelectItem value="blacklisted">Blacklisted</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Nodes Table */}
      <div className="border rounded-lg">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Node</TableHead>
              <TableHead>Location</TableHead>
              <TableHead>Connection</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Quality</TableHead>
              <TableHead>Usage</TableHead>
              <TableHead>Last Seen</TableHead>
              <TableHead className="w-[100px]">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={8} className="text-center py-8">
                  Loading nodes...
                </TableCell>
              </TableRow>
            ) : filteredNodes.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="text-center py-8">
                  No nodes found
                </TableCell>
              </TableRow>
            ) : (
              filteredNodes.map((node) => (
                <TableRow key={node.id}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      {getDeviceIcon(node.device_type)}
                      <div>
                        <div className="font-mono text-xs">{node.id.slice(0, 8)}...</div>
                        <div className="text-xs text-muted-foreground">{node.ip_address}</div>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <Globe className="w-4 h-4" />
                      <div>
                        <div>{node.country_name || node.country}</div>
                        <div className="text-xs text-muted-foreground">{node.city}</div>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      {getConnectionIcon(node.connection_type)}
                      <div>
                        <div className="capitalize">{node.connection_type}</div>
                        <div className="text-xs text-muted-foreground">{node.isp}</div>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>{getStatusBadge(node.status)}</TableCell>
                  <TableCell>
                    <div className={`font-bold ${getQualityColor(node.quality_score)}`}>
                      {node.quality_score}%
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="text-sm">
                      <div>{(node.bandwidth_used_mb / 1024).toFixed(2)} GB</div>
                      <div className="text-xs text-muted-foreground">
                        {node.requests_handled.toLocaleString()} req
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="text-sm text-muted-foreground">
                      {formatTimeAgo(node.last_heartbeat)}
                    </div>
                  </TableCell>
                  <TableCell>
                    {node.status === 'blacklisted' ? (
                      <Button 
                        size="sm" 
                        variant="outline"
                        onClick={() => handleUnblacklistNode(node.id)}
                      >
                        Unblock
                      </Button>
                    ) : (
                      <Button 
                        size="sm" 
                        variant="destructive"
                        onClick={() => handleBlacklistNode(node.id)}
                      >
                        <Ban className="w-4 h-4" />
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
