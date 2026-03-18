import { PendingPermissions } from './gateway.js';
import type { BaseChannelAdapter } from '../channels/base.js';
import type { OutboundMessage } from '../channels/types.js';

export class PermissionBroker {
  private gateway: PendingPermissions;
  private publicUrl: string;

  constructor(gateway: PendingPermissions, publicUrl: string) {
    this.gateway = gateway;
    this.publicUrl = publicUrl;
  }

  async forwardPermissionRequest(
    request: { permissionRequestId: string; toolName: string; toolInput: unknown },
    chatId: string,
    adapters: BaseChannelAdapter[]
  ): Promise<void> {
    const inputStr = typeof request.toolInput === 'string'
      ? request.toolInput
      : JSON.stringify(request.toolInput, null, 2);
    const truncatedInput = inputStr.length > 300 ? inputStr.slice(0, 297) + '...' : inputStr;

    const webLink = this.publicUrl ? `\n\nView Terminal: ${this.publicUrl}` : '';

    const message: OutboundMessage = {
      chatId,
      text: `Permission Required\n\nTool: ${request.toolName}\n\`\`\`\n${truncatedInput}\n\`\`\`\n\nExpires in 5 minutes${webLink}`,
      buttons: [
        { label: 'Allow', callbackData: `perm:allow:${request.permissionRequestId}`, style: 'primary' },
        { label: 'Allow Session', callbackData: `perm:allow_session:${request.permissionRequestId}`, style: 'default' },
        { label: 'Deny', callbackData: `perm:deny:${request.permissionRequestId}`, style: 'danger' },
      ],
    };

    // Send to all connected adapters
    await Promise.all(adapters.map(a => a.send(message)));
  }

  handlePermissionCallback(callbackData: string): boolean {
    // Format: perm:allow:<id>, perm:deny:<id>, perm:allow_session:<id>
    const match = callbackData.match(/^perm:(allow|deny|allow_session):(.+)$/);
    if (!match) return false;

    const [, action, permId] = match;
    const allowed = action === 'allow' || action === 'allow_session';
    return this.gateway.resolve(permId, allowed);
  }
}
