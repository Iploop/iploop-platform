import { NextResponse } from 'next/server'

const API_URL = process.env.DATABASE_URL ? 'postgresql' : 'http://localhost:5432'

// For now, we'll query the database directly via node-registration's API
// In production, this would go through customer-api

export async function GET() {
  try {
    // Fetch from node-registration which has DB access
    // For MVP, return mock + real key we created
    const keys = [
      {
        id: '4c3e802c-1cf7-4e73-9a00-15a64cfa067b',
        name: 'Test Proxy Key',
        key_preview: 'testkey123',
        permissions: ['proxy:read', 'proxy:write'],
        is_active: true,
        last_used: new Date().toISOString(),
        created_at: '2026-02-03T10:00:00Z',
      }
    ]
    
    return NextResponse.json({
      keys,
      count: keys.length,
    })
  } catch (error) {
    console.error('Error fetching keys:', error)
    return NextResponse.json({ error: 'Failed to fetch API keys', keys: [] }, { status: 500 })
  }
}

export async function POST(request: Request) {
  try {
    const body = await request.json()
    const { name } = body
    
    // Generate a new API key
    const newKey = {
      id: crypto.randomUUID(),
      name: name || 'New API Key',
      key: `iploop_${crypto.randomUUID().replace(/-/g, '').substring(0, 32)}`,
      permissions: ['proxy:read', 'proxy:write'],
      is_active: true,
      created_at: new Date().toISOString(),
    }
    
    // In production, this would save to the database
    // For now, just return the generated key
    
    return NextResponse.json({
      success: true,
      key: newKey,
      message: 'API key created. Save this key - it won\'t be shown again!',
    })
  } catch (error) {
    console.error('Error creating key:', error)
    return NextResponse.json({ error: 'Failed to create API key' }, { status: 500 })
  }
}
