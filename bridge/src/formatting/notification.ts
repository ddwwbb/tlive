import type { ChannelType, OutboundMessage } from '../channels/types.js';
import type { NotificationData } from './types.js';

interface NotificationMessage {
  text?: string;
  html?: string;
  embed?: OutboundMessage['embed'];
  feishuHeader?: { template: string; title: string };
  /** Feishu Card 2.0: structured elements for richer layout */
  feishuElements?: Array<Record<string, unknown>>;
}

const COLOR_MAP: Record<NotificationData['type'], number> = {
  stop: 0x00CC66,       // green
  idle_prompt: 0x3399FF, // blue
  generic: 0x888888,     // gray
};

const HEADER_MAP: Record<NotificationData['type'], string> = {
  stop: 'green',
  idle_prompt: 'yellow',
  generic: 'blue',
};

const EMOJI_MAP: Record<NotificationData['type'], string> = {
  stop: '✅',
  idle_prompt: '⏳',
  generic: '📢',
};

function truncateSummary(s: string, max = 3000): string {
  return s.length > max ? s.slice(0, max - 3) + '...' : s;
}

export function formatNotification(data: NotificationData, channelType: ChannelType): NotificationMessage {
  const summary = data.summary ? truncateSummary(data.summary) : undefined;
  const emoji = EMOJI_MAP[data.type];

  switch (channelType) {
    case 'telegram': {
      const parts = [`${emoji} <b>${data.title}</b>`];
      if (summary) {
        parts.push('', `<blockquote>${summary}</blockquote>`);
      }
      if (data.terminalUrl) {
        parts.push('', `🔗 <a href="${data.terminalUrl}">Open Terminal</a>`);
      }
      return { html: parts.join('\n') };
    }

    case 'discord': {
      return {
        embed: {
          title: `${emoji} ${data.title}`,
          color: COLOR_MAP[data.type],
          description: summary ? `> ${summary.replace(/\n/g, '\n> ')}` : undefined,
          footer: data.terminalUrl ? `🔗 Open Terminal: ${data.terminalUrl}` : undefined,
        },
      };
    }

    case 'feishu': {
      const elements: Array<Record<string, unknown>> = [];
      if (summary) {
        elements.push({ tag: 'markdown', content: summary });
      }
      if (data.terminalUrl) {
        elements.push({ tag: 'hr' });
        elements.push({
          tag: 'note',
          elements: [{ tag: 'plain_text', content: `🔗 Open Terminal: ${data.terminalUrl}` }],
        });
      }
      return {
        text: summary || '',
        feishuHeader: { template: HEADER_MAP[data.type], title: `${emoji} ${data.title}` },
        feishuElements: elements,
      };
    }
  }
}
