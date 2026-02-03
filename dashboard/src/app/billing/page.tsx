'use client'

import { Layout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { 
  CreditCard, 
  Download, 
  Plus, 
  AlertCircle, 
  TrendingUp, 
  DollarSign,
  Calendar,
  Zap,
  Shield,
  CheckCircle
} from 'lucide-react'
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, ResponsiveContainer, BarChart, Bar } from 'recharts'

const usageData = [
  { date: '2024-01-01', cost: 45.20, requests: 120000, bandwidth: 18.5 },
  { date: '2024-01-02', cost: 52.80, requests: 140000, bandwidth: 22.1 },
  { date: '2024-01-03', cost: 38.90, requests: 98000, bandwidth: 15.2 },
  { date: '2024-01-04', cost: 61.40, requests: 165000, bandwidth: 28.9 },
  { date: '2024-01-05', cost: 49.60, requests: 132000, bandwidth: 20.3 },
  { date: '2024-01-06', cost: 55.70, requests: 148000, bandwidth: 24.1 },
  { date: '2024-01-07', cost: 43.20, requests: 115000, bandwidth: 17.8 }
]

const monthlyUsage = [
  { month: 'Aug', cost: 1240.50, budget: 2000 },
  { month: 'Sep', cost: 1680.20, budget: 2000 },
  { month: 'Oct', cost: 1890.75, budget: 2000 },
  { month: 'Nov', cost: 2120.40, budget: 2500 },
  { month: 'Dec', cost: 1950.30, budget: 2500 },
  { month: 'Jan', cost: 2240.80, budget: 2500 }
]

const transactions = [
  { id: 'inv_001', date: '2024-02-01', description: 'Monthly Subscription - Pro Plan', amount: 299.00, status: 'paid' },
  { id: 'inv_002', date: '2024-01-15', description: 'Additional Bandwidth - 500GB', amount: 150.00, status: 'paid' },
  { id: 'inv_003', date: '2024-01-01', description: 'Monthly Subscription - Pro Plan', amount: 299.00, status: 'paid' },
  { id: 'inv_004', date: '2023-12-01', description: 'Monthly Subscription - Pro Plan', amount: 299.00, status: 'paid' },
  { id: 'inv_005', date: '2023-11-20', description: 'Setup Fee - Premium Endpoints', amount: 99.00, status: 'paid' }
]

const plans = [
  {
    name: 'Starter',
    price: 29,
    period: 'month',
    features: ['100K requests/month', '50GB bandwidth', 'Basic endpoints', 'Email support'],
    current: false
  },
  {
    name: 'Pro',
    price: 299,
    period: 'month',
    features: ['2M requests/month', '1TB bandwidth', 'Premium endpoints', 'Priority support', 'Custom integrations'],
    current: true
  },
  {
    name: 'Enterprise',
    price: 999,
    period: 'month',
    features: ['Unlimited requests', 'Unlimited bandwidth', 'Dedicated endpoints', '24/7 phone support', 'SLA guarantee'],
    current: false
  }
]

export default function BillingPage() {
  const currentBalance = 1456.80
  const monthlySpend = 2240.80
  const remainingCredits = 759200
  const billingCycle = '2024-03-01'

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold">Billing & Credits</h1>
            <p className="text-muted-foreground">Manage your subscription and track usage costs</p>
          </div>
          <div className="flex gap-2">
            <Button variant="outline">
              <Download className="w-4 h-4 mr-2" />
              Download Invoice
            </Button>
            <Button>
              <Plus className="w-4 h-4 mr-2" />
              Add Credits
            </Button>
          </div>
        </div>

        {/* Account Overview */}
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Current Balance</CardTitle>
              <DollarSign className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">${currentBalance.toFixed(2)}</div>
              <p className="text-xs text-muted-foreground">Available credits</p>
            </CardContent>
          </Card>
          
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">This Month</CardTitle>
              <TrendingUp className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">${monthlySpend.toFixed(2)}</div>
              <p className="text-xs text-muted-foreground">
                <span className="text-red-500">+12.5%</span> from last month
              </p>
            </CardContent>
          </Card>
          
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Remaining Requests</CardTitle>
              <Zap className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{(remainingCredits / 1000).toFixed(0)}K</div>
              <p className="text-xs text-muted-foreground">of 2M monthly limit</p>
            </CardContent>
          </Card>
          
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Next Billing</CardTitle>
              <Calendar className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">Mar 1</div>
              <p className="text-xs text-muted-foreground">Pro Plan renewal</p>
            </CardContent>
          </Card>
        </div>

        {/* Low Balance Warning */}
        <Card className="border-yellow-200 bg-yellow-50 dark:bg-yellow-900/20 dark:border-yellow-800">
          <CardContent className="pt-6">
            <div className="flex items-start gap-3">
              <AlertCircle className="w-5 h-5 text-yellow-600 dark:text-yellow-400 mt-0.5" />
              <div>
                <h3 className="font-semibold text-yellow-800 dark:text-yellow-200">Balance Running Low</h3>
                <p className="text-sm text-yellow-700 dark:text-yellow-300 mt-1">
                  Your account balance is below $2,000. Consider adding credits to avoid service interruption.
                </p>
                <Button variant="outline" size="sm" className="mt-2">
                  Add Credits Now
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        <div className="grid gap-4 lg:grid-cols-3">
          {/* Daily Usage Chart */}
          <Card className="lg:col-span-2">
            <CardHeader>
              <CardTitle>Daily Usage & Costs</CardTitle>
              <CardDescription>Track your spending and usage patterns</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="h-[300px]">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={usageData}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                    <XAxis 
                      dataKey="date" 
                      className="text-muted-foreground"
                      tickFormatter={(value) => new Date(value).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                    />
                    <YAxis className="text-muted-foreground" tickFormatter={(value) => `$${value}`} />
                    <Area 
                      type="monotone" 
                      dataKey="cost" 
                      stroke="#3b82f6" 
                      fill="#3b82f6" 
                      fillOpacity={0.6} 
                    />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>

          {/* Current Plan */}
          <Card>
            <CardHeader>
              <CardTitle>Current Plan</CardTitle>
              <CardDescription>Pro subscription details</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="text-center">
                <div className="text-3xl font-bold">$299</div>
                <div className="text-sm text-muted-foreground">per month</div>
              </div>
              
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-sm">
                  <CheckCircle className="w-4 h-4 text-green-500" />
                  <span>2M requests/month</span>
                </div>
                <div className="flex items-center gap-2 text-sm">
                  <CheckCircle className="w-4 h-4 text-green-500" />
                  <span>1TB bandwidth</span>
                </div>
                <div className="flex items-center gap-2 text-sm">
                  <CheckCircle className="w-4 h-4 text-green-500" />
                  <span>Premium endpoints</span>
                </div>
                <div className="flex items-center gap-2 text-sm">
                  <CheckCircle className="w-4 h-4 text-green-500" />
                  <span>Priority support</span>
                </div>
              </div>

              <div className="space-y-2">
                <Button className="w-full" variant="outline">
                  Change Plan
                </Button>
                <Button className="w-full" variant="ghost">
                  Cancel Subscription
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Monthly Budget Overview */}
        <Card>
          <CardHeader>
            <CardTitle>Monthly Spending Trends</CardTitle>
            <CardDescription>Track your monthly costs against budget</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[250px]">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={monthlyUsage}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis dataKey="month" className="text-muted-foreground" />
                  <YAxis className="text-muted-foreground" tickFormatter={(value) => `$${value}`} />
                  <Bar dataKey="cost" fill="#3b82f6" />
                  <Bar dataKey="budget" fill="#e5e7eb" opacity={0.5} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>

        {/* Transaction History */}
        <Card>
          <CardHeader>
            <CardTitle>Recent Transactions</CardTitle>
            <CardDescription>Your payment and billing history</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {transactions.map((transaction) => (
                <div key={transaction.id} className="flex items-center justify-between p-4 border border-border rounded-lg">
                  <div className="flex items-center gap-4">
                    <CreditCard className="w-8 h-8 text-muted-foreground" />
                    <div>
                      <p className="font-medium">{transaction.description}</p>
                      <p className="text-sm text-muted-foreground">
                        {new Date(transaction.date).toLocaleDateString()} • {transaction.id}
                      </p>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-lg font-semibold">${transaction.amount.toFixed(2)}</div>
                    <Badge variant="success">Paid</Badge>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Available Plans */}
        <Card>
          <CardHeader>
            <CardTitle>Available Plans</CardTitle>
            <CardDescription>Choose the right plan for your needs</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid gap-6 md:grid-cols-3">
              {plans.map((plan) => (
                <div key={plan.name} className={`p-6 rounded-lg border-2 ${
                  plan.current ? 'border-primary bg-primary/5' : 'border-border'
                }`}>
                  <div className="text-center">
                    <h3 className="text-xl font-bold">{plan.name}</h3>
                    {plan.current && <Badge className="mt-1">Current Plan</Badge>}
                    <div className="mt-4">
                      <span className="text-3xl font-bold">${plan.price}</span>
                      <span className="text-muted-foreground">/{plan.period}</span>
                    </div>
                  </div>
                  
                  <ul className="mt-6 space-y-3">
                    {plan.features.map((feature, index) => (
                      <li key={index} className="flex items-center gap-2 text-sm">
                        <CheckCircle className="w-4 h-4 text-green-500" />
                        <span>{feature}</span>
                      </li>
                    ))}
                  </ul>
                  
                  <Button 
                    className="w-full mt-6" 
                    variant={plan.current ? 'secondary' : 'default'}
                    disabled={plan.current}
                  >
                    {plan.current ? 'Current Plan' : 'Upgrade'}
                  </Button>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
    </Layout>
  )
}