'use client'

import { Layout } from '@/components/layout'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { BarChart3, Globe, Zap, Activity, TrendingUp, Users, Server, Clock } from 'lucide-react'
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, ResponsiveContainer, BarChart, Bar, PieChart, Pie, Cell } from 'recharts'

const statsData = [
  {
    title: "Total Requests",
    value: "2.4M",
    change: "+12.5%",
    changeType: "increase" as const,
    icon: BarChart3,
    description: "Last 30 days"
  },
  {
    title: "Bandwidth Used",
    value: "1.2TB",
    change: "+8.2%",
    changeType: "increase" as const,
    icon: Globe,
    description: "Current month"
  },
  {
    title: "Active Sessions",
    value: "1,453",
    change: "+3.1%",
    changeType: "increase" as const,
    icon: Users,
    description: "Live connections"
  },
  {
    title: "Success Rate",
    value: "99.8%",
    change: "+0.1%",
    changeType: "increase" as const,
    icon: TrendingUp,
    description: "Last 24 hours"
  }
]

const usageData = [
  { name: 'Jan', requests: 1200, bandwidth: 450 },
  { name: 'Feb', requests: 1900, bandwidth: 680 },
  { name: 'Mar', requests: 3000, bandwidth: 920 },
  { name: 'Apr', requests: 2800, bandwidth: 1100 },
  { name: 'May', requests: 3200, bandwidth: 1300 },
  { name: 'Jun', requests: 4100, bandwidth: 1600 },
  { name: 'Jul', requests: 4500, bandwidth: 1800 },
  { name: 'Aug', requests: 3900, bandwidth: 1550 },
  { name: 'Sep', requests: 4200, bandwidth: 1700 },
  { name: 'Oct', requests: 4800, bandwidth: 1900 },
  { name: 'Nov', requests: 5100, bandwidth: 2100 },
  { name: 'Dec', requests: 4600, bandwidth: 1950 }
]

const regionData = [
  { name: 'North America', value: 35, color: '#3b82f6' },
  { name: 'Europe', value: 28, color: '#10b981' },
  { name: 'Asia Pacific', value: 22, color: '#f59e0b' },
  { name: 'South America', value: 10, color: '#ef4444' },
  { name: 'Others', value: 5, color: '#8b5cf6' }
]

const recentActivity = [
  { action: 'API Key Generated', time: '2 minutes ago', status: 'success' },
  { action: 'High Usage Alert', time: '15 minutes ago', status: 'warning' },
  { action: 'New Session Started', time: '23 minutes ago', status: 'info' },
  { action: 'Payment Processed', time: '1 hour ago', status: 'success' },
  { action: 'Endpoint Added', time: '2 hours ago', status: 'info' }
]

export default function DashboardPage() {
  return (
    <Layout>
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold">Dashboard Overview</h1>
          <p className="text-muted-foreground">Welcome back! Here's what's happening with your proxy infrastructure.</p>
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
                <div className="text-2xl font-bold">{stat.value}</div>
                <p className="text-xs text-muted-foreground flex items-center gap-1">
                  <span className={`text-${stat.changeType === 'increase' ? 'green' : 'red'}-500`}>
                    {stat.change}
                  </span>
                  from {stat.description}
                </p>
              </CardContent>
            </Card>
          ))}
        </div>

        <div className="grid gap-4 lg:grid-cols-3">
          {/* Usage Chart */}
          <Card className="lg:col-span-2">
            <CardHeader>
              <CardTitle>Usage Overview</CardTitle>
              <CardDescription>Monthly requests and bandwidth consumption</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="h-[300px]">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={usageData}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                    <XAxis dataKey="name" className="text-muted-foreground" />
                    <YAxis className="text-muted-foreground" />
                    <Area type="monotone" dataKey="requests" stackId="1" stroke="#3b82f6" fill="#3b82f6" fillOpacity={0.6} />
                    <Area type="monotone" dataKey="bandwidth" stackId="2" stroke="#10b981" fill="#10b981" fillOpacity={0.6} />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>

          {/* Regional Distribution */}
          <Card>
            <CardHeader>
              <CardTitle>Traffic by Region</CardTitle>
              <CardDescription>Geographic distribution of requests</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="h-[200px]">
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie
                      data={regionData}
                      cx="50%"
                      cy="50%"
                      innerRadius={40}
                      outerRadius={80}
                      paddingAngle={5}
                      dataKey="value"
                    >
                      {regionData.map((entry, index) => (
                        <Cell key={`cell-${index}`} fill={entry.color} />
                      ))}
                    </Pie>
                  </PieChart>
                </ResponsiveContainer>
              </div>
              <div className="mt-4 space-y-2">
                {regionData.map((region) => (
                  <div key={region.name} className="flex items-center justify-between text-sm">
                    <div className="flex items-center gap-2">
                      <div className="w-3 h-3 rounded-full" style={{ backgroundColor: region.color }} />
                      <span>{region.name}</span>
                    </div>
                    <span className="font-medium">{region.value}%</span>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </div>

        <div className="grid gap-4 lg:grid-cols-2">
          {/* Recent Activity */}
          <Card>
            <CardHeader>
              <CardTitle>Recent Activity</CardTitle>
              <CardDescription>Latest events and notifications</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {recentActivity.map((activity, index) => (
                  <div key={index} className="flex items-center gap-4">
                    <div className={`w-2 h-2 rounded-full ${
                      activity.status === 'success' ? 'bg-green-500' :
                      activity.status === 'warning' ? 'bg-yellow-500' :
                      'bg-blue-500'
                    }`} />
                    <div className="flex-1">
                      <p className="text-sm font-medium">{activity.action}</p>
                      <p className="text-xs text-muted-foreground">{activity.time}</p>
                    </div>
                    <Badge variant={
                      activity.status === 'success' ? 'success' :
                      activity.status === 'warning' ? 'warning' :
                      'secondary'
                    }>
                      {activity.status}
                    </Badge>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>

          {/* Quick Stats */}
          <Card>
            <CardHeader>
              <CardTitle>System Health</CardTitle>
              <CardDescription>Current system status and metrics</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Server className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm">Server Uptime</span>
                </div>
                <Badge variant="success">99.9%</Badge>
              </div>
              
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Activity className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm">Response Time</span>
                </div>
                <Badge variant="secondary">45ms</Badge>
              </div>
              
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Clock className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm">Queue Status</span>
                </div>
                <Badge variant="success">Healthy</Badge>
              </div>
              
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Zap className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm">Rate Limiting</span>
                </div>
                <Badge variant="secondary">Active</Badge>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </Layout>
  )
}