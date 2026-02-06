'use client';

import Link from 'next/link';

const financePages = [
  {
    title: 'System Costs',
    description: 'AI usage and infrastructure costs breakdown by person, project, and platform',
    href: '/finance/system-costs',
    icon: '💰',
    color: 'from-purple-600 to-indigo-600',
  },
  {
    title: 'Invoices',
    description: 'View and manage invoices, billing history',
    href: '/billing',
    icon: '📄',
    color: 'from-green-600 to-emerald-600',
  },
  {
    title: 'Revenue',
    description: 'Track revenue across all products and partners',
    href: '/finance/revenue',
    icon: '📈',
    color: 'from-blue-600 to-cyan-600',
  },
  {
    title: 'Expenses',
    description: 'Monthly expenses and budget tracking',
    href: '/finance/expenses',
    icon: '💳',
    color: 'from-orange-600 to-red-600',
  },
];

export default function FinancePage() {
  return (
    <div className="min-h-screen bg-gray-950 text-white p-6">
      <div className="mb-8">
        <h1 className="text-3xl font-bold">💼 Finance</h1>
        <p className="text-gray-400 mt-1">Financial overview and cost tracking</p>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <p className="text-gray-400 text-sm">Weekly AI Cost</p>
          <p className="text-3xl font-bold text-purple-400">$124</p>
          <p className="text-green-400 text-sm mt-1">↓ 12% vs last week</p>
        </div>
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <p className="text-gray-400 text-sm">Monthly Revenue</p>
          <p className="text-3xl font-bold text-green-400">$938K</p>
          <p className="text-green-400 text-sm mt-1">↑ 8% vs last month</p>
        </div>
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <p className="text-gray-400 text-sm">Pending Invoices</p>
          <p className="text-3xl font-bold text-yellow-400">3</p>
          <p className="text-gray-400 text-sm mt-1">$12,450 total</p>
        </div>
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <p className="text-gray-400 text-sm">Monthly Projection</p>
          <p className="text-3xl font-bold text-blue-400">$450</p>
          <p className="text-gray-400 text-sm mt-1">AI costs</p>
        </div>
      </div>

      {/* Finance Sections */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {financePages.map((page) => (
          <Link
            key={page.title}
            href={page.href}
            className="bg-gray-900 rounded-xl p-6 border border-gray-800 hover:border-gray-600 transition group"
          >
            <div className="flex items-start gap-4">
              <div className={`w-12 h-12 rounded-xl bg-gradient-to-br ${page.color} flex items-center justify-center text-2xl`}>
                {page.icon}
              </div>
              <div>
                <h2 className="text-xl font-semibold group-hover:text-purple-400 transition">{page.title}</h2>
                <p className="text-gray-400 mt-1">{page.description}</p>
              </div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
