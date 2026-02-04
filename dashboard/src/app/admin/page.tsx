'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { 
  Users, DollarSign, Activity, Globe, Settings, 
  Plus, Trash2, Edit, Search, RefreshCw, Server, Wifi, WifiOff, MapPin
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

interface User {
  id: string
  email: string
  firstName: string
  lastName: string
  company: string | null
  status: string
  role: string
  planName: string
  gbBalance: number
  gbUsed: number
  createdAt: string
}

interface NewUser {
  email: string
  password: string
  firstName: string
  lastName: string
  company: string
  role: string
}

interface Plan {
  id: string
  name: string
  monthlyGb: number
  pricePerGb: number
  isActive: boolean
}

interface Node {
  id: string
  deviceId: string
  ipAddress: string
  country: string
  countryName: string
  city: string
  region: string
  isp: string
  connectionType: string
  deviceType: string
  sdkVersion: string
  status: string
  qualityScore: number
  bandwidthUsedMb: number
  totalRequests: number
  lastHeartbeat: string
  connectedSince: string
}

interface NodeStats {
  totalNodes: number
  activeNodes: number
  inactiveNodes: number
  countryBreakdown: Record<string, number>
  deviceTypes: Record<string, number>
  connectionTypes: Record<string, number>
}

export default function AdminPage() {
  const router = useRouter()
  const [activeTab, setActiveTab] = useState<'users' | 'plans' | 'supply' | 'settings'>('users')
  const [users, setUsers] = useState<User[]>([])
  const [plans, setPlans] = useState<Plan[]>([])
  const [nodes, setNodes] = useState<Node[]>([])
  const [nodeStats, setNodeStats] = useState<NodeStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [searchTerm, setSearchTerm] = useState('')
  const [showAddUser, setShowAddUser] = useState(false)
  const [showEditUser, setShowEditUser] = useState<User | null>(null)
  const [newUser, setNewUser] = useState<NewUser>({
    email: '', password: '', firstName: '', lastName: '', company: '', role: 'customer'
  })
  const [editBalance, setEditBalance] = useState('')
  const [addingUser, setAddingUser] = useState(false)
  const [savingUser, setSavingUser] = useState(false)
  const [showAddPlan, setShowAddPlan] = useState(false)
  const [newPlan, setNewPlan] = useState({ name: '', description: '', pricePerGb: '', includedGb: '', maxConnections: '10' })
  const [addingPlan, setAddingPlan] = useState(false)

  const [accessDenied, setAccessDenied] = useState(false)

  // Admin check
  useEffect(() => {
    const token = localStorage.getItem('token')
    const userStr = localStorage.getItem('user')
    
    if (!token) {
      router.push('/')
      return
    }

    // Check user role
    if (userStr) {
      const user = JSON.parse(userStr)
      if (user.role !== 'admin') {
        setAccessDenied(true)
        return
      }
    }
    
    fetchData()
  }, [router])

  const fetchData = async () => {
    setLoading(true)
    try {
      const token = localStorage.getItem('token')
      
      // Fetch users
      const usersRes = await fetch('/api/admin/users', {
        headers: { Authorization: `Bearer ${token}` }
      })
      if (usersRes.ok) {
        const data = await usersRes.json()
        setUsers(data.users || [])
      }

      // Fetch plans
      const plansRes = await fetch('/api/admin/plans', {
        headers: { Authorization: `Bearer ${token}` }
      })
      if (plansRes.ok) {
        const data = await plansRes.json()
        setPlans(data.plans || [])
      }

      // Fetch nodes/supply
      const nodesRes = await fetch('/api/admin/nodes', {
        headers: { Authorization: `Bearer ${token}` }
      })
      if (nodesRes.ok) {
        const data = await nodesRes.json()
        setNodes(data.nodes || [])
        setNodeStats(data.stats || null)
      }
    } catch (err) {
      console.error('Failed to fetch admin data:', err)
    } finally {
      setLoading(false)
    }
  }

  const addUser = async () => {
    if (!newUser.email || !newUser.password || !newUser.firstName || !newUser.lastName) {
      alert('Please fill all required fields')
      return
    }
    
    setAddingUser(true)
    try {
      const token = localStorage.getItem('token')
      const res = await fetch('/api/admin/users/create', {
        method: 'POST',
        headers: { 
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify(newUser)
      })
      
      if (res.ok) {
        setShowAddUser(false)
        setNewUser({ email: '', password: '', firstName: '', lastName: '', company: '', role: 'customer' })
        fetchData()
      } else {
        const data = await res.json()
        alert(data.error || 'Failed to create user')
      }
    } catch (err) {
      alert('Failed to create user')
    } finally {
      setAddingUser(false)
    }
  }

  const updateUser = async (userId: string, data: any) => {
    setSavingUser(true)
    try {
      const token = localStorage.getItem('token')
      const res = await fetch(`/api/admin/users/${userId}`, {
        method: 'PUT',
        headers: { 
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify(data)
      })
      
      if (res.ok) {
        setShowEditUser(null)
        fetchData()
      } else {
        const errData = await res.json()
        alert(errData.error || 'Failed to update user')
      }
    } catch (err) {
      alert('Failed to update user')
    } finally {
      setSavingUser(false)
    }
  }

  const deleteUser = async (userId: string, email: string) => {
    if (!confirm(`Are you sure you want to delete user ${email}?`)) return
    
    try {
      const token = localStorage.getItem('token')
      const res = await fetch(`/api/admin/users/${userId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` }
      })
      
      if (res.ok) {
        fetchData()
      } else {
        const errData = await res.json()
        alert(errData.error || 'Failed to delete user')
      }
    } catch (err) {
      alert('Failed to delete user')
    }
  }

  const addPlan = async () => {
    if (!newPlan.name || !newPlan.pricePerGb) {
      alert('Name and Price per GB are required')
      return
    }
    
    setAddingPlan(true)
    try {
      const token = localStorage.getItem('token')
      const res = await fetch('/api/admin/plans', {
        method: 'POST',
        headers: { 
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({
          name: newPlan.name,
          description: newPlan.description,
          pricePerGb: parseFloat(newPlan.pricePerGb),
          includedGb: parseFloat(newPlan.includedGb) || 0,
          maxConnections: parseInt(newPlan.maxConnections) || 10
        })
      })
      
      if (res.ok) {
        setShowAddPlan(false)
        setNewPlan({ name: '', description: '', pricePerGb: '', includedGb: '', maxConnections: '10' })
        fetchData()
      } else {
        const data = await res.json()
        alert(data.error || 'Failed to create plan')
      }
    } catch (err) {
      alert('Failed to create plan')
    } finally {
      setAddingPlan(false)
    }
  }

  const toggleAdmin = async (userId: string, currentRole: string) => {
    const action = currentRole === 'admin' ? 'remove-admin' : 'make-admin'
    try {
      const token = localStorage.getItem('token')
      const res = await fetch(`/api/admin/users/${userId}/${action}`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` }
      })
      
      if (res.ok) {
        fetchData()
      } else {
        const errData = await res.json()
        alert(errData.error || 'Failed to update role')
      }
    } catch (err) {
      alert('Failed to update role')
    }
  }

  const filteredUsers = users.filter(user => 
    user.email.toLowerCase().includes(searchTerm.toLowerCase()) ||
    user.firstName?.toLowerCase().includes(searchTerm.toLowerCase()) ||
    user.lastName?.toLowerCase().includes(searchTerm.toLowerCase()) ||
    user.company?.toLowerCase().includes(searchTerm.toLowerCase())
  )

  const totalRevenue = users.reduce((sum, u) => sum + (u.gbUsed * 5), 0) // $5/GB estimate
  const totalUsage = users.reduce((sum, u) => sum + u.gbUsed, 0)
  const activeUsers = users.filter(u => u.status === 'active').length

  if (accessDenied) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center">
        <Card className="w-full max-w-md">
          <CardHeader className="text-center">
            <div className="flex justify-center mb-4">
              <div className="p-3 bg-destructive/10 rounded-full">
                <Settings className="h-8 w-8 text-destructive" />
              </div>
            </div>
            <CardTitle className="text-2xl">Access Denied</CardTitle>
            <CardDescription>
              You don't have permission to access the admin panel.
              Contact an administrator if you need access.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Button className="w-full" onClick={() => router.push('/dashboard')}>
              Go to Dashboard
            </Button>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <div className="border-b">
        <div className="container mx-auto px-4 py-4 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <h1 className="text-2xl font-bold">IPLoop Admin</h1>
            <Badge variant="secondary">Internal</Badge>
          </div>
          <Button variant="outline" size="sm" onClick={fetchData}>
            <RefreshCw className="h-4 w-4 mr-2" />
            Refresh
          </Button>
        </div>
      </div>

      <div className="container mx-auto px-4 py-6">
        {/* Stats */}
        <div className="grid gap-4 md:grid-cols-4 mb-6">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium">Total Users</CardTitle>
              <Users className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{users.length}</div>
              <p className="text-xs text-muted-foreground">{activeUsers} active</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium">Total Usage</CardTitle>
              <Activity className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{totalUsage.toFixed(2)} GB</div>
              <p className="text-xs text-muted-foreground">All time</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium">Est. Revenue</CardTitle>
              <DollarSign className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">${totalRevenue.toFixed(0)}</div>
              <p className="text-xs text-muted-foreground">@ $5/GB avg</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium">Plans</CardTitle>
              <Globe className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{plans.length}</div>
              <p className="text-xs text-muted-foreground">Active plans</p>
            </CardContent>
          </Card>
        </div>

        {/* Tabs */}
        <div className="flex gap-2 mb-6">
          <Button 
            variant={activeTab === 'users' ? 'default' : 'outline'}
            onClick={() => setActiveTab('users')}
          >
            <Users className="h-4 w-4 mr-2" />
            Users
          </Button>
          <Button 
            variant={activeTab === 'plans' ? 'default' : 'outline'}
            onClick={() => setActiveTab('plans')}
          >
            <DollarSign className="h-4 w-4 mr-2" />
            Plans
          </Button>
          <Button 
            variant={activeTab === 'supply' ? 'default' : 'outline'}
            onClick={() => setActiveTab('supply')}
          >
            <Server className="h-4 w-4 mr-2" />
            Supply
          </Button>
          <Button 
            variant={activeTab === 'settings' ? 'default' : 'outline'}
            onClick={() => setActiveTab('settings')}
          >
            <Settings className="h-4 w-4 mr-2" />
            Settings
          </Button>
        </div>

        {/* Edit User Modal */}
        {showEditUser && (
          <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <Card className="w-full max-w-md">
              <CardHeader>
                <CardTitle>Edit User</CardTitle>
                <CardDescription>{showEditUser.email}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">GB Balance</label>
                  <Input
                    type="number"
                    step="0.01"
                    value={editBalance}
                    onChange={(e) => setEditBalance(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Status</label>
                  <select
                    className="w-full p-2 border rounded-md bg-background"
                    defaultValue={showEditUser.status}
                    id="edit-status"
                  >
                    <option value="active">Active</option>
                    <option value="suspended">Suspended</option>
                    <option value="inactive">Inactive</option>
                  </select>
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Role</label>
                  <div className="flex items-center gap-2">
                    <Badge variant={showEditUser.role === 'admin' ? 'default' : 'secondary'}>
                      {showEditUser.role || 'customer'}
                    </Badge>
                    <Button 
                      size="sm" 
                      variant="outline"
                      onClick={() => {
                        toggleAdmin(showEditUser.id, showEditUser.role || 'customer')
                        setShowEditUser(null)
                      }}
                    >
                      {showEditUser.role === 'admin' ? 'Remove Admin' : 'Make Admin'}
                    </Button>
                  </div>
                </div>
                <div className="flex gap-2 pt-4">
                  <Button variant="outline" className="flex-1" onClick={() => setShowEditUser(null)}>
                    Cancel
                  </Button>
                  <Button 
                    className="flex-1" 
                    disabled={savingUser}
                    onClick={() => {
                      const status = (document.getElementById('edit-status') as HTMLSelectElement)?.value
                      updateUser(showEditUser.id, { 
                        gbBalance: parseFloat(editBalance),
                        status 
                      })
                    }}
                  >
                    {savingUser ? 'Saving...' : 'Save Changes'}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>
        )}

        {/* Add User Modal */}
        {showAddUser && (
          <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <Card className="w-full max-w-md">
              <CardHeader>
                <CardTitle>Add New User</CardTitle>
                <CardDescription>Create a new customer account</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label className="text-sm font-medium">First Name *</label>
                    <Input
                      value={newUser.firstName}
                      onChange={(e) => setNewUser({...newUser, firstName: e.target.value})}
                      placeholder="John"
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium">Last Name *</label>
                    <Input
                      value={newUser.lastName}
                      onChange={(e) => setNewUser({...newUser, lastName: e.target.value})}
                      placeholder="Doe"
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Email *</label>
                  <Input
                    type="email"
                    value={newUser.email}
                    onChange={(e) => setNewUser({...newUser, email: e.target.value})}
                    placeholder="john@example.com"
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Password *</label>
                  <Input
                    type="password"
                    value={newUser.password}
                    onChange={(e) => setNewUser({...newUser, password: e.target.value})}
                    placeholder="Min 8 characters"
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Company</label>
                  <Input
                    value={newUser.company}
                    onChange={(e) => setNewUser({...newUser, company: e.target.value})}
                    placeholder="Acme Inc."
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Role</label>
                  <select
                    className="w-full p-2 border rounded-md bg-background"
                    value={newUser.role}
                    onChange={(e) => setNewUser({...newUser, role: e.target.value})}
                  >
                    <option value="customer">Customer</option>
                    <option value="admin">Admin</option>
                  </select>
                </div>
                <div className="flex gap-2 pt-4">
                  <Button variant="outline" className="flex-1" onClick={() => setShowAddUser(false)}>
                    Cancel
                  </Button>
                  <Button className="flex-1" onClick={addUser} disabled={addingUser}>
                    {addingUser ? 'Creating...' : 'Create User'}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>
        )}

        {/* Users Tab */}
        {activeTab === 'users' && (
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>Users</CardTitle>
                  <CardDescription>Manage customer accounts</CardDescription>
                </div>
                <div className="flex gap-2">
                  <div className="relative">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                    <Input
                      placeholder="Search users..."
                      value={searchTerm}
                      onChange={(e) => setSearchTerm(e.target.value)}
                      className="pl-9 w-64"
                    />
                  </div>
                  <Button onClick={() => setShowAddUser(true)}>
                    <Plus className="h-4 w-4 mr-2" />
                    Add User
                  </Button>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="text-center py-8 text-muted-foreground">Loading...</div>
              ) : filteredUsers.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">No users found</div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full">
                    <thead>
                      <tr className="border-b">
                        <th className="text-left py-3 px-4">User</th>
                        <th className="text-left py-3 px-4">Plan</th>
                        <th className="text-left py-3 px-4">Balance</th>
                        <th className="text-left py-3 px-4">Used</th>
                        <th className="text-left py-3 px-4">Status</th>
                        <th className="text-left py-3 px-4">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredUsers.map((user) => (
                        <tr key={user.id} className="border-b hover:bg-muted/50">
                          <td className="py-3 px-4">
                            <div>
                              <div className="font-medium">{user.firstName} {user.lastName}</div>
                              <div className="text-sm text-muted-foreground">{user.email}</div>
                              {user.company && (
                                <div className="text-xs text-muted-foreground">{user.company}</div>
                              )}
                            </div>
                          </td>
                          <td className="py-3 px-4">
                            <Badge variant="outline">{user.planName || 'Free'}</Badge>
                          </td>
                          <td className="py-3 px-4">{user.gbBalance?.toFixed(2) || 0} GB</td>
                          <td className="py-3 px-4">{user.gbUsed?.toFixed(2) || 0} GB</td>
                          <td className="py-3 px-4">
                            <Badge variant={user.status === 'active' ? 'default' : 'secondary'}>
                              {user.status}
                            </Badge>
                          </td>
                          <td className="py-3 px-4">
                            <div className="flex gap-1">
                              <Button 
                                size="sm" 
                                variant="ghost"
                                onClick={() => {
                                  setShowEditUser(user)
                                  setEditBalance(user.gbBalance?.toString() || '0')
                                }}
                              >
                                <Edit className="h-4 w-4" />
                              </Button>
                              <Button 
                                size="sm" 
                                variant="ghost" 
                                className="text-destructive"
                                onClick={() => deleteUser(user.id, user.email)}
                              >
                                <Trash2 className="h-4 w-4" />
                              </Button>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </CardContent>
          </Card>
        )}

        {/* Add Plan Modal */}
        {showAddPlan && (
          <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <Card className="w-full max-w-md">
              <CardHeader>
                <CardTitle>Add New Plan</CardTitle>
                <CardDescription>Create a new pricing plan</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Plan Name *</label>
                  <Input
                    value={newPlan.name}
                    onChange={(e) => setNewPlan({...newPlan, name: e.target.value})}
                    placeholder="e.g. Basic, Pro, Enterprise"
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Description</label>
                  <Input
                    value={newPlan.description}
                    onChange={(e) => setNewPlan({...newPlan, description: e.target.value})}
                    placeholder="Plan description"
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label className="text-sm font-medium">Price per GB ($) *</label>
                    <Input
                      type="number"
                      step="0.01"
                      value={newPlan.pricePerGb}
                      onChange={(e) => setNewPlan({...newPlan, pricePerGb: e.target.value})}
                      placeholder="5.00"
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium">Included GB</label>
                    <Input
                      type="number"
                      value={newPlan.includedGb}
                      onChange={(e) => setNewPlan({...newPlan, includedGb: e.target.value})}
                      placeholder="0"
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Max Connections</label>
                  <Input
                    type="number"
                    value={newPlan.maxConnections}
                    onChange={(e) => setNewPlan({...newPlan, maxConnections: e.target.value})}
                    placeholder="10"
                  />
                </div>
                <div className="flex gap-2 pt-4">
                  <Button variant="outline" className="flex-1" onClick={() => setShowAddPlan(false)}>
                    Cancel
                  </Button>
                  <Button className="flex-1" onClick={addPlan} disabled={addingPlan}>
                    {addingPlan ? 'Creating...' : 'Create Plan'}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>
        )}

        {/* Plans Tab */}
        {activeTab === 'plans' && (
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>Plans</CardTitle>
                  <CardDescription>Manage pricing plans</CardDescription>
                </div>
                <Button size="sm" onClick={() => setShowAddPlan(true)}>
                  <Plus className="h-4 w-4 mr-2" />
                  Add Plan
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4 md:grid-cols-3">
                {plans.map((plan) => (
                  <Card key={plan.id}>
                    <CardHeader>
                      <CardTitle className="flex items-center justify-between">
                        {plan.name}
                        <Badge variant={plan.isActive ? 'default' : 'secondary'}>
                          {plan.isActive ? 'Active' : 'Inactive'}
                        </Badge>
                      </CardTitle>
                      {(plan as any).description && (
                        <CardDescription>{(plan as any).description}</CardDescription>
                      )}
                    </CardHeader>
                    <CardContent>
                      <div className="space-y-2">
                        <div className="flex justify-between">
                          <span className="text-muted-foreground">Price/GB:</span>
                          <span className="font-medium text-lg">${plan.pricePerGb}</span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-muted-foreground">Included GB:</span>
                          <span className="font-medium">{plan.monthlyGb} GB</span>
                        </div>
                        {(plan as any).maxConnections && (
                          <div className="flex justify-between">
                            <span className="text-muted-foreground">Max Connections:</span>
                            <span className="font-medium">{(plan as any).maxConnections}</span>
                          </div>
                        )}
                      </div>
                      <div className="flex gap-2 mt-4">
                        <Button size="sm" variant="outline" className="flex-1">
                          <Edit className="h-4 w-4 mr-1" />
                          Edit
                        </Button>
                      </div>
                    </CardContent>
                  </Card>
                ))}
                
                {plans.length === 0 && (
                  <div className="col-span-3 text-center py-8 text-muted-foreground">
                    No plans configured. Click "Add Plan" to create one.
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        )}

        {/* Supply Tab */}
        {activeTab === 'supply' && (
          <div className="space-y-6">
            {/* Supply Stats */}
            <div className="grid gap-4 md:grid-cols-4">
              <Card>
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium">Total Nodes</CardTitle>
                  <Server className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{nodeStats?.totalNodes || 0}</div>
                  <p className="text-xs text-muted-foreground">All registered</p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium">Active Nodes</CardTitle>
                  <Wifi className="h-4 w-4 text-green-500" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold text-green-600">{nodeStats?.activeNodes || 0}</div>
                  <p className="text-xs text-muted-foreground">Currently online</p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium">Countries</CardTitle>
                  <Globe className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {Object.keys(nodeStats?.countryBreakdown || {}).length}
                  </div>
                  <p className="text-xs text-muted-foreground">Geographic coverage</p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium">Device Types</CardTitle>
                  <Activity className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {Object.keys(nodeStats?.deviceTypes || {}).length}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {Object.entries(nodeStats?.deviceTypes || {}).map(([type, count]) => 
                      `${type}: ${count}`
                    ).join(', ') || 'None'}
                  </p>
                </CardContent>
              </Card>
            </div>

            {/* Country Breakdown */}
            {nodeStats?.countryBreakdown && Object.keys(nodeStats.countryBreakdown).length > 0 && (
              <Card>
                <CardHeader>
                  <CardTitle>Geographic Distribution</CardTitle>
                  <CardDescription>Nodes by country</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="flex flex-wrap gap-2">
                    {Object.entries(nodeStats.countryBreakdown)
                      .sort(([,a], [,b]) => (b as number) - (a as number))
                      .map(([country, count]) => (
                        <Badge key={country} variant="outline" className="text-sm">
                          {country}: {count as number}
                        </Badge>
                      ))
                    }
                  </div>
                </CardContent>
              </Card>
            )}

            {/* Nodes Table */}
            <Card>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle>Connected Nodes</CardTitle>
                    <CardDescription>SDK devices in the network</CardDescription>
                  </div>
                  <Button size="sm" variant="outline" onClick={fetchData}>
                    <RefreshCw className="h-4 w-4 mr-2" />
                    Refresh
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                {loading ? (
                  <div className="text-center py-8 text-muted-foreground">Loading...</div>
                ) : nodes.length === 0 ? (
                  <div className="text-center py-8 text-muted-foreground">
                    <Server className="h-12 w-12 mx-auto mb-4 opacity-50" />
                    <p>No nodes connected yet</p>
                    <p className="text-sm mt-2">Deploy the SDK to start building your network</p>
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full">
                      <thead>
                        <tr className="border-b">
                          <th className="text-left py-3 px-4">Status</th>
                          <th className="text-left py-3 px-4">Location</th>
                          <th className="text-left py-3 px-4">Device</th>
                          <th className="text-left py-3 px-4">IP</th>
                          <th className="text-left py-3 px-4">Quality</th>
                          <th className="text-left py-3 px-4">Last Seen</th>
                        </tr>
                      </thead>
                      <tbody>
                        {nodes.map((node) => (
                          <tr key={node.id} className="border-b hover:bg-muted/50">
                            <td className="py-3 px-4">
                              {node.status === 'available' ? (
                                <Badge variant="default" className="bg-green-600">
                                  <Wifi className="h-3 w-3 mr-1" />
                                  Online
                                </Badge>
                              ) : (
                                <Badge variant="secondary">
                                  <WifiOff className="h-3 w-3 mr-1" />
                                  {node.status}
                                </Badge>
                              )}
                            </td>
                            <td className="py-3 px-4">
                              <div className="flex items-center gap-2">
                                <MapPin className="h-4 w-4 text-muted-foreground" />
                                <div>
                                  <div className="font-medium">{node.city || 'Unknown'}</div>
                                  <div className="text-xs text-muted-foreground">
                                    {node.countryName || node.country} • {node.isp || 'Unknown ISP'}
                                  </div>
                                </div>
                              </div>
                            </td>
                            <td className="py-3 px-4">
                              <div>
                                <Badge variant="outline">{node.deviceType || 'unknown'}</Badge>
                                <div className="text-xs text-muted-foreground mt-1">
                                  {node.connectionType || 'wifi'} • v{node.sdkVersion || '1.0.0'}
                                </div>
                              </div>
                            </td>
                            <td className="py-3 px-4">
                              <code className="text-xs bg-muted px-2 py-1 rounded">
                                {node.ipAddress}
                              </code>
                            </td>
                            <td className="py-3 px-4">
                              <div className="flex items-center gap-2">
                                <div className="w-16 h-2 bg-muted rounded-full overflow-hidden">
                                  <div 
                                    className={`h-full ${
                                      node.qualityScore >= 80 ? 'bg-green-500' : 
                                      node.qualityScore >= 50 ? 'bg-yellow-500' : 'bg-red-500'
                                    }`}
                                    style={{ width: `${node.qualityScore}%` }}
                                  />
                                </div>
                                <span className="text-sm">{node.qualityScore}%</span>
                              </div>
                            </td>
                            <td className="py-3 px-4 text-sm text-muted-foreground">
                              {node.lastHeartbeat ? (
                                new Date(node.lastHeartbeat).toLocaleString()
                              ) : 'Never'}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        )}

        {/* Settings Tab */}
        {activeTab === 'settings' && (
          <Card>
            <CardHeader>
              <CardTitle>System Settings</CardTitle>
              <CardDescription>Configure global settings</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Default Price per GB</label>
                  <Input type="number" placeholder="5.00" defaultValue="5.00" />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Free Tier GB</label>
                  <Input type="number" placeholder="5" defaultValue="5" />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Rate Limit (req/min)</label>
                  <Input type="number" placeholder="100" defaultValue="100" />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Session Timeout (min)</label>
                  <Input type="number" placeholder="30" defaultValue="30" />
                </div>
              </div>
              <Button>Save Settings</Button>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}
