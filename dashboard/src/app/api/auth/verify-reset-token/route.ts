import { NextResponse } from 'next/server'

const CUSTOMER_API_URL = process.env.CUSTOMER_API_URL || 'http://localhost:8002'

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url)
    const token = searchParams.get('token')
    
    if (!token) {
      return NextResponse.json({ valid: false, message: 'Token is required' })
    }

    const response = await fetch(`${CUSTOMER_API_URL}/api/v1/auth/verify-reset-token?token=${token}`)
    const data = await response.json()

    return NextResponse.json(data)
  } catch (error) {
    console.error('Verify token error:', error)
    return NextResponse.json({ valid: false, message: 'Verification failed' })
  }
}
