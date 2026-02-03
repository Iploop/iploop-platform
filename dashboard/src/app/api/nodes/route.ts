import { NextResponse } from 'next/server'

const NODE_REGISTRATION_URL = process.env.NODE_REGISTRATION_URL || 'http://localhost:8001'

export async function GET() {
  try {
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
    
    return NextResponse.json({
      nodes: nodesData.nodes || [],
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
