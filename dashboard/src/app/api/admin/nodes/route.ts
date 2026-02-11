import { NextResponse } from 'next/server'

const NODE_REGISTRATION_URL = process.env.NODE_REGISTRATION_URL || 'http://localhost:8001'
const CUSTOMER_API_URL = process.env.CUSTOMER_API_URL || 'http://localhost:8002'

export async function GET(request: Request) {
  try {
    // Require admin authentication
    const authHeader = request.headers.get('Authorization')
    if (!authHeader) {
      return NextResponse.json({ error: 'Authorization required' }, { status: 401 })
    }

    // Validate token and check admin role by calling admin users endpoint
    const validateRes = await fetch(`${CUSTOMER_API_URL}/api/v1/admin/users?limit=1`, {
      headers: { Authorization: authHeader }
    })
    
    if (validateRes.status === 403) {
      return NextResponse.json({ error: 'Admin access required' }, { status: 403 })
    }
    
    if (!validateRes.ok) {
      return NextResponse.json({ error: 'Invalid or expired token' }, { status: 401 })
    }

    // Fetch nodes from node-registration service (limited for performance)
    // Only fetch active/available nodes to avoid massive payload
    const url = new URL(request.url)
    const status = url.searchParams.get('status') || 'available'
    const limit = Math.min(parseInt(url.searchParams.get('limit') || '100'), 500)
    
    const nodesRes = await fetch(`${NODE_REGISTRATION_URL}/nodes?status=${status}&limit=${limit}`, {
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
    
    let statsData: any = {}
    if (statsRes.ok) {
      statsData = await statsRes.json()
    }
    
    // Fetch health
    const healthRes = await fetch(`${NODE_REGISTRATION_URL}/health`, {
      cache: 'no-store',
    })
    
    let healthData: any = {}
    if (healthRes.ok) {
      healthData = await healthRes.json()
    }

    // Process nodes for admin view — limit to prevent browser crash
    const allNodes = nodesData.nodes || nodesData || []
    const limitedNodes = Array.isArray(allNodes) ? allNodes.slice(0, limit) : []
    const nodes = limitedNodes.map((node: any) => ({
      id: node.id,
      deviceId: node.device_id,
      ipAddress: node.ip_address,
      country: node.country,
      countryName: node.country_name,
      city: node.city,
      region: node.region,
      latitude: node.latitude,
      longitude: node.longitude,
      asn: node.asn,
      isp: node.isp,
      carrier: node.carrier,
      connectionType: node.connection_type,
      deviceType: node.device_type,
      sdkVersion: node.sdk_version,
      status: node.status,
      qualityScore: node.quality_score,
      bandwidthUsedMb: node.bandwidth_used_mb,
      totalRequests: node.total_requests,
      successfulRequests: node.successful_requests,
      lastHeartbeat: node.last_heartbeat,
      connectedSince: node.connected_since,
      createdAt: node.created_at,
    }))
    
    return NextResponse.json({
      nodes,
      nodeCount: nodesData.count || nodes.length,
      stats: {
        totalNodes: healthData.total_nodes || statsData.total_nodes || 0,
        activeNodes: healthData.active_nodes || statsData.active_nodes || 0,
        inactiveNodes: healthData.inactive_nodes || statsData.inactive_nodes || 0,
        countryBreakdown: healthData.country_breakdown || statsData.country_breakdown || {},
        deviceTypes: healthData.device_types || statsData.device_types || {},
        connectionTypes: healthData.connection_types || statsData.connection_types || {},
      },
      health: {
        status: healthData.status,
        connectedNodes: healthData.connected_nodes,
      },
      timestamp: new Date().toISOString(),
    })
    
  } catch (error) {
    console.error('Error fetching admin nodes:', error)
    return NextResponse.json(
      { error: 'Failed to fetch nodes data' },
      { status: 500 }
    )
  }
}
