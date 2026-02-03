const http = require('http');
const https = require('https');
const WebSocket = require('ws');
const url = require('url');

const PORT = 8765;

// Store connected devices
const devices = new Map();

// Create HTTP server
const server = http.createServer((req, res) => {
  const parsedUrl = url.parse(req.url, true);
  
  // Health check
  if (parsedUrl.pathname === '/health') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ 
      status: 'ok', 
      devices: devices.size,
      deviceList: Array.from(devices.keys())
    }));
    return;
  }
  
  // Proxy request endpoint - send HTTP request through a device
  if (parsedUrl.pathname === '/proxy' && req.method === 'POST') {
    let body = '';
    req.on('data', chunk => body += chunk);
    req.on('end', async () => {
      try {
        const request = JSON.parse(body);
        const result = await sendProxyRequest(request);
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(result));
      } catch (e) {
        res.writeHead(500, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: e.message }));
      }
    });
    return;
  }
  
  res.writeHead(200, { 'Content-Type': 'text/plain' });
  res.end('IPLoop Test Server Running\n');
});

// Create WebSocket server
const wss = new WebSocket.Server({ server });

console.log('IPLoop Test Server starting...');

wss.on('connection', (ws, req) => {
  const clientIP = req.socket.remoteAddress;
  console.log(`[${ts()}] New connection from: ${clientIP}`);
  
  let deviceId = null;
  
  // Pending proxy requests for this device
  const pendingRequests = new Map();
  
  ws.on('message', (data) => {
    try {
      const message = JSON.parse(data);
      console.log(`[${ts()}] Received: ${message.type}`);
      
      switch (message.type) {
        case 'register':
          deviceId = message.device_info?.device_id || `device_${Date.now()}`;
          console.log(`Device registered: ${deviceId}`);
          
          // Store device connection
          devices.set(deviceId, { 
            ws, 
            ip: clientIP, 
            pendingRequests,
            registeredAt: Date.now(),
            lastSeen: Date.now()
          });
          
          ws.send(JSON.stringify({
            type: 'registered',
            status: 'ok',
            node_id: deviceId,
            config: {
              heartbeat_interval: 30000,
              max_bandwidth: 100 * 1024 * 1024
            }
          }));
          break;
          
        case 'heartbeat':
          console.log(`Heartbeat from: ${deviceId}`);
          if (devices.has(deviceId)) {
            devices.get(deviceId).lastSeen = Date.now();
          }
          ws.send(JSON.stringify({
            type: 'heartbeat_ack',
            timestamp: Date.now()
          }));
          break;
          
        case 'proxy_response':
          // Response from device for a proxy request
          const reqId = message.request_id;
          console.log(`[${ts()}] Proxy response for ${reqId}: status=${message.status_code}`);
          
          if (pendingRequests.has(reqId)) {
            const { resolve } = pendingRequests.get(reqId);
            pendingRequests.delete(reqId);
            resolve({
              success: true,
              request_id: reqId,
              status_code: message.status_code,
              headers: message.headers,
              body: message.body,
              device_id: deviceId,
              device_ip: clientIP,
              latency_ms: Date.now() - message.started_at
            });
          }
          break;
          
        case 'proxy_error':
          const errorReqId = message.request_id;
          console.log(`[${ts()}] Proxy error for ${errorReqId}: ${message.error}`);
          
          if (pendingRequests.has(errorReqId)) {
            const { reject } = pendingRequests.get(errorReqId);
            pendingRequests.delete(errorReqId);
            reject(new Error(message.error));
          }
          break;
          
        default:
          console.log('Unknown message:', message);
      }
    } catch (e) {
      console.log('Raw message:', data.toString());
    }
  });
  
  ws.on('close', () => {
    console.log(`[${ts()}] Connection closed: ${deviceId || clientIP}`);
    if (deviceId) {
      devices.delete(deviceId);
    }
  });
  
  ws.on('error', (err) => {
    console.error('WebSocket error:', err.message);
  });
});

// Send proxy request to a device
async function sendProxyRequest(request) {
  const { device_id, url: targetUrl, method = 'GET', headers = {}, body = null, sticky = false } = request;
  
  // Find device
  let device;
  if (device_id && devices.has(device_id)) {
    device = devices.get(device_id);
  } else if (devices.size > 0) {
    // Pick first available device (for non-sticky or unspecified)
    device = devices.values().next().value;
  } else {
    throw new Error('No devices connected');
  }
  
  const requestId = `req_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
  
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      device.pendingRequests.delete(requestId);
      reject(new Error('Proxy request timeout'));
    }, 30000);
    
    device.pendingRequests.set(requestId, { 
      resolve: (result) => {
        clearTimeout(timeout);
        resolve(result);
      }, 
      reject: (error) => {
        clearTimeout(timeout);
        reject(error);
      }
    });
    
    // Send request to device
    const proxyRequest = {
      type: 'proxy_request',
      request_id: requestId,
      url: targetUrl,
      method,
      headers,
      body,
      sticky,
      timestamp: Date.now()
    };
    
    console.log(`[${ts()}] Sending proxy request ${requestId} to device: ${method} ${targetUrl}`);
    device.ws.send(JSON.stringify(proxyRequest));
  });
}

function ts() {
  return new Date().toISOString();
}

server.listen(PORT, '0.0.0.0', () => {
  console.log(`✅ IPLoop Test Server running on port ${PORT}`);
  console.log(`WebSocket: ws://0.0.0.0:${PORT}`);
  console.log(`HTTP API: http://0.0.0.0:${PORT}`);
  console.log(`  GET  /health - Check status and connected devices`);
  console.log(`  POST /proxy  - Send proxy request through device`);
});
