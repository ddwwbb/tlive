import { describe, it, expect } from 'vitest';
import { createAdapter, getRegisteredTypes } from '../channels/index.js';

describe('Channel Adapter Registry', () => {
  it('starts with no registered adapters', () => {
    // No adapters imported yet
    expect(getRegisteredTypes()).toEqual([]);
  });

  it('throws on unknown channel type', () => {
    expect(() => createAdapter('telegram')).toThrow('Unknown channel type: telegram');
  });
});
