import { useState, useEffect } from 'react';
import { useRouter } from 'next/router';
import { getUserData, getApiKeys, createApiKey, deleteApiKey, getUsageStats } from '../utils/api';
import Head from 'next/head';

export default function Dashboard() {
  const [user, setUser] = useState(null);
  const [apiKeys, setApiKeys] = useState([]);
  const [usageStats, setUsageStats] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showCreateKey, setShowCreateKey] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [creatingKey, setCreatingKey] = useState(false);
  const router = useRouter();

  const proxyEndpoint = process.env.NEXT_PUBLIC_PROXY_ENDPOINT || 'localhost';
  const proxyHttpPort = process.env.NEXT_PUBLIC_PROXY_HTTP_PORT || '7777';
  const proxySocksPort = process.env.NEXT_PUBLIC_PROXY_SOCKS_PORT || '1080';

  useEffect(() => {
    const token = localStorage.getItem('iploop_token');
    if (!token) {
      router.push('/login');
      return;
    }
    loadDashboardData();
  }, [router]);

  const loadDashboardData = async () => {
    try {
      const [userData, keysData, statsData] = await Promise.all([
        getUserData(),
        getApiKeys(),
        getUsageStats()
      ]);
      
      setUser(userData);
      setApiKeys(keysData);
      setUsageStats(statsData);
    } catch (err) {
      if (err.message.includes('401') || err.message.includes('Unauthorized')) {
        localStorage.removeItem('iploop_token');
        router.push('/login');
      } else {
        setError('Failed to load dashboard data');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleCreateApiKey = async (e) => {
    e.preventDefault();
    if (!newKeyName.trim()) return;
    
    setCreatingKey(true);
    try {
      const newKey = await createApiKey(newKeyName.trim());
      setApiKeys([...apiKeys, newKey]);
      setNewKeyName('');
      setShowCreateKey(false);
    } catch (err) {
      setError('Failed to create API key');
    } finally {
      setCreatingKey(false);
    }
  };

  const handleDeleteApiKey = async (keyId) => {
    if (!confirm('Are you sure you want to delete this API key?')) return;
    
    try {
      await deleteApiKey(keyId);
      setApiKeys(apiKeys.filter(key => key.id !== keyId));
    } catch (err) {
      setError('Failed to delete API key');
    }
  };

  const handleLogout = () => {
    localStorage.removeItem('iploop_token');
    router.push('/login');
  };

  if (loading) {
    return (
      <div style={{
        minHeight: '100vh',
        backgroundColor: '#111827',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center'
      }}>
        <div style={{ textAlign: 'center' }}>
          <div style={{
            width: '48px',
            height: '48px',
            border: '2px solid #3b82f6',
            borderTop: '2px solid transparent',
            borderRadius: '50%',
            animation: 'spin 1s linear infinite',
            margin: '0 auto'
          }}></div>
          <p style={{ marginTop: '16px', color: '#9ca3af' }}>Loading dashboard...</p>
        </div>
        <style jsx>{`
          @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
          }
        `}</style>
      </div>
    );
  }

  return (
    <>
      <Head>
        <title>Dashboard - IPLoop</title>
      </Head>
      <div style={{ minHeight: '100vh', backgroundColor: '#111827' }}>
        {/* Header */}
        <header style={{
          backgroundColor: '#1f2937',
          borderBottom: '1px solid #374151',
          padding: '16px'
        }}>
          <div style={{
            maxWidth: '1200px',
            margin: '0 auto',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center'
          }}>
            <div style={{ display: 'flex', alignItems: 'center' }}>
              <h1 style={{ fontSize: '24px', fontWeight: 'bold', color: '#3b82f6', margin: 0 }}>
                IPLoop
              </h1>
              <span style={{ marginLeft: '12px', color: '#9ca3af' }}>Dashboard</span>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
              <span style={{ color: '#d1d5db' }}>
                Welcome, {user?.firstName || user?.email}
              </span>
              <button
                onClick={handleLogout}
                style={{
                  color: '#d1d5db',
                  background: 'none',
                  border: '1px solid #4b5563',
                  padding: '8px 12px',
                  borderRadius: '6px',
                  cursor: 'pointer'
                }}
              >
                Logout
              </button>
            </div>
          </div>
        </header>

        <main style={{ maxWidth: '1200px', margin: '24px auto', padding: '0 16px' }}>
          {error && (
            <div style={{
              marginBottom: '24px',
              backgroundColor: '#7f1d1d',
              border: '1px solid #dc2626',
              color: '#fecaca',
              padding: '12px',
              borderRadius: '8px'
            }}>
              {error}
            </div>
          )}

          {/* Usage Stats */}
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))',
            gap: '24px',
            marginBottom: '32px'
          }}>
            <div style={{
              backgroundColor: '#1f2937',
              border: '1px solid #374151',
              borderRadius: '8px',
              padding: '24px'
            }}>
              <h3 style={{ color: 'white', fontSize: '18px', marginBottom: '8px' }}>Data Used</h3>
              <p style={{ color: '#3b82f6', fontSize: '32px', fontWeight: 'bold', margin: '0' }}>
                {usageStats?.totalGB?.toFixed(2) || '0.00'} GB
              </p>
              <p style={{ color: '#9ca3af', fontSize: '14px', margin: '4px 0 0 0' }}>This month</p>
            </div>
            <div style={{
              backgroundColor: '#1f2937',
              border: '1px solid #374151',
              borderRadius: '8px',
              padding: '24px'
            }}>
              <h3 style={{ color: 'white', fontSize: '18px', marginBottom: '8px' }}>Remaining Balance</h3>
              <p style={{ color: '#10b981', fontSize: '32px', fontWeight: 'bold', margin: '0' }}>
                {usageStats?.remainingGB?.toFixed(2) || '0.00'} GB
              </p>
              <p style={{ color: '#9ca3af', fontSize: '14px', margin: '4px 0 0 0' }}>Available</p>
            </div>
            <div style={{
              backgroundColor: '#1f2937',
              border: '1px solid #374151',
              borderRadius: '8px',
              padding: '24px'
            }}>
              <h3 style={{ color: 'white', fontSize: '18px', marginBottom: '8px' }}>Requests</h3>
              <p style={{ color: '#8b5cf6', fontSize: '32px', fontWeight: 'bold', margin: '0' }}>
                {usageStats?.totalRequests?.toLocaleString() || '0'}
              </p>
              <p style={{ color: '#9ca3af', fontSize: '14px', margin: '4px 0 0 0' }}>This month</p>
            </div>
          </div>

          {/* Proxy Endpoints */}
          <div style={{
            backgroundColor: '#1f2937',
            border: '1px solid #374151',
            borderRadius: '8px',
            padding: '24px',
            marginBottom: '32px'
          }}>
            <h2 style={{ color: 'white', fontSize: '20px', marginBottom: '16px' }}>Proxy Endpoints</h2>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: '24px' }}>
              <div>
                <h3 style={{ color: 'white', fontSize: '16px', marginBottom: '8px' }}>HTTP Proxy</h3>
                <div style={{
                  backgroundColor: '#111827',
                  borderRadius: '6px',
                  padding: '12px',
                  fontFamily: 'monospace',
                  fontSize: '14px'
                }}>
                  <p style={{ color: '#10b981', margin: '0 0 4px 0' }}>Host: {proxyEndpoint}</p>
                  <p style={{ color: '#10b981', margin: '0 0 8px 0' }}>Port: {proxyHttpPort}</p>
                  <p style={{ color: '#9ca3af', margin: '0' }}>
                    Format: http://apikey:@{proxyEndpoint}:{proxyHttpPort}
                  </p>
                </div>
              </div>
              <div>
                <h3 style={{ color: 'white', fontSize: '16px', marginBottom: '8px' }}>SOCKS5 Proxy</h3>
                <div style={{
                  backgroundColor: '#111827',
                  borderRadius: '6px',
                  padding: '12px',
                  fontFamily: 'monospace',
                  fontSize: '14px'
                }}>
                  <p style={{ color: '#10b981', margin: '0 0 4px 0' }}>Host: {proxyEndpoint}</p>
                  <p style={{ color: '#10b981', margin: '0 0 8px 0' }}>Port: {proxySocksPort}</p>
                  <p style={{ color: '#9ca3af', margin: '0' }}>
                    Username: apikey, Password: [empty]
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* API Keys */}
          <div style={{
            backgroundColor: '#1f2937',
            border: '1px solid #374151',
            borderRadius: '8px',
            padding: '24px'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' }}>
              <h2 style={{ color: 'white', fontSize: '20px', margin: 0 }}>API Keys</h2>
              <button
                onClick={() => setShowCreateKey(true)}
                style={{
                  backgroundColor: '#3b82f6',
                  color: 'white',
                  border: 'none',
                  padding: '8px 16px',
                  borderRadius: '6px',
                  fontSize: '14px',
                  fontWeight: '500',
                  cursor: 'pointer'
                }}
              >
                Create API Key
              </button>
            </div>

            {showCreateKey && (
              <form onSubmit={handleCreateApiKey} style={{
                marginBottom: '24px',
                padding: '16px',
                backgroundColor: '#111827',
                borderRadius: '6px',
                border: '1px solid #374151'
              }}>
                <div style={{ display: 'flex', gap: '12px' }}>
                  <input
                    type="text"
                    placeholder="API Key Name"
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                    style={{
                      flex: 1,
                      padding: '8px 12px',
                      backgroundColor: '#1f2937',
                      border: '1px solid #374151',
                      borderRadius: '6px',
                      color: 'white',
                      fontSize: '14px'
                    }}
                    required
                  />
                  <button
                    type="submit"
                    disabled={creatingKey}
                    style={{
                      backgroundColor: '#10b981',
                      color: 'white',
                      border: 'none',
                      padding: '8px 16px',
                      borderRadius: '6px',
                      fontSize: '14px',
                      fontWeight: '500',
                      cursor: creatingKey ? 'not-allowed' : 'pointer',
                      opacity: creatingKey ? 0.5 : 1
                    }}
                  >
                    {creatingKey ? 'Creating...' : 'Create'}
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      setShowCreateKey(false);
                      setNewKeyName('');
                    }}
                    style={{
                      backgroundColor: '#4b5563',
                      color: 'white',
                      border: 'none',
                      padding: '8px 16px',
                      borderRadius: '6px',
                      fontSize: '14px',
                      fontWeight: '500',
                      cursor: 'pointer'
                    }}
                  >
                    Cancel
                  </button>
                </div>
              </form>
            )}

            <div>
              {apiKeys.length === 0 ? (
                <p style={{
                  color: '#9ca3af',
                  textAlign: 'center',
                  padding: '32px',
                  margin: 0
                }}>
                  No API keys found. Create your first API key to get started.
                </p>
              ) : (
                apiKeys.map((key) => (
                  <div key={key.id} style={{
                    backgroundColor: '#111827',
                    border: '1px solid #374151',
                    borderRadius: '6px',
                    padding: '16px',
                    marginBottom: '16px'
                  }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                      <div style={{ flex: 1 }}>
                        <h4 style={{ color: 'white', fontSize: '16px', margin: '0 0 8px 0' }}>{key.name}</h4>
                        <p style={{ color: '#9ca3af', fontSize: '14px', margin: '0 0 4px 0' }}>
                          Created: {new Date(key.created_at).toLocaleDateString()}
                        </p>
                        {key.last_used_at && (
                          <p style={{ color: '#9ca3af', fontSize: '14px', margin: '0 0 8px 0' }}>
                            Last used: {new Date(key.last_used_at).toLocaleDateString()}
                          </p>
                        )}
                        {key.key && (
                          <div style={{ marginTop: '8px' }}>
                            <p style={{ color: '#6b7280', fontSize: '12px', margin: '0 0 4px 0' }}>
                              API Key (save this, it won't be shown again):
                            </p>
                            <code style={{
                              backgroundColor: '#1f2937',
                              padding: '4px 8px',
                              borderRadius: '4px',
                              fontSize: '14px',
                              color: '#10b981',
                              wordBreak: 'break-all'
                            }}>
                              {key.key}
                            </code>
                          </div>
                        )}
                      </div>
                      <div style={{ display: 'flex', gap: '8px', marginLeft: '16px' }}>
                        <span style={{
                          padding: '4px 8px',
                          borderRadius: '12px',
                          fontSize: '12px',
                          backgroundColor: key.is_active ? '#064e3b' : '#7f1d1d',
                          color: key.is_active ? '#6ee7b7' : '#fecaca',
                          border: key.is_active ? '1px solid #059669' : '1px solid #dc2626'
                        }}>
                          {key.is_active ? 'Active' : 'Disabled'}
                        </span>
                        <button
                          onClick={() => handleDeleteApiKey(key.id)}
                          style={{
                            color: '#f87171',
                            background: 'none',
                            border: '1px solid #dc2626',
                            padding: '4px 8px',
                            borderRadius: '4px',
                            fontSize: '12px',
                            cursor: 'pointer'
                          }}
                        >
                          Delete
                        </button>
                      </div>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        </main>
      </div>
    </>
  );
}