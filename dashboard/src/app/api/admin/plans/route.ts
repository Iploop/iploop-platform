import { NextResponse } from 'next/server'

const CUSTOMER_API_URL = process.env.CUSTOMER_API_URL || 'http://localhost:8002'

export async function GET(request: Request) {
  try {
    const authHeader = request.headers.get('Authorization')
    
    if (!authHeader) {
      return NextResponse.json({ error: 'Authorization required' }, { status: 401 })
    }
    
    const response = await fetch(`${CUSTOMER_API_URL}/api/v1/admin/plans`, {
      headers: { Authorization: authHeader }
    })

    if (response.status === 403) {
      return NextResponse.json({ error: 'Admin access required' }, { status: 403 })
    }

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}))
      return NextResponse.json(
        { error: errorData.error?.message || 'Failed to fetch plans' },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Admin plans error:', error)
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 })
  }
}

export async function POST(request: Request) {
  try {
    const authHeader = request.headers.get('Authorization')
    
    if (!authHeader) {
      return NextResponse.json({ error: 'Authorization required' }, { status: 401 })
    }

    const body = await request.json()
    
    const response = await fetch(`${CUSTOMER_API_URL}/api/v1/admin/plans`, {
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
        { error: data.error?.message || 'Failed to create plan' },
        { status: response.status }
      )
    }

    return NextResponse.json(data)
  } catch (error) {
    console.error('Create plan error:', error)
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 })
  }
}
