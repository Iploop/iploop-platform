'use client';

import { useState } from 'react';

// Cost data - would be fetched from API in production
const weeklyData = {
  byPerson: [
    { name: 'Mika', cost: 81, percentage: 65, color: 'bg-purple-500' },
    { name: 'Igal', cost: 34, percentage: 27, color: 'bg-blue-500' },
    { name: 'System/Auto', cost: 9, percentage: 8, color: 'bg-gray-500' },
  ],
  byProject: [
    { id: 'UMP-008', name: 'Management Platform', cost: 50, percentage: 40, color: 'bg-indigo-500' },
    { id: 'IPL-001', name: 'IPLoop Proxy', cost: 25, percentage: 20, color: 'bg-green-500' },
    { id: 'VRS-002', name: 'Verso', cost: 20, percentage: 16, color: 'bg-yellow-500' },
    { id: 'SFZ-004', name: 'Softzero/SOAX', cost: 8, percentage: 7, color: 'bg-orange-500' },
    { id: '-', name: 'Earn FM (Partner)', cost: 5, percentage: 4, color: 'bg-pink-500' },
    { id: 'WTH-003', name: 'Weathero', cost: 3, percentage: 2, color: 'bg-cyan-500' },
    { id: '-', name: 'General/Research', cost: 8, percentage: 7, color: 'bg-slate-500' },
    { id: '-', name: 'Personal/Misc', cost: 5, percentage: 4, color: 'bg-rose-500' },
  ],
  bySystem: [
    { name: 'Claude API (Opus)', cost: 110, percentage: 88, color: 'bg-purple-600' },
    { name: 'OpenAI (Embeddings)', cost: 5, percentage: 4, color: 'bg-green-600' },
    { name: 'Brave Search API', cost: 3, percentage: 2, color: 'bg-orange-600' },
    { name: 'Server (DO)', cost: 3, percentage: 2, color: 'bg-blue-600' },
    { name: 'Other APIs', cost: 3, percentage: 2, color: 'bg-gray-600' },
  ],
  daily: [
    { date: 'Feb 6', mika: 8, igal: 3, system: 1, total: 12, project: 'IPL-001' },
    { date: 'Feb 5', mika: 45, igal: 10, system: 3, total: 58, project: 'UMP-008' },
    { date: 'Feb 4', mika: 12, igal: 8, system: 2, total: 22, project: 'IPL-001' },
    { date: 'Feb 3', mika: 3, igal: 4, system: 1, total: 8, project: 'General' },
    { date: 'Feb 2', mika: 8, igal: 5, system: 1, total: 14, project: 'VRS-002' },
    { date: 'Feb 1', mika: 5, igal: 4, system: 1, total: 10, project: 'Setup' },
  ],
  totals: {
    weekly: 124,
    monthlyProjection: 450,
    avgDaily: 18,
  }
};

export default function SystemCostsPage() {
  const [timeRange, setTimeRange] = useState<'week' | 'month'>('week');

  return (
    <div className="min-h-screen bg-gray-950 text-white p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold">💰 System Costs</h1>
          <p className="text-gray-400 mt-1">AI usage and infrastructure costs breakdown</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setTimeRange('week')}
            className={`px-4 py-2 rounded-lg font-medium transition ${
              timeRange === 'week' ? 'bg-purple-600' : 'bg-gray-800 hover:bg-gray-700'
            }`}
          >
            This Week
          </button>
          <button
            onClick={() => setTimeRange('month')}
            className={`px-4 py-2 rounded-lg font-medium transition ${
              timeRange === 'month' ? 'bg-purple-600' : 'bg-gray-800 hover:bg-gray-700'
            }`}
          >
            This Month
          </button>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <p className="text-gray-400 text-sm">Weekly Total</p>
          <p className="text-3xl font-bold text-green-400">${weeklyData.totals.weekly}</p>
          <p className="text-gray-500 text-sm mt-1">Feb 1-6, 2026</p>
        </div>
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <p className="text-gray-400 text-sm">Monthly Projection</p>
          <p className="text-3xl font-bold text-yellow-400">${weeklyData.totals.monthlyProjection}</p>
          <p className="text-gray-500 text-sm mt-1">Based on current usage</p>
        </div>
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <p className="text-gray-400 text-sm">Daily Average</p>
          <p className="text-3xl font-bold text-blue-400">${weeklyData.totals.avgDaily}</p>
          <p className="text-gray-500 text-sm mt-1">Per day</p>
        </div>
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <p className="text-gray-400 text-sm">Biggest Day</p>
          <p className="text-3xl font-bold text-purple-400">$58</p>
          <p className="text-gray-500 text-sm mt-1">Feb 5 (UMP-008)</p>
        </div>
      </div>

      {/* Charts Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-8">
        {/* By Person */}
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <h2 className="text-xl font-semibold mb-4">👤 By Person</h2>
          <div className="space-y-4">
            {weeklyData.byPerson.map((item) => (
              <div key={item.name}>
                <div className="flex justify-between mb-1">
                  <span className="text-gray-300">{item.name}</span>
                  <span className="text-white font-medium">${item.cost} ({item.percentage}%)</span>
                </div>
                <div className="w-full bg-gray-800 rounded-full h-3">
                  <div
                    className={`${item.color} h-3 rounded-full transition-all`}
                    style={{ width: `${item.percentage}%` }}
                  />
                </div>
              </div>
            ))}
          </div>
          <div className="mt-4 pt-4 border-t border-gray-800">
            <div className="flex justify-between">
              <span className="text-gray-400">Total</span>
              <span className="text-white font-bold">${weeklyData.totals.weekly}</span>
            </div>
          </div>
        </div>

        {/* By Project */}
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <h2 className="text-xl font-semibold mb-4">📁 By Project</h2>
          <div className="space-y-3">
            {weeklyData.byProject.map((item) => (
              <div key={item.name} className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className={`w-3 h-3 rounded-full ${item.color}`} />
                  <span className="text-gray-300 text-sm">{item.name}</span>
                </div>
                <span className="text-white font-medium">${item.cost}</span>
              </div>
            ))}
          </div>
        </div>

        {/* By System */}
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <h2 className="text-xl font-semibold mb-4">⚙️ By Platform</h2>
          <div className="space-y-3">
            {weeklyData.bySystem.map((item) => (
              <div key={item.name}>
                <div className="flex justify-between mb-1">
                  <span className="text-gray-300 text-sm">{item.name}</span>
                  <span className="text-white font-medium">${item.cost}</span>
                </div>
                <div className="w-full bg-gray-800 rounded-full h-2">
                  <div
                    className={`${item.color} h-2 rounded-full`}
                    style={{ width: `${item.percentage}%` }}
                  />
                </div>
              </div>
            ))}
          </div>
          <div className="mt-4 p-3 bg-purple-900/30 rounded-lg border border-purple-800">
            <p className="text-purple-300 text-sm">💡 Claude Opus is 88% of total cost</p>
          </div>
        </div>
      </div>

      {/* Daily Breakdown Table */}
      <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
        <h2 className="text-xl font-semibold mb-4">📅 Daily Breakdown</h2>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="text-left text-gray-400 border-b border-gray-800">
                <th className="pb-3 font-medium">Date</th>
                <th className="pb-3 font-medium">Mika</th>
                <th className="pb-3 font-medium">Igal</th>
                <th className="pb-3 font-medium">System</th>
                <th className="pb-3 font-medium">Total</th>
                <th className="pb-3 font-medium">Main Project</th>
              </tr>
            </thead>
            <tbody>
              {weeklyData.daily.map((day) => (
                <tr key={day.date} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="py-3 text-white font-medium">{day.date}</td>
                  <td className="py-3 text-purple-400">${day.mika}</td>
                  <td className="py-3 text-blue-400">${day.igal}</td>
                  <td className="py-3 text-gray-400">${day.system}</td>
                  <td className="py-3 text-green-400 font-medium">${day.total}</td>
                  <td className="py-3">
                    <span className="px-2 py-1 bg-gray-800 rounded text-sm text-gray-300">
                      {day.project}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr className="text-white font-bold">
                <td className="pt-4">Total</td>
                <td className="pt-4 text-purple-400">$81</td>
                <td className="pt-4 text-blue-400">$34</td>
                <td className="pt-4 text-gray-400">$9</td>
                <td className="pt-4 text-green-400">${weeklyData.totals.weekly}</td>
                <td className="pt-4"></td>
              </tr>
            </tfoot>
          </table>
        </div>
      </div>

      {/* Insights */}
      <div className="mt-6 grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="bg-gradient-to-r from-purple-900/30 to-indigo-900/30 rounded-xl p-6 border border-purple-800">
          <h3 className="text-lg font-semibold mb-2">📊 Key Insights</h3>
          <ul className="space-y-2 text-gray-300">
            <li>• Management Platform build (Feb 5) = 45% of weekly cost</li>
            <li>• Mika&apos;s dev work drives 65% of usage</li>
            <li>• Claude Opus is the main cost driver (88%)</li>
            <li>• Daily average is ~$18/day</li>
          </ul>
        </div>
        <div className="bg-gradient-to-r from-green-900/30 to-emerald-900/30 rounded-xl p-6 border border-green-800">
          <h3 className="text-lg font-semibold mb-2">💡 Optimization Tips</h3>
          <ul className="space-y-2 text-gray-300">
            <li>• Use Sonnet for routine tasks (-70% cost)</li>
            <li>• Batch similar requests together</li>
            <li>• Keep context focused (less = cheaper)</li>
            <li>• Big builds on dedicated sessions</li>
          </ul>
        </div>
      </div>
    </div>
  );
}
