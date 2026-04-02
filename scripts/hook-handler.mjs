#!/usr/bin/env node
import { readFileSync, existsSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';

// Read stdin
let input = '';
for await (const chunk of process.stdin) {
  input += chunk;
}

const sessionId = process.env.TLIVE_SESSION_ID;
if (!sessionId) process.exit(0);

const pauseFile = join(homedir(), '.tlive', 'hooks-paused');
if (existsSync(pauseFile)) process.exit(0);

// Parse and inject session info
let hookJson = JSON.parse(input);
hookJson.tlive_session_id = sessionId;
hookJson.tlive_cwd = process.cwd();

// Load config
let port = '8080';
let token = '';
const configPath = join(homedir(), '.tlive', 'config.env');
if (existsSync(configPath)) {
  try {
    const lines = readFileSync(configPath, 'utf-8').split('\n');
    for (const line of lines) {
      const match = line.match(/^(\w+)=["']?(.*?)["']?\s*$/);
      if (match) {
        if (match[1] === 'TL_PORT') port = match[2];
        if (match[1] === 'TL_TOKEN') token = match[2];
      }
    }
  } catch {}
}

const baseUrl = `http://localhost:${port}`;
const headers = {
  'Authorization': `Bearer ${token}`,
  'Content-Type': 'application/json',
};

// Check if core is running
try {
  const status = await fetch(`${baseUrl}/api/status`, {
    headers: { 'Authorization': `Bearer ${token}` },
    signal: AbortSignal.timeout(5000),
  });
  if (!status.ok) process.exit(0);
} catch {
  process.exit(0);
}

// Long-poll permission endpoint (up to 300s)
let response;
try {
  response = await fetch(`${baseUrl}/api/hooks/permission`, {
    method: 'POST',
    headers,
    body: JSON.stringify(hookJson),
    signal: AbortSignal.timeout(300_000),
  });
} catch {
  process.exit(0);
}

if (!response.ok) process.exit(0);

let body;
try {
  body = await response.json();
} catch {
  process.exit(0);
}

const decision = body.decision || 'allow';

switch (decision) {
  case 'allow':
    process.stdout.write(JSON.stringify({
      hookSpecificOutput: {
        hookEventName: 'PermissionRequest',
        decision: { behavior: 'allow' },
      },
    }));
    break;
  case 'allow_always': {
    const suggestions = body.suggestions || [];
    process.stdout.write(JSON.stringify({
      hookSpecificOutput: {
        hookEventName: 'PermissionRequest',
        decision: { behavior: 'allow', updatedPermissions: suggestions },
      },
    }));
    break;
  }
  case 'deny':
    process.stdout.write(JSON.stringify({
      hookSpecificOutput: {
        hookEventName: 'PermissionRequest',
        decision: { behavior: 'deny' },
      },
    }));
    break;
}
