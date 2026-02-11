import { NextResponse } from 'next/server'

// Simple support bot responses - can be enhanced with AI later
const KNOWLEDGE_BASE = {
  greeting: [
    "Hi! I'm here to help with IPLoop. What can I assist you with?",
    "Hello! How can I help you with IPLoop today?"
  ],
  
  getStarted: `Here's how to get started with IPLoop:

1. **Create an API Key**
   Go to API Keys in your dashboard and click "Create API Key"

2. **Make your first request**
   \`\`\`bash
   curl -x http://user:YOUR_API_KEY@proxy.iploop.io:7777 https://httpbin.org/ip
   \`\`\`

3. **Check the response**
   You should see a different IP address - that's your residential proxy working!

Need help with a specific programming language? Just ask!`,

  countryTargeting: `To target a specific country, add \`-country-XX\` to your API key:

**Examples:**
- US: \`YOUR_API_KEY-country-US\`
- Israel: \`YOUR_API_KEY-country-IL\`
- Germany: \`YOUR_API_KEY-country-DE\`
- UK: \`YOUR_API_KEY-country-UK\`

**Usage:**
\`\`\`bash
curl -x http://user:YOUR_API_KEY-country-US@proxy.iploop.io:7777 https://httpbin.org/ip
\`\`\`

Available countries depend on our current node network. Check the Nodes page to see active regions.`,

  error407: `A **407 Proxy Authentication Required** error means there's an issue with your credentials.

**Common causes:**
1. **Wrong API key** - Copy it again from the API Keys page
2. **Key disabled** - Check if your key is active in the dashboard
3. **Format issue** - Make sure the format is \`user:API_KEY@proxy.iploop.io:7777\`

**Try this:**
1. Go to API Keys page
2. Create a new key or copy an existing one
3. Test with: \`curl -x http://test:YOUR_KEY@proxy.iploop.io:7777 https://httpbin.org/ip\`

Still having issues? Email support@iploop.io with your error details.`,

  createApiKey: `To create an API key:

1. Go to **API Keys** in the sidebar
2. Click **"Create API Key"**
3. Enter a name (e.g., "Production", "Testing")
4. Click **Create**
5. **Copy the key immediately** - it's only shown once!

Your key will look like: \`iploop_abc123...\`

Use it in requests like:
\`\`\`
http://user:iploop_abc123@proxy.iploop.io:7777
\`\`\``,

  python: `**Python Integration:**

\`\`\`python
import requests

API_KEY = "your_api_key_here"

proxies = {
    'http': f'http://user:{API_KEY}@proxy.iploop.io:7777',
    'https': f'http://user:{API_KEY}@proxy.iploop.io:7777'
}

# Make a request
response = requests.get('https://httpbin.org/ip', proxies=proxies)
print(response.json())

# With country targeting
proxies_us = {
    'http': f'http://user:{API_KEY}-country-US@proxy.iploop.io:7777',
    'https': f'http://user:{API_KEY}-country-US@proxy.iploop.io:7777'
}
\`\`\``,

  nodejs: `**Node.js Integration:**

\`\`\`javascript
const axios = require('axios');

const API_KEY = 'your_api_key_here';

const proxy = {
  host: 'proxy.iploop.io',
  port: 7777,
  auth: {
    username: 'user',
    password: API_KEY
  }
};

// Make a request
axios.get('https://httpbin.org/ip', { proxy })
  .then(res => console.log(res.data))
  .catch(err => console.error(err));
\`\`\``,

  billing: `For billing questions:

- View your current plan: **Billing** page in dashboard
- Upgrade your plan: Click "Upgrade" on the Billing page
- Payment issues: Email **billing@iploop.io**
- Invoices: Available in the **Invoices** tab

Need to cancel or change your subscription? Contact billing@iploop.io`,

  default: `I'm not sure I understood that question. Here are some things I can help with:

• **Getting started** - How to make your first proxy request
• **API keys** - Creating and managing authentication
• **Country targeting** - How to use specific regions
• **Error troubleshooting** - 407 errors, timeouts, etc.
• **Code examples** - Python, Node.js, cURL, etc.

Could you rephrase your question? Or email support@iploop.io for detailed help.`
}

function getResponse(message: string): string {
  const msg = message.toLowerCase()
  
  // Greetings
  if (msg.match(/^(hi|hello|hey|good morning|good afternoon)/)) {
    return KNOWLEDGE_BASE.greeting[Math.floor(Math.random() * KNOWLEDGE_BASE.greeting.length)]
  }
  
  // Getting started
  if (msg.includes('get started') || msg.includes('start') || msg.includes('begin') || msg.includes('how do i use')) {
    return KNOWLEDGE_BASE.getStarted
  }
  
  // Country targeting
  if (msg.includes('country') || msg.includes('target') || msg.includes('location') || msg.includes('region') || msg.includes('geo')) {
    return KNOWLEDGE_BASE.countryTargeting
  }
  
  // 407 errors
  if (msg.includes('407') || msg.includes('authentication') || msg.includes('auth error') || msg.includes('unauthorized')) {
    return KNOWLEDGE_BASE.error407
  }
  
  // API key
  if (msg.includes('api key') || msg.includes('create key') || msg.includes('new key')) {
    return KNOWLEDGE_BASE.createApiKey
  }
  
  // Python
  if (msg.includes('python') || msg.includes('requests')) {
    return KNOWLEDGE_BASE.python
  }
  
  // Node.js
  if (msg.includes('node') || msg.includes('javascript') || msg.includes('axios') || msg.includes('js')) {
    return KNOWLEDGE_BASE.nodejs
  }
  
  // Billing
  if (msg.includes('billing') || msg.includes('payment') || msg.includes('invoice') || msg.includes('plan') || msg.includes('pricing') || msg.includes('upgrade')) {
    return KNOWLEDGE_BASE.billing
  }
  
  // Timeout/connection issues
  if (msg.includes('timeout') || msg.includes('connection') || msg.includes('refused') || msg.includes("can't connect")) {
    return `**Connection Issues:**

1. **Check proxy endpoint**: \`proxy.iploop.io:7777\` (HTTP) or \`:1080\` (SOCKS5)
2. **Verify your API key** is active
3. **Check node availability** - Some regions may have limited nodes

If the issue persists:
- Try a different country with \`-country-XX\`
- Check our status page
- Email support@iploop.io with error details`
  }

  return KNOWLEDGE_BASE.default
}

export async function POST(request: Request) {
  try {
    const { message } = await request.json()
    
    if (!message) {
      return NextResponse.json({ error: 'Message is required' }, { status: 400 })
    }

    const response = getResponse(message)
    
    return NextResponse.json({ response })
  } catch (error) {
    console.error('Support chat error:', error)
    return NextResponse.json({ 
      response: "I'm having trouble right now. Please email support@iploop.io for assistance." 
    })
  }
}
