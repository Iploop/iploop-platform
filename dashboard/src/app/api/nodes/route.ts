import { NextResponse } from 'next/server'

// Use new gateway server for live connection data, fallback to local
const NODE_REGISTRATION_URL = process.env.NODE_REGISTRATION_URL || 'http://localhost:8001'
const GATEWAY_URL = process.env.GATEWAY_LIVE_URL || 'http://100.122.79.93:8001'
const CUSTOMER_API_URL = process.env.CUSTOMER_API_URL || 'http://localhost:8002'

export async function GET(request: Request) {
  try {
    // Public endpoint — shows aggregate stats only, no individual node data
    // Auth is optional (logged-in users see more detail)
    
    // Fetch stats (lightweight)
    const statsRes = await fetch(`${NODE_REGISTRATION_URL}/stats`, {
      cache: 'no-store',
    })
    
    let statsData: any = {}
    if (statsRes.ok) {
      statsData = await statsRes.json()
    }
    
    // Fetch health from live gateway (has real WebSocket connection count)
    let healthData: any = {}
    try {
      const healthRes = await fetch(`${GATEWAY_URL}/health`, {
        cache: 'no-store',
        signal: AbortSignal.timeout(3000),
      })
      if (healthRes.ok) {
        healthData = await healthRes.json()
      }
    } catch {
      // Fallback to local
      const healthRes = await fetch(`${NODE_REGISTRATION_URL}/health`, { cache: 'no-store' })
      if (healthRes.ok) healthData = await healthRes.json()
    }

    // Only fetch individual nodes if authenticated
    let sanitizedNodes: any[] = []
    let nodeCount = statsData.total_nodes || healthData.total_nodes || 0
    
    const authHeader = request.headers.get('Authorization')
    if (authHeader) {
      const validateRes = await fetch(`${CUSTOMER_API_URL}/api/v1/auth/profile`, {
        headers: { Authorization: authHeader }
      })
      
      if (validateRes.ok) {
        const nodesRes = await fetch(`${NODE_REGISTRATION_URL}/nodes`, {
          cache: 'no-store',
        })
        
        if (nodesRes.ok) {
          const nodesData = await nodesRes.json()
          const allNodes = nodesData.nodes || nodesData || []
          sanitizedNodes = (Array.isArray(allNodes) ? allNodes : []).slice(0, 100).map((node: any) => ({
            id: node.id,
            country: node.country,
            countryName: node.country_name,
            city: node.city,
            region: node.region,
            connectionType: node.connection_type,
            deviceType: node.device_type,
            status: node.status,
            qualityScore: node.quality_score,
          }))
          nodeCount = nodesData.count || allNodes.length || nodeCount
        }
      }
    }
    
    return NextResponse.json({
      nodes: sanitizedNodes,
      nodeCount,
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
