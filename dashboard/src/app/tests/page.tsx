'use client';

import { useState, useEffect } from 'react';

interface TestSummary {
  timestamp: string;
  passed: boolean;
  duration: number;
  summary: Record<string, number>;
}

interface LatestResults {
  [testName: string]: TestSummary;
}

interface HistoricalResult {
  testName: string;
  timestamp: string;
  duration: number;
  passed: boolean;
  summary: Record<string, number>;
}

const TEST_CATEGORIES = {
  'Quick Tests': ['stress-smoke', 'leak', 'protocol'],
  'Stress Tests': ['stress-ramp', 'stress-sustained', 'stress-burst'],
  'Feature Tests': ['geo', 'sticky', 'rotation', 'peer'],
  'Security Tests': ['security', 'auth'],
  'Protocol & Reliability': ['bandwidth', 'failure', 'latency', 'connection', 'concurrency'],
  'Long Running': ['stability', 'scenario']
};

export default function TestDashboard() {
  const [latest, setLatest] = useState<LatestResults>({});
  const [history, setHistory] = useState<Record<string, HistoricalResult[]>>({});
  const [selectedTest, setSelectedTest] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [running, setRunning] = useState<string | null>(null);
  const [liveOutput, setLiveOutput] = useState<string>('');

  useEffect(() => {
    fetchResults();
    const interval = setInterval(fetchResults, 5000); // Poll every 5s
    return () => clearInterval(interval);
  }, []);

  const fetchResults = async () => {
    try {
      const res = await fetch('/api/tests');
      const data = await res.json();
      setLatest(data.latest || {});
      setHistory(data.history || {});
      setLoading(false);
    } catch (e) {
      console.error('Failed to fetch results:', e);
      setLoading(false);
    }
  };

  const runTest = async (testName: string) => {
    setRunning(testName);
    setLiveOutput('Starting test...\n');
    
    try {
      const res = await fetch('/api/tests/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ test: testName })
      });
      
      const reader = res.body?.getReader();
      const decoder = new TextDecoder();
      
      while (reader) {
        const { done, value } = await reader.read();
        if (done) break;
        setLiveOutput(prev => prev + decoder.decode(value));
      }
    } catch (e) {
      setLiveOutput(prev => prev + `\nError: ${e}\n`);
    } finally {
      setRunning(null);
      fetchResults();
    }
  };

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
    return `${(ms / 60000).toFixed(1)}m`;
  };

  const formatTime = (iso: string) => {
    const d = new Date(iso);
    return d.toLocaleString();
  };

  const getStatusColor = (passed: boolean) => {
    return passed ? 'text-green-500' : 'text-red-500';
  };

  const getChangeIndicator = (current: number, previous: number | undefined, inverse = false) => {
    if (previous === undefined) return '';
    const change = current - previous;
    const pct = previous !== 0 ? ((change / previous) * 100) : 0;
    const isGood = inverse ? change < 0 : change > 0;
    
    if (Math.abs(pct) < 1) return '';
    return (
      <span className={`text-sm ml-2 ${isGood ? 'text-green-400' : 'text-red-400'}`}>
        {change > 0 ? '↑' : '↓'} {Math.abs(pct).toFixed(1)}%
      </span>
    );
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-900 text-white flex items-center justify-center">
        <div className="text-xl">Loading test results...</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-900 text-white p-8">
      <div className="max-w-7xl mx-auto">
        <header className="mb-8">
          <h1 className="text-3xl font-bold mb-2">🧪 IPLoop Test Dashboard</h1>
          <p className="text-gray-400">Live test results with historical comparison</p>
        </header>

        {/* Summary Cards */}
        <div className="grid grid-cols-4 gap-4 mb-8">
          <div className="bg-gray-800 rounded-lg p-4">
            <div className="text-gray-400 text-sm">Total Tests</div>
            <div className="text-2xl font-bold">{Object.keys(latest).length}</div>
          </div>
          <div className="bg-gray-800 rounded-lg p-4">
            <div className="text-gray-400 text-sm">Passing</div>
            <div className="text-2xl font-bold text-green-500">
              {Object.values(latest).filter(t => t.passed).length}
            </div>
          </div>
          <div className="bg-gray-800 rounded-lg p-4">
            <div className="text-gray-400 text-sm">Failing</div>
            <div className="text-2xl font-bold text-red-500">
              {Object.values(latest).filter(t => !t.passed).length}
            </div>
          </div>
          <div className="bg-gray-800 rounded-lg p-4">
            <div className="text-gray-400 text-sm">Last Run</div>
            <div className="text-lg">
              {Object.values(latest).length > 0
                ? formatTime(Object.values(latest).sort((a, b) => 
                    new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
                  )[0].timestamp)
                : 'Never'}
            </div>
          </div>
        </div>

        {/* Test Categories */}
        {Object.entries(TEST_CATEGORIES).map(([category, tests]) => (
          <div key={category} className="mb-8">
            <h2 className="text-xl font-semibold mb-4 text-gray-300">{category}</h2>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {tests.map(testName => {
                const result = latest[testName];
                const testHistory = history[testName] || [];
                const previous = testHistory[1]; // Second most recent is previous

                return (
                  <div
                    key={testName}
                    className={`bg-gray-800 rounded-lg p-4 cursor-pointer hover:bg-gray-700 transition ${
                      selectedTest === testName ? 'ring-2 ring-blue-500' : ''
                    }`}
                    onClick={() => setSelectedTest(selectedTest === testName ? null : testName)}
                  >
                    <div className="flex justify-between items-start mb-2">
                      <h3 className="font-medium">{testName}</h3>
                      {result && (
                        <span className={`text-sm font-semibold ${getStatusColor(result.passed)}`}>
                          {result.passed ? '✓ PASS' : '✗ FAIL'}
                        </span>
                      )}
                      {!result && (
                        <span className="text-sm text-gray-500">Not run</span>
                      )}
                    </div>

                    {result && (
                      <div className="text-sm text-gray-400">
                        <div>Duration: {formatDuration(result.duration)}</div>
                        <div>Last run: {formatTime(result.timestamp)}</div>
                        
                        {/* Key Metrics */}
                        {result.summary && Object.entries(result.summary).slice(0, 3).map(([key, value]) => (
                          <div key={key} className="flex items-center">
                            <span className="capitalize">{key.replace(/([A-Z])/g, ' $1')}: </span>
                            <span className="font-medium ml-1">
                              {typeof value === 'number' ? value.toLocaleString() : value}
                            </span>
                            {previous?.summary && getChangeIndicator(
                              value as number,
                              previous.summary[key] as number,
                              key.includes('error') || key.includes('fail') || key.includes('latency')
                            )}
                          </div>
                        ))}
                      </div>
                    )}

                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        runTest(testName);
                      }}
                      disabled={running !== null}
                      className={`mt-3 w-full py-1 px-3 rounded text-sm ${
                        running === testName
                          ? 'bg-yellow-600 animate-pulse'
                          : 'bg-blue-600 hover:bg-blue-500'
                      } disabled:opacity-50`}
                    >
                      {running === testName ? 'Running...' : 'Run Test'}
                    </button>
                  </div>
                );
              })}
            </div>
          </div>
        ))}

        {/* Live Output */}
        {running && (
          <div className="fixed bottom-0 left-0 right-0 bg-gray-950 border-t border-gray-700 p-4">
            <div className="max-w-7xl mx-auto">
              <div className="flex justify-between items-center mb-2">
                <h3 className="font-medium">Running: {running}</h3>
                <button
                  onClick={() => setRunning(null)}
                  className="text-gray-400 hover:text-white"
                >
                  ✕ Close
                </button>
              </div>
              <pre className="bg-black rounded p-3 h-40 overflow-auto text-sm font-mono text-green-400">
                {liveOutput}
              </pre>
            </div>
          </div>
        )}

        {/* Historical Detail Panel */}
        {selectedTest && history[selectedTest] && history[selectedTest].length > 0 && (
          <div className="mt-8 bg-gray-800 rounded-lg p-6">
            <h2 className="text-xl font-semibold mb-4">
              📈 Historical Results: {selectedTest}
            </h2>
            
            {/* Chart placeholder - would use recharts/chart.js in production */}
            <div className="bg-gray-900 rounded-lg p-4 mb-4">
              <div className="flex items-end h-32 gap-1">
                {history[selectedTest].slice(0, 20).reverse().map((result, i) => {
                  const maxDuration = Math.max(...history[selectedTest].map(r => r.duration));
                  const height = (result.duration / maxDuration) * 100;
                  return (
                    <div
                      key={i}
                      className={`flex-1 rounded-t transition-all ${
                        result.passed ? 'bg-green-500' : 'bg-red-500'
                      }`}
                      style={{ height: `${Math.max(height, 5)}%` }}
                      title={`${formatTime(result.timestamp)}: ${formatDuration(result.duration)}`}
                    />
                  );
                })}
              </div>
              <div className="flex justify-between text-xs text-gray-500 mt-2">
                <span>Older</span>
                <span>Recent</span>
              </div>
            </div>

            {/* History Table */}
            <table className="w-full text-sm">
              <thead>
                <tr className="text-gray-400 border-b border-gray-700">
                  <th className="text-left py-2">Timestamp</th>
                  <th className="text-left py-2">Status</th>
                  <th className="text-left py-2">Duration</th>
                  <th className="text-left py-2">Key Metrics</th>
                </tr>
              </thead>
              <tbody>
                {history[selectedTest].slice(0, 10).map((result, i) => (
                  <tr key={i} className="border-b border-gray-700/50">
                    <td className="py-2">{formatTime(result.timestamp)}</td>
                    <td className={`py-2 ${getStatusColor(result.passed)}`}>
                      {result.passed ? 'PASS' : 'FAIL'}
                    </td>
                    <td className="py-2">{formatDuration(result.duration)}</td>
                    <td className="py-2 text-gray-400">
                      {result.summary && Object.entries(result.summary).slice(0, 3).map(([k, v]) => (
                        <span key={k} className="mr-3">
                          {k}: <span className="text-white">{typeof v === 'number' ? v.toLocaleString() : v}</span>
                        </span>
                      ))}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
