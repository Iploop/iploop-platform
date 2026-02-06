'use client'

import { useState } from 'react'
import { Layout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { BarChart3, Download, Filter, Calendar } from 'lucide-react'
import {
  AreaChart,
  Area,
  BarChart,
  Bar,
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  Tooltip
} from 'recharts'

const timeRanges = ['7 days', '30 days', '3 months', '1 year']

const dailyUsageData = [
  { date: '2024-01-27', requests: 45000, bandwidth: 15.2, success: 99.8, errors: 90 },
  { date: '2024-01-28', requests: 52000, bandwidth: 17.8, success: 99.9, errors: 52 },
  { date: '2024-01-29', requests: 48000, bandwidth: 16.1, success: 99.7, errors: 144 },
  { date: '2024-01-30', requests: 61000, bandwidth: 20.5, success: 99.9, errors: 61 },
  { date: '2024-01-31', requests: 58000, bandwidth: 19.2, success: 99.8, errors: 116 },
  { date: '2024-02-01', requests: 67000, bandwidth: 22.1, success: 99.9, errors: 67 },
  { date: '2024-02-02', requests: 71000, bandwidth: 23.8, success: 99.8, errors: 142 },
  { date: '2024-02-03', requests: 69000, bandwidth: 22.9, success: 99.9, errors: 69 }
]

const hourlyData = [
  { hour: '00:00', requests: 1200 },
  { hour: '01:00', requests: 800 },
  { hour: '02:00', requests: 600 },
  { hour: '03:00', requests: 500 },
  { hour: '04:00', requests: 700 },
  { hour: '05:00', requests: 900 },
  { hour: '06:00', requests: 1400 },
  { hour: '07:00', requests: 1800 },
  { hour: '08:00', requests: 2200 },
  { hour: '09:00', requests: 2800 },
  { hour: '10:00', requests: 3200 },
  { hour: '11:00', requests: 3600 },
  { hour: '12:00', requests: 4000 },
  { hour: '13:00', requests: 4200 },
  { hour: '14:00', requests: 4500 },
  { hour: '15:00', requests: 4300 },
  { hour: '16:00', requests: 3900 },
  { hour: '17:00', requests: 3500 },
  { hour: '18:00', requests: 3000 },
  { hour: '19:00', requests: 2500 },
  { hour: '20:00', requests: 2000 },
  { hour: '21:00', requests: 1800 },
  { hour: '22:00', requests: 1500 },
  { hour: '23:00', requests: 1300 }
]

const statusCodeData = [
  { name: '200 OK', value: 89.2, color: '#10b981' },
  { name: '301 Redirect', value: 7.8, color: '#3b82f6' },
  { name: '404 Not Found', value: 2.1, color: '#f59e0b' },
  { name: '500 Error', value: 0.7, color: '#ef4444' },
  { name: 'Other', value: 0.2, color: '#8b5cf6' }
]

const endpointData = [
  { endpoint: '/api/v1/proxy', requests: 180000, avgResponse: 45, errors: 0.2 },
  { endpoint: '/api/v1/session', requests: 120000, avgResponse: 32, errors: 0.1 },
  { endpoint: '/api/v1/health', requests: 89000, avgResponse: 12, errors: 0.0 },
  { endpoint: '/api/v1/usage', requests: 67000, avgResponse: 28, errors: 0.3 },
  { endpoint: '/api/v1/billing', requests: 45000, avgResponse: 56, errors: 0.1 }
]

export default function UsagePage() {
  const [selectedRange, setSelectedRange] = useState('7 days')
  const [selectedMetric, setSelectedMetric] = useState('requests')

  const formatBandwidth = (value: number) => `${value.toFixed(1)}GB`
  const formatRequests = (value: number) => `${(value / 1000).toFixed(0)}K`

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold">Usage Statistics</h1>
            <p className="text-muted-foreground">Detailed analytics and usage patterns</p>
          </div>
          <div className="flex gap-2">
            <Button variant="outline">
              <Download className="w-4 h-4 mr-2" />
              Export
            </Button>
            <Button variant="outline">
              <Filter className="w-4 h-4 mr-2" />
              Filter
            </Button>
          </div>
        </div>

        {/* Time Range Selector */}
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Calendar className="w-4 h-4" />
                <span className="text-sm font-medium">Time Range:</span>
              </div>
              <div className="flex gap-1">
                {timeRanges.map((range) => (
                  <Button
                    key={range}
                    variant={selectedRange === range ? 'default' : 'outline'}
                    size="sm"
                    onClick={() => setSelectedRange(range)}
                  >
                    {range}
                  </Button>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Main Usage Chart */}
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle>Usage Trends</CardTitle>
                <CardDescription>Daily requests and bandwidth consumption</CardDescription>
              </div>
              <div className="flex gap-1">
                <Button
                  variant={selectedMetric === 'requests' ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setSelectedMetric('requests')}
                >
                  Requests
                </Button>
                <Button
                  variant={selectedMetric === 'bandwidth' ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setSelectedMetric('bandwidth')}
                >
                  Bandwidth
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <div className="h-[400px]">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={dailyUsageData}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis 
                    dataKey="date" 
                    className="text-muted-foreground"
                    tickFormatter={(value) => new Date(value).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                  />
                  <YAxis 
                    className="text-muted-foreground"
                    tickFormatter={selectedMetric === 'requests' ? formatRequests : formatBandwidth}
                  />
                  <Tooltip 
                    formatter={(value) => [
                      selectedMetric === 'requests' ? formatRequests(value as number ?? 0) : formatBandwidth(value as number ?? 0),
                      selectedMetric === 'requests' ? 'Requests' : 'Bandwidth (GB)'
                    ]}
                    labelFormatter={(value) => new Date(value).toLocaleDateString()}
                  />
                  <Area 
                    type="monotone" 
                    dataKey={selectedMetric} 
                    stroke="#3b82f6" 
                    fill="#3b82f6" 
                    fillOpacity={0.6} 
                  />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>

        <div className="grid gap-4 lg:grid-cols-2">
          {/* Hourly Distribution */}
          <Card>
            <CardHeader>
              <CardTitle>Hourly Distribution</CardTitle>
              <CardDescription>Request patterns throughout the day</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="h-[300px]">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={hourlyData}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                    <XAxis dataKey="hour" className="text-muted-foreground" />
                    <YAxis className="text-muted-foreground" tickFormatter={formatRequests} />
                    <Tooltip formatter={(value) => [formatRequests(value as number ?? 0), 'Requests']} />
                    <Bar dataKey="requests" fill="#10b981" />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>

          {/* Status Code Distribution */}
          <Card>
            <CardHeader>
              <CardTitle>Response Status Distribution</CardTitle>
              <CardDescription>HTTP status code breakdown</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="h-[200px]">
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie
                      data={statusCodeData}
                      cx="50%"
                      cy="50%"
                      innerRadius={40}
                      outerRadius={80}
                      paddingAngle={2}
                      dataKey="value"
                    >
                      {statusCodeData.map((entry, index) => (
                        <Cell key={`cell-${index}`} fill={entry.color} />
                      ))}
                    </Pie>
                    <Tooltip formatter={(value) => [`${value}%`, 'Percentage']} />
                  </PieChart>
                </ResponsiveContainer>
              </div>
              <div className="mt-4 space-y-2">
                {statusCodeData.map((status) => (
                  <div key={status.name} className="flex items-center justify-between text-sm">
                    <div className="flex items-center gap-2">
                      <div className="w-3 h-3 rounded-full" style={{ backgroundColor: status.color }} />
                      <span>{status.name}</span>
                    </div>
                    <span className="font-medium">{status.value}%</span>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Success Rate Trend */}
        <Card>
          <CardHeader>
            <CardTitle>Success Rate & Error Tracking</CardTitle>
            <CardDescription>Monitor service reliability and error patterns</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[300px]">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={dailyUsageData}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis 
                    dataKey="date" 
                    className="text-muted-foreground"
                    tickFormatter={(value) => new Date(value).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                  />
                  <YAxis 
                    yAxisId="left"
                    className="text-muted-foreground"
                    domain={[99, 100]}
                    tickFormatter={(value) => `${value.toFixed(1)}%`}
                  />
                  <YAxis 
                    yAxisId="right" 
                    orientation="right" 
                    className="text-muted-foreground"
                  />
                  <Tooltip 
                    formatter={(value, name) => [
                      name === 'success' ? `${(value as number ?? 0).toFixed(2)}%` : value,
                      name === 'success' ? 'Success Rate' : 'Errors'
                    ]}
                    labelFormatter={(value) => new Date(value).toLocaleDateString()}
                  />
                  <Line 
                    yAxisId="left"
                    type="monotone" 
                    dataKey="success" 
                    stroke="#10b981" 
                    strokeWidth={2}
                    dot={{ r: 4 }}
                  />
                  <Line 
                    yAxisId="right"
                    type="monotone" 
                    dataKey="errors" 
                    stroke="#ef4444" 
                    strokeWidth={2}
                    dot={{ r: 4 }}
                  />
                </LineChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>

        {/* Top Endpoints */}
        <Card>
          <CardHeader>
            <CardTitle>Top Endpoints</CardTitle>
            <CardDescription>Most frequently used API endpoints</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {endpointData.map((endpoint, index) => (
                <div key={endpoint.endpoint} className="flex items-center justify-between p-4 border border-border rounded-lg">
                  <div className="flex items-center gap-4">
                    <div className="text-2xl font-bold text-muted-foreground">#{index + 1}</div>
                    <div>
                      <code className="text-sm bg-muted px-2 py-1 rounded">{endpoint.endpoint}</code>
                      <div className="flex items-center gap-4 mt-2 text-sm text-muted-foreground">
                        <span>{endpoint.requests.toLocaleString()} requests</span>
                        <span>{endpoint.avgResponse}ms avg</span>
                        <Badge variant={endpoint.errors < 0.5 ? 'success' : 'warning'}>
                          {endpoint.errors}% errors
                        </Badge>
                      </div>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-lg font-semibold">{formatRequests(endpoint.requests)}</div>
                    <div className="text-sm text-muted-foreground">requests</div>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
    </Layout>
  )
}