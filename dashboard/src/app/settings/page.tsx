'use client'

import { useState } from 'react'
import { Layout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { 
  User, 
  Lock, 
  Bell, 
  Shield, 
  Key, 
  Eye, 
  EyeOff, 
  Check,
  Mail,
  Phone,
  Globe,
  Smartphone,
  AlertTriangle,
  Trash2
} from 'lucide-react'

export default function SettingsPage() {
  const [showCurrentPassword, setShowCurrentPassword] = useState(false)
  const [showNewPassword, setShowNewPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)
  
  // Form states
  const [profile, setProfile] = useState({
    firstName: 'John',
    lastName: 'Doe',
    email: 'demo@iploop.io',
    company: 'Demo Corp',
    phone: '+1 (555) 123-4567',
    timezone: 'America/New_York'
  })

  const [passwords, setPasswords] = useState({
    current: '',
    new: '',
    confirm: ''
  })

  const [notifications, setNotifications] = useState({
    emailAlerts: true,
    usageAlerts: true,
    billingAlerts: true,
    securityAlerts: true,
    maintenanceAlerts: false,
    weeklyReports: true,
    monthlyReports: true
  })

  const [security, setSecurity] = useState({
    twoFactor: false,
    sessionTimeout: 30,
    ipWhitelist: ['192.168.1.100', '10.0.0.50']
  })

  const [apiSettings, setApiSettings] = useState({
    rateLimitAlerts: true,
    webhookUrl: 'https://myapp.com/webhook',
    retryAttempts: 3,
    timeoutSeconds: 30
  })

  const handleProfileUpdate = () => {
    // Handle profile update
    console.log('Profile updated:', profile)
  }

  const handlePasswordChange = () => {
    // Handle password change
    console.log('Password change requested')
    setPasswords({ current: '', new: '', confirm: '' })
  }

  const handleNotificationUpdate = () => {
    // Handle notification preferences update
    console.log('Notifications updated:', notifications)
  }

  const handle2FAToggle = () => {
    setSecurity(prev => ({ ...prev, twoFactor: !prev.twoFactor }))
  }

  return (
    <Layout>
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold">Account Settings</h1>
          <p className="text-muted-foreground">Manage your account preferences and security settings</p>
        </div>

        {/* Profile Settings */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <User className="w-5 h-5" />
              Profile Information
            </CardTitle>
            <CardDescription>Update your personal and company information</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label htmlFor="firstName" className="text-sm font-medium">First Name</label>
                <Input
                  id="firstName"
                  value={profile.firstName}
                  onChange={(e) => setProfile(prev => ({ ...prev, firstName: e.target.value }))}
                />
              </div>
              <div className="space-y-2">
                <label htmlFor="lastName" className="text-sm font-medium">Last Name</label>
                <Input
                  id="lastName"
                  value={profile.lastName}
                  onChange={(e) => setProfile(prev => ({ ...prev, lastName: e.target.value }))}
                />
              </div>
            </div>

            <div className="space-y-2">
              <label htmlFor="email" className="text-sm font-medium">Email Address</label>
              <Input
                id="email"
                type="email"
                value={profile.email}
                onChange={(e) => setProfile(prev => ({ ...prev, email: e.target.value }))}
              />
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label htmlFor="company" className="text-sm font-medium">Company</label>
                <Input
                  id="company"
                  value={profile.company}
                  onChange={(e) => setProfile(prev => ({ ...prev, company: e.target.value }))}
                />
              </div>
              <div className="space-y-2">
                <label htmlFor="phone" className="text-sm font-medium">Phone Number</label>
                <Input
                  id="phone"
                  value={profile.phone}
                  onChange={(e) => setProfile(prev => ({ ...prev, phone: e.target.value }))}
                />
              </div>
            </div>

            <div className="space-y-2">
              <label htmlFor="timezone" className="text-sm font-medium">Timezone</label>
              <select
                id="timezone"
                value={profile.timezone}
                onChange={(e) => setProfile(prev => ({ ...prev, timezone: e.target.value }))}
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              >
                <option value="America/New_York">Eastern Time (ET)</option>
                <option value="America/Chicago">Central Time (CT)</option>
                <option value="America/Denver">Mountain Time (MT)</option>
                <option value="America/Los_Angeles">Pacific Time (PT)</option>
                <option value="Europe/London">Greenwich Mean Time (GMT)</option>
                <option value="Europe/Berlin">Central European Time (CET)</option>
                <option value="Asia/Tokyo">Japan Standard Time (JST)</option>
              </select>
            </div>

            <Button onClick={handleProfileUpdate}>
              <Check className="w-4 h-4 mr-2" />
              Update Profile
            </Button>
          </CardContent>
        </Card>

        {/* Password Settings */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Lock className="w-5 h-5" />
              Password & Security
            </CardTitle>
            <CardDescription>Change your password and manage security settings</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label htmlFor="currentPassword" className="text-sm font-medium">Current Password</label>
              <div className="relative">
                <Input
                  id="currentPassword"
                  type={showCurrentPassword ? 'text' : 'password'}
                  value={passwords.current}
                  onChange={(e) => setPasswords(prev => ({ ...prev, current: e.target.value }))}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="absolute right-0 top-0 h-full"
                  onClick={() => setShowCurrentPassword(!showCurrentPassword)}
                >
                  {showCurrentPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </Button>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label htmlFor="newPassword" className="text-sm font-medium">New Password</label>
                <div className="relative">
                  <Input
                    id="newPassword"
                    type={showNewPassword ? 'text' : 'password'}
                    value={passwords.new}
                    onChange={(e) => setPasswords(prev => ({ ...prev, new: e.target.value }))}
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="absolute right-0 top-0 h-full"
                    onClick={() => setShowNewPassword(!showNewPassword)}
                  >
                    {showNewPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </Button>
                </div>
              </div>
              <div className="space-y-2">
                <label htmlFor="confirmPassword" className="text-sm font-medium">Confirm Password</label>
                <div className="relative">
                  <Input
                    id="confirmPassword"
                    type={showConfirmPassword ? 'text' : 'password'}
                    value={passwords.confirm}
                    onChange={(e) => setPasswords(prev => ({ ...prev, confirm: e.target.value }))}
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="absolute right-0 top-0 h-full"
                    onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                  >
                    {showConfirmPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </Button>
                </div>
              </div>
            </div>

            <Button onClick={handlePasswordChange}>
              <Lock className="w-4 h-4 mr-2" />
              Change Password
            </Button>
          </CardContent>
        </Card>

        {/* Security Settings */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Shield className="w-5 h-5" />
              Security Settings
            </CardTitle>
            <CardDescription>Advanced security and access controls</CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {/* Two-Factor Authentication */}
            <div className="flex items-center justify-between">
              <div>
                <h4 className="font-medium">Two-Factor Authentication</h4>
                <p className="text-sm text-muted-foreground">Add an extra layer of security to your account</p>
              </div>
              <div className="flex items-center gap-2">
                {security.twoFactor && (
                  <Badge variant="success">Enabled</Badge>
                )}
                <Button
                  variant={security.twoFactor ? 'destructive' : 'default'}
                  onClick={handle2FAToggle}
                >
                  <Smartphone className="w-4 h-4 mr-2" />
                  {security.twoFactor ? 'Disable' : 'Enable'} 2FA
                </Button>
              </div>
            </div>

            {/* Session Timeout */}
            <div className="space-y-2">
              <label htmlFor="sessionTimeout" className="text-sm font-medium">Session Timeout (minutes)</label>
              <select
                id="sessionTimeout"
                value={security.sessionTimeout}
                onChange={(e) => setSecurity(prev => ({ ...prev, sessionTimeout: parseInt(e.target.value) }))}
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              >
                <option value={15}>15 minutes</option>
                <option value={30}>30 minutes</option>
                <option value={60}>1 hour</option>
                <option value={120}>2 hours</option>
                <option value={480}>8 hours</option>
              </select>
            </div>

            {/* IP Whitelist */}
            <div className="space-y-2">
              <label className="text-sm font-medium">IP Whitelist</label>
              <p className="text-xs text-muted-foreground">Only allow access from these IP addresses</p>
              <div className="space-y-2">
                {security.ipWhitelist.map((ip, index) => (
                  <div key={index} className="flex items-center gap-2">
                    <Input value={ip} onChange={(e) => {
                      const newList = [...security.ipWhitelist]
                      newList[index] = e.target.value
                      setSecurity(prev => ({ ...prev, ipWhitelist: newList }))
                    }} />
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => {
                        const newList = security.ipWhitelist.filter((_, i) => i !== index)
                        setSecurity(prev => ({ ...prev, ipWhitelist: newList }))
                      }}
                    >
                      <Trash2 className="w-4 h-4" />
                    </Button>
                  </div>
                ))}
                <Button
                  variant="outline"
                  onClick={() => setSecurity(prev => ({ ...prev, ipWhitelist: [...prev.ipWhitelist, ''] }))}
                >
                  Add IP Address
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Notification Settings */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Bell className="w-5 h-5" />
              Notification Preferences
            </CardTitle>
            <CardDescription>Choose which notifications you want to receive</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="space-y-4">
                <h4 className="font-medium">Alert Notifications</h4>
                {Object.entries({
                  emailAlerts: 'Email Notifications',
                  usageAlerts: 'Usage Threshold Alerts',
                  billingAlerts: 'Billing & Payment Alerts',
                  securityAlerts: 'Security Notifications',
                  maintenanceAlerts: 'Maintenance Notifications'
                }).map(([key, label]) => (
                  <label key={key} className="flex items-center space-x-2">
                    <input
                      type="checkbox"
                      checked={notifications[key as keyof typeof notifications] as boolean}
                      onChange={(e) => setNotifications(prev => ({ ...prev, [key]: e.target.checked }))}
                      className="rounded border-border"
                    />
                    <span className="text-sm">{label}</span>
                  </label>
                ))}
              </div>

              <div className="space-y-4">
                <h4 className="font-medium">Reports</h4>
                {Object.entries({
                  weeklyReports: 'Weekly Usage Reports',
                  monthlyReports: 'Monthly Billing Reports'
                }).map(([key, label]) => (
                  <label key={key} className="flex items-center space-x-2">
                    <input
                      type="checkbox"
                      checked={notifications[key as keyof typeof notifications] as boolean}
                      onChange={(e) => setNotifications(prev => ({ ...prev, [key]: e.target.checked }))}
                      className="rounded border-border"
                    />
                    <span className="text-sm">{label}</span>
                  </label>
                ))}
              </div>
            </div>

            <Button onClick={handleNotificationUpdate}>
              <Bell className="w-4 h-4 mr-2" />
              Update Preferences
            </Button>
          </CardContent>
        </Card>

        {/* API Settings */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Key className="w-5 h-5" />
              API Configuration
            </CardTitle>
            <CardDescription>Configure API behavior and webhook settings</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label htmlFor="webhookUrl" className="text-sm font-medium">Webhook URL</label>
              <Input
                id="webhookUrl"
                value={apiSettings.webhookUrl}
                onChange={(e) => setApiSettings(prev => ({ ...prev, webhookUrl: e.target.value }))}
                placeholder="https://your-app.com/webhook"
              />
              <p className="text-xs text-muted-foreground">Receive real-time notifications about API events</p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label htmlFor="retryAttempts" className="text-sm font-medium">Retry Attempts</label>
                <select
                  id="retryAttempts"
                  value={apiSettings.retryAttempts}
                  onChange={(e) => setApiSettings(prev => ({ ...prev, retryAttempts: parseInt(e.target.value) }))}
                  className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                >
                  <option value={1}>1 attempt</option>
                  <option value={3}>3 attempts</option>
                  <option value={5}>5 attempts</option>
                  <option value={10}>10 attempts</option>
                </select>
              </div>
              <div className="space-y-2">
                <label htmlFor="timeoutSeconds" className="text-sm font-medium">Request Timeout (seconds)</label>
                <select
                  id="timeoutSeconds"
                  value={apiSettings.timeoutSeconds}
                  onChange={(e) => setApiSettings(prev => ({ ...prev, timeoutSeconds: parseInt(e.target.value) }))}
                  className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                >
                  <option value={10}>10 seconds</option>
                  <option value={30}>30 seconds</option>
                  <option value={60}>60 seconds</option>
                  <option value={120}>120 seconds</option>
                </select>
              </div>
            </div>

            <label className="flex items-center space-x-2">
              <input
                type="checkbox"
                checked={apiSettings.rateLimitAlerts}
                onChange={(e) => setApiSettings(prev => ({ ...prev, rateLimitAlerts: e.target.checked }))}
                className="rounded border-border"
              />
              <span className="text-sm">Send alerts when approaching rate limits</span>
            </label>

            <Button>
              <Key className="w-4 h-4 mr-2" />
              Update API Settings
            </Button>
          </CardContent>
        </Card>

        {/* Danger Zone */}
        <Card className="border-red-200 bg-red-50 dark:bg-red-900/20 dark:border-red-800">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-red-700 dark:text-red-300">
              <AlertTriangle className="w-5 h-5" />
              Danger Zone
            </CardTitle>
            <CardDescription className="text-red-600 dark:text-red-400">
              These actions are irreversible. Please proceed with caution.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <h4 className="font-medium text-red-700 dark:text-red-300">Delete Account</h4>
              <p className="text-sm text-red-600 dark:text-red-400">
                Permanently delete your account and all associated data. This action cannot be undone.
              </p>
              <Button variant="destructive">
                <Trash2 className="w-4 h-4 mr-2" />
                Delete Account
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </Layout>
  )
}