import { NextRequest, NextResponse } from 'next/server'

// Knowledge base for IPLoop - can be expanded
const knowledgeBase: Record<string, string> = {
  'proxy': `IPLoop offers residential proxy endpoints that route through our network of mobile and residential nodes. 

To set up a proxy:
1. Go to API Keys and generate a new key
2. Navigate to Proxy Endpoints to see your secure endpoint URL
3. Use the endpoint URL with your API key for authentication

Supported protocols: HTTP, HTTPS, SOCKS5
Authentication: API key as username (password can be empty)
Security: All connections through encrypted HTTPS tunnels`,

  'endpoint': `Proxy endpoints are access points to our residential network.

Your proxy endpoint URL is shown in the Proxy Endpoints page after you generate an API key. All connections go through secure HTTPS tunnels.

Connection example:
curl -x http://YOUR_API_KEY:@your-endpoint-url https://example.com

Features:
- Automatic IP rotation
- Sticky sessions (up to 30 min)
- Country/city targeting
- All traffic encrypted via HTTPS`,

  'node': `Nodes are devices that contribute to the IPLoop network.

To add a new node:
1. Download the IPLoop app on your Android device
2. Sign in with your account
3. The device will automatically register as a node

Node earnings:
- Earn based on bandwidth shared
- Higher quality connections = better rates
- View earnings in the Billing section`,

  'bandwidth': `Your bandwidth usage is tracked in real-time.

View detailed analytics:
- Go to Analytics for usage charts
- Dashboard shows current bandwidth used
- Set up webhooks for usage alerts

Billing is based on:
- GB of data transferred through proxies
- Premium features (dedicated IPs, etc.)`,

  'billing': `IPLoop uses a pay-as-you-go model with free tier included.

Pricing tiers:
- Free: 1GB/month, shared pool
- Starter ($29/mo): 10GB, priority support
- Pro ($99/mo): 50GB, dedicated endpoints
- Enterprise: Custom pricing

Node earnings:
- Residential: $0.10-0.30 per GB
- Mobile: $0.20-0.50 per GB
- Payouts: Monthly, minimum $10`,

  'webhook': `Webhooks notify your server about events in real-time.

Supported events:
- node.connected / node.disconnected
- usage.threshold (customizable)
- payment.received
- api_key.created / api_key.revoked

Setup:
1. Go to Webhooks page
2. Add endpoint URL
3. Select events to subscribe
4. Save and test

Webhooks include HMAC signature for verification.`,

  'troubleshoot': `Common connection issues and solutions:

1. Connection refused:
   - Check API key is valid
   - Verify endpoint URL is correct
   - Ensure your IP isn't blocked

2. Slow speeds:
   - Try different region endpoints
   - Check your local network
   - Contact support for dedicated pool

3. IP blocked:
   - Enable auto-rotation
   - Use sticky sessions sparingly
   - Try different country targeting

Need more help? Contact support@iploop.io`
}

// Simple keyword matching for responses
function findResponse(message: string): string {
  const lowerMessage = message.toLowerCase()
  
  // Check for specific topics
  if (lowerMessage.includes('proxy') || lowerMessage.includes('endpoint') || lowerMessage.includes('set up')) {
    if (lowerMessage.includes('endpoint')) {
      return knowledgeBase['endpoint']
    }
    return knowledgeBase['proxy']
  }
  
  if (lowerMessage.includes('node') || lowerMessage.includes('device') || lowerMessage.includes('add')) {
    return knowledgeBase['node']
  }
  
  if (lowerMessage.includes('bandwidth') || lowerMessage.includes('usage') || lowerMessage.includes('analytics')) {
    return knowledgeBase['bandwidth']
  }
  
  if (lowerMessage.includes('billing') || lowerMessage.includes('price') || lowerMessage.includes('pricing') || lowerMessage.includes('cost') || lowerMessage.includes('pay') || lowerMessage.includes('tier')) {
    return knowledgeBase['billing']
  }
  
  if (lowerMessage.includes('webhook') || lowerMessage.includes('notification') || lowerMessage.includes('alert')) {
    return knowledgeBase['webhook']
  }
  
  if (lowerMessage.includes('troubleshoot') || lowerMessage.includes('problem') || lowerMessage.includes('issue') || lowerMessage.includes('error') || lowerMessage.includes('not working') || lowerMessage.includes('connection')) {
    return knowledgeBase['troubleshoot']
  }
  
  if (lowerMessage.includes('hello') || lowerMessage.includes('hi') || lowerMessage.includes('hey')) {
    return `Hello! 👋 I'm the IPLoop AI Assistant. I can help you with:

• Setting up proxy endpoints
• Managing nodes and devices
• Understanding billing and pricing
• Troubleshooting connection issues
• Configuring webhooks

What would you like to know about?`
  }
  
  if (lowerMessage.includes('thank')) {
    return `You're welcome! 😊 Is there anything else I can help you with?`
  }
  
  // Default response
  return `I'm not sure I understand your question completely. Here are some topics I can help with:

• **Proxy setup** - How to configure and use proxy endpoints
• **Nodes** - Adding devices to the network
• **Billing** - Pricing tiers and earnings
• **Webhooks** - Setting up notifications
• **Troubleshooting** - Fixing common issues

Could you rephrase your question or pick one of these topics?`
}

export async function POST(request: NextRequest) {
  try {
    // Verify auth token
    const authHeader = request.headers.get('Authorization')
    if (!authHeader || !authHeader.startsWith('Bearer ')) {
      return NextResponse.json({ error: 'Unauthorized' }, { status: 401 })
    }
    
    const body = await request.json()
    const { message, sessionId } = body
    
    if (!message || typeof message !== 'string') {
      return NextResponse.json({ error: 'Message is required' }, { status: 400 })
    }
    
    // Generate response
    const response = findResponse(message)
    
    // Simulate some processing time for realism
    await new Promise(resolve => setTimeout(resolve, 500 + Math.random() * 1000))
    
    return NextResponse.json({
      response,
      sessionId
    })
  } catch (error) {
    console.error('AI chat error:', error)
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    )
  }
}
