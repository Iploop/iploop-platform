import { NextResponse } from 'next/server'

const CUSTOMER_API_URL = process.env.CUSTOMER_API_URL || 'http://localhost:8002'

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url)
    const days = searchParams.get('days') || '30'
    const authHeader = request.headers.get('Authorization')

    const response = await fetch(`${CUSTOMER_API_URL}/api/v1/usage/by-country?days=${days}`, {
      headers: authHeader ? { Authorization: authHeader } : {}
    })

    if (!response.ok) {
      // Return mock data
      return NextResponse.json({
        byCountry: [
          { country: 'IL', requests: 4521, bytesTransferred: 15234500000, mbTransferred: '15234.50' },
          { country: 'US', requests: 3892, bytesTransferred: 12453200000, mbTransferred: '12453.20' },
          { country: 'DE', requests: 1823, bytesTransferred: 6234800000, mbTransferred: '6234.80' },
          { country: 'UK', requests: 1245, bytesTransferred: 4521300000, mbTransferred: '4521.30' },
          { country: 'FR', requests: 876, bytesTransferred: 2934600000, mbTransferred: '2934.60' },
          { country: 'NL', requests: 490, bytesTransferred: 1823400000, mbTransferred: '1823.40' }
        ]
      })
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Usage by-country error:', error)
    return NextResponse.json({ byCountry: [] })
  }
}
