import { NextRequest, NextResponse } from 'next/server';
import { spawn } from 'child_process';
import path from 'path';

const TESTS_DIR = path.join(process.cwd(), '..', 'tests', 'pilot');

// Map test names to commands
const TEST_COMMANDS: Record<string, string[]> = {
  'stress-smoke': ['./run-tests.sh', 'smoke'],
  'stress-ramp': ['./run-tests.sh', 'stress'],
  'stress-sustained': ['./run-tests.sh', 'sustained'],
  'stress-burst': ['./run-tests.sh', 'burst'],
  'leak': ['node', 'leak-test.js'],
  'protocol': ['node', 'protocol-test.js'],
  'geo': ['node', 'geo-test.js'],
  'sticky': ['node', 'sticky-stress.js'],
  'rotation': ['node', 'rotation-test.js'],
  'peer': ['node', 'peer-behavior-test.js'],
  'security': ['node', 'security-test.js'],
  'auth': ['node', 'auth-test.js'],
  'bandwidth': ['node', 'bandwidth-test.js'],
  'failure': ['node', 'failure-test.js'],
  'latency': ['node', 'latency-test.js'],
  'connection': ['node', 'connection-limits.js'],
  'concurrency': ['node', 'concurrency-edge.js'],
  'stability': ['node', 'stability-test.js'],
  'scenario': ['node', 'scenario-price-scrape.js'],
};

export async function POST(request: NextRequest) {
  const { test } = await request.json();
  
  if (!test || !TEST_COMMANDS[test]) {
    return NextResponse.json({ error: 'Invalid test name' }, { status: 400 });
  }

  const [cmd, ...args] = TEST_COMMANDS[test];

  // Create a streaming response
  const encoder = new TextEncoder();
  const stream = new ReadableStream({
    start(controller) {
      const proc = spawn(cmd, args, {
        cwd: TESTS_DIR,
        env: {
          ...process.env,
          PROXY_HOST: process.env.PROXY_HOST || 'localhost',
          PROXY_PORT: process.env.PROXY_PORT || '8080',
          PROXY_USER: process.env.PROXY_USER || 'test_customer',
          PROXY_PASS: process.env.PROXY_PASS || 'test_api_key',
        }
      });

      proc.stdout?.on('data', (data) => {
        controller.enqueue(encoder.encode(data.toString()));
      });

      proc.stderr?.on('data', (data) => {
        controller.enqueue(encoder.encode(data.toString()));
      });

      proc.on('close', (code) => {
        controller.enqueue(encoder.encode(`\n\nTest finished with code: ${code}\n`));
        controller.close();
      });

      proc.on('error', (err) => {
        controller.enqueue(encoder.encode(`\nError: ${err.message}\n`));
        controller.close();
      });
    }
  });

  return new Response(stream, {
    headers: {
      'Content-Type': 'text/plain; charset=utf-8',
      'Transfer-Encoding': 'chunked',
    }
  });
}
