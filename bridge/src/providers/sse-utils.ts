export function sseEvent(type: string, data: unknown): string {
  return `data: ${JSON.stringify({ type, data })}\n`;
}

export function parseSSE(line: string): { type: string; data: unknown } | null {
  if (!line.startsWith('data: ')) return null;
  try {
    return JSON.parse(line.slice(6));
  } catch {
    return null;
  }
}
