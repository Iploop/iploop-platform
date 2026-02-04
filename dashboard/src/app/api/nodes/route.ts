import { NextResponse } from 'next/server'

const NODE_REGISTRATION_URL = process.env.NODE_REGISTRATION_URL || 'http://localhost:8001'
const CUSTOMER_API_URL = process.env.CUSTOMER_API_URL || 'http://localhost:8002'

export async function GET(request: Request) {
  try {
    // Require authentication
    const authHeader = request.headers.get('Authorization')
    if (!authHeader) {
      return NextResponse.json({ error: 'Authorization required' }, { status: 401 })
    }

    // Validate token with customer-api
    const validateRes = await fetch(`${CUSTOMER_API_URL}/api/v1/auth/profile`, {
      headers: { Authorization: authHeader }
    })
    
    if (!validateRes.ok) {
      return NextResponse.json({ error: 'Invalid or expired token' }, { status: 401 })
    }
    // Fetch nodes from node-registration service
    const nodesRes = await fetch(`${NODE_REGISTRATION_URL}/nodes`, {
      cache: 'no-store',
    })
    
    if (!nodesRes.ok) {
      throw new Error('Failed to fetch nodes')
    }
    
    const nodesData = await nodesRes.json()
    
    // Fetch stats
    const statsRes = await fetch(`${NODE_REGISTRATION_URL}/stats`, {
      cache: 'no-store',
    })
    
    let statsData = {}
    if (statsRes.ok) {
      statsData = await statsRes.json()
    }
    
    // Fetch health
    const healthRes = await fetch(`${NODE_REGISTRATION_URL}/health`, {
      cache: 'no-store',
    })
    
    let healthData = {}
    if (healthRes.ok) {
      healthData = await healthRes.json()
    }
    
    // Sanitize node data - remove sensitive fields like IP addresses
    const sanitizedNodes = (nodesData.nodes || []).map((node: any) => ({
      id: node.id,
      country: node.country,
      countryName: node.country_name,
      city: node.city,
      region: node.region,
      connectionType: node.connection_type,
      deviceType: node.device_type,
      status: node.status,
      qualityScore: node.quality_score,
      // Intentionally omitting: ip_address, device_id, asn, isp, latitude, longitude
    }))
    
    return NextResponse.json({
      nodes: sanitizedNodes,
      nodeCount: nodesData.count || 0,
      stats: statsData,
      health: healthData,
      timestamp: new Date().toISOString(),
    })
    
  } catch (error) {
    console.error('Error fetching nodes:', error)
    return NextResponse.json(
      { error: 'Failed to fetch nodes data', nodes: [], nodeCount: 0 },
      { status: 500 }
    )
  }
}
