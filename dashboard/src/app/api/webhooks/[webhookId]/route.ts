import { NextResponse } from 'next/server'

const CUSTOMER_API_URL = process.env.CUSTOMER_API_URL || 'http://localhost:8002'

export async function DELETE(
  request: Request,
  { params }: { params: { webhookId: string } }
) {
  try {
    const authHeader = request.headers.get('Authorization')
    if (!authHeader) {
      return NextResponse.json({ error: 'Authorization required' }, { status: 401 })
    }

    const response = await fetch(`${CUSTOMER_API_URL}/api/v1/webhooks/${params.webhookId}`, {
      method: 'DELETE',
      headers: { Authorization: authHeader }
    })

    const data = await response.json()
    if (!response.ok) {
      return NextResponse.json({ error: data.error?.message || 'Failed' }, { status: response.status })
    }
    return NextResponse.json(data)
  } catch (error) {
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 })
  }
}
