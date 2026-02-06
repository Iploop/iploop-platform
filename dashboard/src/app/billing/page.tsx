'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  CreditCard, Download, CheckCircle, AlertTriangle,
  TrendingUp, Calendar, DollarSign, Zap, ArrowUpRight
} from 'lucide-react';

interface Plan {
  id: string;
  name: string;
  price_monthly: number;
  bandwidth_gb: number;
  requests_per_day: number;
  concurrent_conns: number;
  features: string[];
}

interface Subscription {
  plan_id: string;
  status: string;
  current_period_end: string;
  cancel_at_period_end: boolean;
}

interface UsageSummary {
  total_bytes: number;
  total_requests: number;
  plan_limit_bytes: number;
  usage_percent: number;
  estimated_cost: number;
}

interface Invoice {
  id: string;
  amount_paid: number;
  status: string;
  created: number;
  invoice_pdf?: string;
}

export default function BillingPage() {
  const [plans, setPlans] = useState<Plan[]>([]);
  const [subscription, setSubscription] = useState<Subscription | null>(null);
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  const [loading, setLoading] = useState(true);
  const [upgrading, setUpgrading] = useState(false);

  useEffect(() => {
    fetchBillingData();
  }, []);

  const fetchBillingData = async () => {
    try {
      const results = await Promise.all([
        fetch('/api/billing/plans'),
        fetch('/api/billing/subscription'),
        fetch('/api/usage/summary'),
        fetch('/api/billing/invoices')
      ]);
      const [plansRes, subRes, usageRes, invoicesRes] = results;

      const plansData = await plansRes.json();
      const subData = await subRes.json();
      const usageData = await usageRes.json();
      const invoicesData = await invoicesRes.json();

      setPlans(plansData || []);
      setSubscription(subData);
      setUsage(usageData);
      setInvoices(invoicesData || []);
    } catch (error) {
      console.error('Failed to fetch billing data:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleUpgrade = async (planId: string) => {
    setUpgrading(true);
    try {
      const res = await fetch('/api/billing/checkout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          plan_id: planId,
          success_url: `${window.location.origin}/billing?success=true`,
          cancel_url: `${window.location.origin}/billing?canceled=true`
        })
      });
      const data = await res.json();
      if (data.checkout_url) {
        window.location.href = data.checkout_url;
      }
    } catch (error) {
      console.error('Failed to create checkout:', error);
    } finally {
      setUpgrading(false);
    }
  };

  const handleCancelSubscription = async () => {
    if (!confirm('Are you sure you want to cancel your subscription?')) return;
    
    try {
      await fetch('/api/billing/subscription/cancel', { method: 'POST' });
      fetchBillingData();
    } catch (error) {
      console.error('Failed to cancel subscription:', error);
    }
  };

  const formatCurrency = (cents: number) => {
    return `$${(cents / 100).toFixed(2)}`;
  };

  const formatBytes = (bytes: number) => {
    const gb = bytes / (1024 * 1024 * 1024);
    if (gb >= 1) return `${gb.toFixed(2)} GB`;
    const mb = bytes / (1024 * 1024);
    return `${mb.toFixed(2)} MB`;
  };

  const formatDate = (timestamp: number) => {
    return new Date(timestamp * 1000).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'long',
      day: 'numeric'
    });
  };

  const currentPlan = plans.find(p => p.id === subscription?.plan_id);

  if (loading) {
    return (
      <div className="p-6 flex items-center justify-center h-64">
        <div className="text-muted-foreground">Loading billing information...</div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Billing & Usage</h1>
        <p className="text-muted-foreground">
          Manage your subscription and monitor usage
        </p>
      </div>

      <Tabs defaultValue="overview" className="space-y-6">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="plans">Plans</TabsTrigger>
          <TabsTrigger value="invoices">Invoices</TabsTrigger>
        </TabsList>

        {/* Overview Tab */}
        <TabsContent value="overview" className="space-y-6">
          {/* Current Plan & Usage */}
          <div className="grid md:grid-cols-2 gap-6">
            {/* Current Plan */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center justify-between">
                  <span>Current Plan</span>
                  {subscription?.status === 'active' && (
                    <Badge variant="default">Active</Badge>
                  )}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                {currentPlan ? (
                  <>
                    <div>
                      <div className="text-3xl font-bold">{currentPlan.name}</div>
                      <div className="text-muted-foreground">
                        {formatCurrency(currentPlan.price_monthly)}/month
                      </div>
                    </div>
                    <div className="space-y-2 text-sm">
                      <div className="flex justify-between">
                        <span>Bandwidth</span>
                        <span>{currentPlan.bandwidth_gb} GB/month</span>
                      </div>
                      <div className="flex justify-between">
                        <span>Requests</span>
                        <span>{currentPlan.requests_per_day.toLocaleString()}/day</span>
                      </div>
                      <div className="flex justify-between">
                        <span>Connections</span>
                        <span>{currentPlan.concurrent_conns} concurrent</span>
                      </div>
                    </div>
                    {subscription?.current_period_end && (
                      <div className="pt-4 border-t text-sm text-muted-foreground">
                        <Calendar className="w-4 h-4 inline mr-2" />
                        {subscription.cancel_at_period_end 
                          ? `Cancels on ${new Date(subscription.current_period_end).toLocaleDateString()}`
                          : `Renews on ${new Date(subscription.current_period_end).toLocaleDateString()}`
                        }
                      </div>
                    )}
                    <div className="pt-4 flex gap-2">
                      <Button variant="outline" size="sm" onClick={() => handleCancelSubscription()}>
                        {subscription?.cancel_at_period_end ? 'Resume' : 'Cancel'}
                      </Button>
                      <Button size="sm">
                        Change Plan
                      </Button>
                    </div>
                  </>
                ) : (
                  <div className="text-center py-4">
                    <p className="text-muted-foreground mb-4">No active subscription</p>
                    <Button onClick={() => handleUpgrade('starter')}>
                      Get Started
                    </Button>
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Usage This Month */}
            <Card>
              <CardHeader>
                <CardTitle>Usage This Month</CardTitle>
              </CardHeader>
              <CardContent className="space-y-6">
                {usage && (
                  <>
                    {/* Bandwidth */}
                    <div className="space-y-2">
                      <div className="flex justify-between text-sm">
                        <span>Bandwidth</span>
                        <span>
                          {formatBytes(usage.total_bytes)} / {formatBytes(usage.plan_limit_bytes)}
                        </span>
                      </div>
                      <Progress value={usage.usage_percent} />
                      {usage.usage_percent > 80 && (
                        <div className="text-xs text-yellow-500 flex items-center gap-1">
                          <AlertTriangle className="w-3 h-3" />
                          {usage.usage_percent > 90 ? 'Almost at limit!' : 'Approaching limit'}
                        </div>
                      )}
                    </div>

                    {/* Requests */}
                    <div className="space-y-2">
                      <div className="flex justify-between text-sm">
                        <span>Requests</span>
                        <span>{usage.total_requests.toLocaleString()}</span>
                      </div>
                    </div>

                    {/* Estimated Cost */}
                    {usage.estimated_cost > 0 && (
                      <div className="pt-4 border-t">
                        <div className="flex justify-between items-center">
                          <span className="text-sm text-muted-foreground">Estimated Cost</span>
                          <span className="text-2xl font-bold">
                            ${usage.estimated_cost.toFixed(2)}
                          </span>
                        </div>
                      </div>
                    )}
                  </>
                )}
              </CardContent>
            </Card>
          </div>

          {/* Quick Stats */}
          <div className="grid md:grid-cols-4 gap-4">
            <Card>
              <CardContent className="pt-6">
                <div className="flex items-center gap-4">
                  <div className="p-3 bg-blue-500/10 rounded-full">
                    <TrendingUp className="w-6 h-6 text-blue-500" />
                  </div>
                  <div>
                    <div className="text-2xl font-bold">
                      {usage ? formatBytes(usage.total_bytes) : '0 B'}
                    </div>
                    <div className="text-sm text-muted-foreground">Data Used</div>
                  </div>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="pt-6">
                <div className="flex items-center gap-4">
                  <div className="p-3 bg-green-500/10 rounded-full">
                    <Zap className="w-6 h-6 text-green-500" />
                  </div>
                  <div>
                    <div className="text-2xl font-bold">
                      {usage?.total_requests.toLocaleString() || 0}
                    </div>
                    <div className="text-sm text-muted-foreground">Requests</div>
                  </div>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="pt-6">
                <div className="flex items-center gap-4">
                  <div className="p-3 bg-purple-500/10 rounded-full">
                    <DollarSign className="w-6 h-6 text-purple-500" />
                  </div>
                  <div>
                    <div className="text-2xl font-bold">
                      {currentPlan ? formatCurrency(currentPlan.price_monthly) : '$0'}
                    </div>
                    <div className="text-sm text-muted-foreground">Monthly Cost</div>
                  </div>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="pt-6">
                <div className="flex items-center gap-4">
                  <div className="p-3 bg-yellow-500/10 rounded-full">
                    <CreditCard className="w-6 h-6 text-yellow-500" />
                  </div>
                  <div>
                    <div className="text-2xl font-bold">
                      {invoices.filter(i => i.status === 'paid').length}
                    </div>
                    <div className="text-sm text-muted-foreground">Paid Invoices</div>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        {/* Plans Tab */}
        <TabsContent value="plans">
          <div className="grid md:grid-cols-4 gap-6">
            {plans.filter(p => p.id !== 'enterprise').map((plan) => (
              <Card 
                key={plan.id} 
                className={plan.id === subscription?.plan_id ? 'border-primary' : ''}
              >
                <CardHeader>
                  <CardTitle className="flex items-center justify-between">
                    {plan.name}
                    {plan.id === subscription?.plan_id && (
                      <Badge>Current</Badge>
                    )}
                  </CardTitle>
                  <CardDescription>
                    <span className="text-3xl font-bold text-foreground">
                      {plan.price_monthly === 0 ? 'Custom' : formatCurrency(plan.price_monthly)}
                    </span>
                    {plan.price_monthly > 0 && (
                      <span className="text-muted-foreground">/month</span>
                    )}
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <ul className="space-y-2">
                    {plan.features.map((feature, i) => (
                      <li key={i} className="flex items-start gap-2 text-sm">
                        <CheckCircle className="w-4 h-4 text-green-500 mt-0.5 flex-shrink-0" />
                        {feature}
                      </li>
                    ))}
                  </ul>
                  <Button 
                    className="w-full" 
                    variant={plan.id === subscription?.plan_id ? 'outline' : 'default'}
                    disabled={plan.id === subscription?.plan_id || upgrading}
                    onClick={() => handleUpgrade(plan.id)}
                  >
                    {plan.id === subscription?.plan_id ? 'Current Plan' : 'Upgrade'}
                    {plan.id !== subscription?.plan_id && (
                      <ArrowUpRight className="w-4 h-4 ml-2" />
                    )}
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        </TabsContent>

        {/* Invoices Tab */}
        <TabsContent value="invoices">
          <Card>
            <CardHeader>
              <CardTitle>Invoice History</CardTitle>
            </CardHeader>
            <CardContent>
              {invoices.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  No invoices yet
                </div>
              ) : (
                <div className="space-y-4">
                  {invoices.map((invoice) => (
                    <div 
                      key={invoice.id}
                      className="flex items-center justify-between p-4 border rounded-lg"
                    >
                      <div className="flex items-center gap-4">
                        <div className="p-2 bg-muted rounded">
                          <CreditCard className="w-5 h-5" />
                        </div>
                        <div>
                          <div className="font-medium">
                            {formatCurrency(invoice.amount_paid)}
                          </div>
                          <div className="text-sm text-muted-foreground">
                            {formatDate(invoice.created)}
                          </div>
                        </div>
                      </div>
                      <div className="flex items-center gap-4">
                        <Badge variant={invoice.status === 'paid' ? 'default' : 'secondary'}>
                          {invoice.status}
                        </Badge>
                        {invoice.invoice_pdf && (
                          <a 
                            href={invoice.invoice_pdf} 
                            target="_blank" 
                            rel="noopener noreferrer"
                            className="inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground h-9 w-9"
                          >
                            <Download className="w-4 h-4" />
                          </a>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
