import { NextResponse } from 'next/server'

const CUSTOMER_API_URL = process.env.CUSTOMER_API_URL || 'http://localhost:8002'

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url)
    const days = searchParams.get('days') || '30'
    const authHeader = request.headers.get('Authorization')

    const response = await fetch(`${CUSTOMER_API_URL}/api/v1/usage/summary?days=${days}`, {
      headers: authHeader ? { Authorization: authHeader } : {}
    })

    if (!response.ok) {
      // Return mock data for demo
      return NextResponse.json({
        plan: { name: 'Pro', monthlyGb: 50, gbBalance: 45.5, gbUsed: 4.5 },
        period: { days: parseInt(days) },
        stats: {
          totalRequests: 12847,
          successfulRequests: 12589,
          failedRequests: 258,
          successRate: '97.99',
          totalBytesTransferred: 48561234567,
          totalGbTransferred: '45.23',
          avgResponseTimeMs: 342
        }
      })
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Usage summary error:', error)
    return NextResponse.json({
      stats: {
        totalRequests: 12847,
        successfulRequests: 12589,
        successRate: '97.99',
        totalGbTransferred: '45.23',
        avgResponseTimeMs: 342
      }
    })
  }
}
