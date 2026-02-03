// API Configuration
const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8002';

// Helper function to get auth token
const getAuthToken = () => {
  if (typeof window !== 'undefined') {
    return localStorage.getItem('iploop_token');
  }
  return null;
};

// Helper function to make authenticated requests
async function apiRequest(endpoint, options = {}) {
  const token = getAuthToken();
  const url = `${API_BASE_URL}${endpoint}`;
  
  const defaultHeaders = {
    'Content-Type': 'application/json',
  };
  
  if (token) {
    defaultHeaders['Authorization'] = `Bearer ${token}`;
  }
  
  const config = {
    ...options,
    headers: {
      ...defaultHeaders,
      ...options.headers,
    },
  };
  
  try {
    const response = await fetch(url, config);
    
    // Handle non-JSON responses (like health checks)
    const contentType = response.headers.get('content-type');
    let data;
    
    if (contentType && contentType.includes('application/json')) {
      data = await response.json();
    } else {
      data = await response.text();
    }
    
    if (!response.ok) {
      throw new Error(data.error || data.message || `HTTP error! status: ${response.status}`);
    }
    
    return data;
  } catch (error) {
    console.error('API Request Error:', error);
    throw error;
  }
}

// Authentication functions
export async function login(email, password) {
  return await apiRequest('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export async function register(userData) {
  return await apiRequest('/auth/register', {
    method: 'POST',
    body: JSON.stringify({
      email: userData.email,
      password: userData.password,
      first_name: userData.firstName,
      last_name: userData.lastName,
      company: userData.company || null,
    }),
  });
}

export async function logout() {
  try {
    await apiRequest('/auth/logout', {
      method: 'POST',
    });
  } catch (error) {
    // Continue with logout even if API call fails
    console.warn('Logout API call failed:', error);
  }
  
  // Remove token from localStorage
  if (typeof window !== 'undefined') {
    localStorage.removeItem('iploop_token');
  }
}

// User data functions
export async function getUserData() {
  return await apiRequest('/user/profile');
}

export async function updateUserProfile(profileData) {
  return await apiRequest('/user/profile', {
    method: 'PUT',
    body: JSON.stringify(profileData),
  });
}

// API Key functions
export async function getApiKeys() {
  return await apiRequest('/user/api-keys');
}

export async function createApiKey(name, permissions = ['proxy']) {
  return await apiRequest('/user/api-keys', {
    method: 'POST',
    body: JSON.stringify({ name, permissions }),
  });
}

export async function deleteApiKey(keyId) {
  return await apiRequest(`/user/api-keys/${keyId}`, {
    method: 'DELETE',
  });
}

export async function updateApiKey(keyId, updateData) {
  return await apiRequest(`/user/api-keys/${keyId}`, {
    method: 'PUT',
    body: JSON.stringify(updateData),
  });
}

// Usage and billing functions
export async function getUsageStats() {
  try {
    return await apiRequest('/user/usage');
  } catch (error) {
    // Return default stats if API fails
    console.warn('Failed to fetch usage stats:', error);
    return {
      totalGB: 0,
      remainingGB: 0,
      totalRequests: 0,
      thisMonth: {
        gb: 0,
        requests: 0,
      },
    };
  }
}

export async function getUsageHistory(startDate, endDate) {
  const params = new URLSearchParams();
  if (startDate) params.append('start_date', startDate);
  if (endDate) params.append('end_date', endDate);
  
  const query = params.toString();
  return await apiRequest(`/user/usage/history${query ? `?${query}` : ''}`);
}

export async function getBillingTransactions() {
  return await apiRequest('/user/billing/transactions');
}

// Proxy and nodes functions
export async function getAvailableCountries() {
  try {
    return await apiRequest('/proxy/countries');
  } catch (error) {
    console.warn('Failed to fetch countries:', error);
    return [];
  }
}

export async function getProxyHealth() {
  try {
    return await apiRequest('/proxy/health');
  } catch (error) {
    console.warn('Failed to fetch proxy health:', error);
    return { status: 'unknown', nodes: 0 };
  }
}

// Plans and subscription functions
export async function getAvailablePlans() {
  return await apiRequest('/plans');
}

export async function getCurrentPlan() {
  return await apiRequest('/user/plan');
}

export async function upgradePlan(planId) {
  return await apiRequest('/user/plan', {
    method: 'PUT',
    body: JSON.stringify({ plan_id: planId }),
  });
}

// Health check function
export async function checkApiHealth() {
  try {
    const response = await apiRequest('/health');
    return { status: 'healthy', message: response };
  } catch (error) {
    return { status: 'unhealthy', error: error.message };
  }
}

// Utility functions
export function formatBytes(bytes) {
  if (bytes === 0) return '0 Bytes';
  
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
  
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

export function formatDate(dateString) {
  const date = new Date(dateString);
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function generateProxyUrl(apiKey, host, port, protocol = 'http') {
  if (protocol === 'socks5') {
    return `socks5://${apiKey}:@${host}:${port}`;
  }
  return `${protocol}://${apiKey}:@${host}:${port}`;
}

// Error handling utility
export function handleApiError(error) {
  if (error.message.includes('401') || error.message.includes('Unauthorized')) {
    if (typeof window !== 'undefined') {
      localStorage.removeItem('iploop_token');
      window.location.href = '/login';
    }
    return 'Session expired. Please log in again.';
  }
  
  return error.message || 'An unexpected error occurred';
}