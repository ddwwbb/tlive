import { describe, it, expect, vi, beforeEach } from 'vitest';
import { BridgeManager } from '../engine/bridge-manager.js';
import { initBridgeContext } from '../context.js';
import type { BaseChannelAdapter } from '../channels/base.js';

function mockAdapter(channelType = 'telegram'): BaseChannelAdapter {
  const messageQueue: any[] = [];
  return {
    channelType,
    start: vi.fn().mockResolvedValue(undefined),
    stop: vi.fn().mockResolvedValue(undefined),
    consumeOne: vi.fn().mockImplementation(() => messageQueue.shift() ?? null),
    send: vi.fn().mockResolvedValue({ messageId: '1', success: true }),
    editMessage: vi.fn(),
    validateConfig: vi.fn().mockReturnValue(null),
    isAuthorized: vi.fn().mockReturnValue(true),
    _pushMessage: (msg: any) => messageQueue.push(msg),
  } as any;
}

describe('BridgeManager', () => {
  let manager: BridgeManager;

  beforeEach(() => {
    initBridgeContext({
      store: {
        getSession: vi.fn().mockResolvedValue({ id: 's1', workingDirectory: '/tmp', createdAt: '' }),
        saveMessage: vi.fn(), getMessages: vi.fn().mockResolvedValue([]),
        acquireLock: vi.fn().mockResolvedValue(true),
        renewLock: vi.fn().mockResolvedValue(true),
        releaseLock: vi.fn(),
        saveSession: vi.fn(), deleteSession: vi.fn(), listSessions: vi.fn(),
        getBinding: vi.fn().mockResolvedValue({ channelType: 'telegram', chatId: 'c1', sessionId: 's1', createdAt: '' }),
        saveBinding: vi.fn(), deleteBinding: vi.fn(), listBindings: vi.fn(),
        isDuplicate: vi.fn().mockResolvedValue(false), markProcessed: vi.fn(),
      } as any,
      llm: {
        streamChat: () => new ReadableStream({
          start(c) { c.enqueue('data: {"type":"text","data":"reply"}\n'); c.enqueue('data: {"type":"result","data":{"session_id":"s1","is_error":false}}\n'); c.close(); }
        }),
      } as any,
      permissions: { resolvePendingPermission: vi.fn() } as any,
      core: { isHealthy: () => true } as any,
    });
    manager = new BridgeManager();
  });

  it('starts adapters', async () => {
    const adapter = mockAdapter();
    manager.registerAdapter(adapter);
    await manager.start();
    expect(adapter.start).toHaveBeenCalled();
  });

  it('stops adapters', async () => {
    const adapter = mockAdapter();
    manager.registerAdapter(adapter);
    await manager.start();
    await manager.stop();
    expect(adapter.stop).toHaveBeenCalled();
  });

  it('skips adapters with invalid config', async () => {
    const adapter = mockAdapter();
    (adapter.validateConfig as any).mockReturnValue('missing token');
    manager.registerAdapter(adapter);
    await manager.start();
    expect(adapter.start).not.toHaveBeenCalled();
  });

  it('filters unauthorized messages', async () => {
    const adapter = mockAdapter();
    (adapter.isAuthorized as any).mockReturnValue(false);
    manager.registerAdapter(adapter);

    const processed = await manager.handleInboundMessage(adapter, {
      channelType: 'telegram', chatId: 'c1', userId: 'u1', text: 'hello', messageId: 'm1',
    });
    expect(processed).toBe(false);
  });

  it('routes callback data to permission broker', async () => {
    const adapter = mockAdapter();
    manager.registerAdapter(adapter);

    const handled = await manager.handleInboundMessage(adapter, {
      channelType: 'telegram', chatId: 'c1', userId: 'u1', text: '',
      callbackData: 'perm:allow:p1', messageId: 'm1',
    });
    // Even if permission not found, it should attempt handling
    expect(handled).toBe(true);
  });

  it('routes /status command', async () => {
    const adapter = mockAdapter();
    manager.registerAdapter(adapter);

    const handled = await manager.handleInboundMessage(adapter, {
      channelType: 'telegram', chatId: 'c1', userId: 'u1', text: '/status', messageId: 'm1',
    });
    expect(handled).toBe(true);
    expect(adapter.send).toHaveBeenCalled();
  });
});
