import { describe, it, expect } from 'vitest';
import { sseEvent, parseSSE } from '../providers/sse-utils.js';

describe('sseEvent', () => {
  it('formats event as SSE data line', () => {
    const result = sseEvent('text', 'hello');
    expect(result).toBe('data: {"type":"text","data":"hello"}\n');
  });

  it('handles object data', () => {
    const result = sseEvent('tool_use', { id: '1', name: 'Edit', input: {} });
    const parsed = JSON.parse(result.slice(6));
    expect(parsed.type).toBe('tool_use');
    expect(parsed.data.name).toBe('Edit');
  });
});

describe('parseSSE', () => {
  it('parses SSE data line', () => {
    const result = parseSSE('data: {"type":"text","data":"hello"}');
    expect(result).toEqual({ type: 'text', data: 'hello' });
  });

  it('returns null for non-data lines', () => {
    expect(parseSSE('')).toBeNull();
    expect(parseSSE('event: message')).toBeNull();
    expect(parseSSE(': comment')).toBeNull();
  });

  it('returns null for malformed JSON', () => {
    expect(parseSSE('data: {invalid')).toBeNull();
  });
});
