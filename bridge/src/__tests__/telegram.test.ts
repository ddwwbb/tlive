import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock the telegram bot before importing
vi.mock('node-telegram-bot-api', () => {
  const MockBot = vi.fn(function (this: any) {
    this.on = vi.fn();
    this.sendMessage = vi.fn().mockResolvedValue({ message_id: 42 });
    this.editMessageText = vi.fn().mockResolvedValue({});
    this.stopPolling = vi.fn().mockResolvedValue(undefined);
  });
  return { default: MockBot };
});

import { TelegramAdapter } from '../channels/telegram.js';

describe('TelegramAdapter', () => {
  let adapter: TelegramAdapter;

  beforeEach(() => {
    adapter = new TelegramAdapter({
      botToken: 'test-token',
      chatId: '12345',
      allowedUsers: ['user1', 'user2'],
    });
  });

  it('has correct channel type', () => {
    expect(adapter.channelType).toBe('telegram');
  });

  it('validates config — requires botToken', () => {
    const bad = new TelegramAdapter({ botToken: '', chatId: '', allowedUsers: [] });
    expect(bad.validateConfig()).toContain('TL_TG_BOT_TOKEN');
  });

  it('validates config — passes with token', () => {
    expect(adapter.validateConfig()).toBeNull();
  });

  it('authorizes allowed users', () => {
    expect(adapter.isAuthorized('user1', '12345')).toBe(true);
    expect(adapter.isAuthorized('unknown', '12345')).toBe(false);
  });

  it('authorizes all users when allowedUsers is empty', () => {
    const openAdapter = new TelegramAdapter({ botToken: 'tok', chatId: '', allowedUsers: [] });
    expect(openAdapter.isAuthorized('anyone', 'anychat')).toBe(true);
  });

  it('sends message with HTML parse mode', async () => {
    await adapter.start();
    const result = await adapter.send({ chatId: '12345', html: '<b>hello</b>' });
    expect(result.success).toBe(true);
    expect(result.messageId).toBe('42');
  });

  it('sends message with buttons as inline keyboard', async () => {
    await adapter.start();
    const result = await adapter.send({
      chatId: '12345',
      text: 'Choose:',
      buttons: [
        { label: 'Allow', callbackData: 'perm:allow:123' },
        { label: 'Deny', callbackData: 'perm:deny:123', style: 'danger' },
      ],
    });
    expect(result.success).toBe(true);
  });
});
