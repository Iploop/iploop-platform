import { NextResponse } from 'next/server';
import fs from 'fs';
import path from 'path';

const RESULTS_DIR = path.join(process.cwd(), '..', 'tests', 'pilot', 'results');

export async function GET() {
  try {
    // Read latest.json
    const latestPath = path.join(RESULTS_DIR, 'latest.json');
    let latest = {};
    
    if (fs.existsSync(latestPath)) {
      latest = JSON.parse(fs.readFileSync(latestPath, 'utf8'));
    }

    // Get history for each test
    const history: Record<string, any[]> = {};
    
    if (fs.existsSync(RESULTS_DIR)) {
      const files = fs.readdirSync(RESULTS_DIR)
        .filter(f => f.endsWith('.json') && f !== 'latest.json')
        .sort()
        .reverse();

      for (const file of files) {
        try {
          const data = JSON.parse(fs.readFileSync(path.join(RESULTS_DIR, file), 'utf8'));
          const testName = data.testName;
          
          if (!history[testName]) {
            history[testName] = [];
          }
          
          if (history[testName].length < 20) {
            history[testName].push(data);
          }
        } catch (e) {
          // Skip invalid files
        }
      }
    }

    return NextResponse.json({ latest, history });
  } catch (error) {
    console.error('Error fetching test results:', error);
    return NextResponse.json({ latest: {}, history: {} }, { status: 500 });
  }
}
