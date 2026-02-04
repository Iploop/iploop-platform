import { NextResponse } from 'next/server'
import { cookies } from 'next/headers'

const CUSTOMER_API_URL = process.env.CUSTOMER_API_URL || 'http://customer-api:8002'

export async function DELETE(
  request: Request,
  { params }: { params: { keyId: string } }
) {
  try {
    const cookieStore = cookies()
    const token = cookieStore.get('token')?.value

    if (!token) {
      return NextResponse.json({ error: 'Unauthorized' }, { status: 401 })
    }

    const response = await fetch(`${CUSTOMER_API_URL}/api/v1/proxy/keys/${params.keyId}`, {
      method: 'DELETE',
      headers: { 'Authorization': `Bearer ${token}` }
    })

    const data = await response.json()
    return NextResponse.json(data, { status: response.status })
  } catch (error) {
    console.error('Delete key error:', error)
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 })
  }
}

export async function PATCH(
  request: Request,
  { params }: { params: { keyId: string } }
) {
  try {
    const cookieStore = cookies()
    const token = cookieStore.get('token')?.value

    if (!token) {
      return NextResponse.json({ error: 'Unauthorized' }, { status: 401 })
    }

    const body = await request.json()

    const response = await fetch(`${CUSTOMER_API_URL}/api/v1/proxy/keys/${params.keyId}`, {
      method: 'PATCH',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(body)
    })

    const data = await response.json()
    return NextResponse.json(data, { status: response.status })
  } catch (error) {
    console.error('Update key error:', error)
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 })
  }
}
