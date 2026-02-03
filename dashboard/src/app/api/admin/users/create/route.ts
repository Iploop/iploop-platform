import { NextResponse } from 'next/server'

const CUSTOMER_API_URL = process.env.CUSTOMER_API_URL || 'http://localhost:8002'

export async function POST(request: Request) {
  try {
    const authHeader = request.headers.get('Authorization')
    
    if (!authHeader) {
      return NextResponse.json({ error: 'Authorization required' }, { status: 401 })
    }

    const body = await request.json()
    
    const response = await fetch(`${CUSTOMER_API_URL}/api/v1/admin/users/create`, {
      method: 'POST',
      headers: { 
        'Content-Type': 'application/json',
        Authorization: authHeader 
      },
      body: JSON.stringify(body)
    })

    if (response.status === 403) {
      return NextResponse.json({ error: 'Admin access required' }, { status: 403 })
    }

    const data = await response.json()

    if (!response.ok) {
      return NextResponse.json(
        { error: data.error?.message || 'Failed to create user' },
        { status: response.status }
      )
    }

    return NextResponse.json(data)
  } catch (error) {
    console.error('Create user error:', error)
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 })
  }
}
