import { NextResponse } from 'next/server'

const CUSTOMER_API_URL = process.env.CUSTOMER_API_URL || 'http://localhost:8002'

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url)
    const token = searchParams.get('token')
    
    if (!token) {
      return NextResponse.json(
        { error: 'Verification token is required' },
        { status: 400 }
      )
    }

    const response = await fetch(`${CUSTOMER_API_URL}/api/v1/auth/verify-email?token=${token}`)
    const data = await response.json()

    if (!response.ok) {
      return NextResponse.json(
        { error: data.error || 'Verification failed' },
        { status: response.status }
      )
    }

    return NextResponse.json(data)
  } catch (error) {
    console.error('Verify email error:', error)
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    )
  }
}
