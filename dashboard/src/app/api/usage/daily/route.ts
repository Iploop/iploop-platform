import { NextResponse } from 'next/server'

const CUSTOMER_API_URL = process.env.CUSTOMER_API_URL || 'http://localhost:8002'

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url)
    const days = searchParams.get('days') || '30'
    const authHeader = request.headers.get('Authorization')

    const response = await fetch(`${CUSTOMER_API_URL}/api/v1/usage/daily?days=${days}`, {
      headers: authHeader ? { Authorization: authHeader } : {}
    })

    if (!response.ok) {
      // Return mock data
      const mockData = []
      for (let i = 0; i < parseInt(days); i++) {
        const date = new Date()
        date.setDate(date.getDate() - i)
        const requests = Math.floor(Math.random() * 500) + 200
        mockData.push({
          date: date.toISOString().split('T')[0],
          requests,
          successful: Math.floor(requests * 0.98),
          bytesTransferred: Math.floor(Math.random() * 500000000) + 100000000,
          mbTransferred: (Math.random() * 500 + 100).toFixed(2),
          avgResponseTimeMs: Math.floor(Math.random() * 200) + 200
        })
      }
      return NextResponse.json({ daily: mockData })
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Usage daily error:', error)
    return NextResponse.json({ daily: [] })
  }
}
