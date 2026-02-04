import { NextResponse } from 'next/server'
import { cookies } from 'next/headers'

const CUSTOMER_API_URL = process.env.CUSTOMER_API_URL || 'http://localhost:8002'

export async function POST(request: Request) {
  try {
    const body = await request.json()
    
    const response = await fetch(`${CUSTOMER_API_URL}/api/v1/auth/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    })

    const data = await response.json()

    if (!response.ok) {
      return NextResponse.json(
        { error: data.error || 'Registration failed' },
        { status: response.status }
      )
    }

    // Set token as HTTP-only cookie for API routes
    if (data.token) {
      const cookieStore = cookies()
      cookieStore.set('token', data.token, {
        httpOnly: true,
        secure: process.env.NODE_ENV === 'production',
        sameSite: 'lax',
        maxAge: 60 * 60 * 24 // 24 hours
      })
    }

    return NextResponse.json(data)
  } catch (error) {
    console.error('Register error:', error)
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    )
  }
}
