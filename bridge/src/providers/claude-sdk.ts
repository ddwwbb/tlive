import { spawn, type ChildProcess } from 'node:child_process';
import { createInterface } from 'node:readline';
import { sseEvent } from './sse-utils.js';
import type { LLMProvider, StreamChatParams } from './base.js';

export interface PermissionHandler {
  resolvePendingPermission(id: string, allowed: boolean): boolean;
}

export class ClaudeSDKProvider implements LLMProvider {
  private permissions: PermissionHandler;

  constructor(permissions: PermissionHandler) {
    this.permissions = permissions;
  }

  streamChat(params: StreamChatParams): ReadableStream<string> {
    return new ReadableStream({
      start: async (controller) => {
        try {
          await this.streamViaSDK(params, controller);
        } catch {
          // SDK not available, fallback to CLI
          await this.streamViaCLI(params, controller);
        }
        controller.close();
      },
    });
  }

  private async streamViaSDK(
    params: StreamChatParams,
    controller: ReadableStreamDefaultController<string>
  ): Promise<void> {
    // Dynamic import — throws if not installed
    const sdk = await import('@anthropic-ai/claude-agent-sdk');

    const result = await sdk.query({
      prompt: params.prompt,
      options: {
        workingDirectory: params.workingDirectory,
        model: params.model,
        sessionId: params.sessionId,
        permissionMode: params.permissionMode ?? 'default',
        abortSignal: params.abortSignal,
      },
      canUseTool: async (toolName: string, toolInput: unknown, meta: { toolUseID: string }) => {
        // Emit permission request event
        controller.enqueue(sseEvent('permission_request', {
          permissionRequestId: meta.toolUseID,
          toolName,
          toolInput,
        }));
        // Block until user responds (handled by permission gateway)
        // This is a simplified version — real implementation would use PendingPermissions
        return true;
      },
      onMessage: (event: any) => {
        // Convert SDK events to our SSE format
        if (event.type === 'text_delta') {
          controller.enqueue(sseEvent('text', event.text));
        } else if (event.type === 'tool_use') {
          controller.enqueue(sseEvent('tool_use', {
            id: event.id,
            name: event.name,
            input: event.input,
          }));
        } else if (event.type === 'tool_result') {
          controller.enqueue(sseEvent('tool_result', {
            tool_use_id: event.tool_use_id,
            content: event.content,
            is_error: event.is_error ?? false,
          }));
        } else if (event.type === 'result') {
          controller.enqueue(sseEvent('result', {
            session_id: event.session_id,
            is_error: event.is_error ?? false,
            usage: event.usage,
          }));
        }
      },
    });
  }

  private async streamViaCLI(
    params: StreamChatParams,
    controller: ReadableStreamDefaultController<string>
  ): Promise<void> {
    const args = [
      '--output-format', 'stream-json',
      '--verbose',
    ];

    if (params.model) {
      args.push('--model', params.model);
    }

    if (params.permissionMode) {
      args.push('--permission-mode', params.permissionMode);
    }

    if (params.sessionId) {
      args.push('--session-id', params.sessionId);
    }

    // Append the prompt
    args.push('-p', params.prompt);

    return new Promise<void>((resolve, reject) => {
      const child = spawn('claude', args, {
        cwd: params.workingDirectory,
        stdio: ['pipe', 'pipe', 'pipe'],
      });

      const rl = createInterface({ input: child.stdout });

      rl.on('line', (line) => {
        if (!line.trim()) return;

        try {
          const event = JSON.parse(line);

          // Map Claude CLI stream-json events to our SSE format
          if (event.type === 'assistant' && event.message?.content) {
            for (const block of event.message.content) {
              if (block.type === 'text') {
                controller.enqueue(sseEvent('text', block.text));
              } else if (block.type === 'tool_use') {
                controller.enqueue(sseEvent('tool_use', {
                  id: block.id,
                  name: block.name,
                  input: block.input,
                }));
              }
            }
          } else if (event.type === 'result') {
            controller.enqueue(sseEvent('result', {
              session_id: event.session_id ?? '',
              is_error: event.is_error ?? false,
              usage: event.usage,
            }));
          }
        } catch {
          // Skip unparseable lines
        }
      });

      child.on('close', (code) => {
        if (code !== 0) {
          controller.enqueue(sseEvent('error', `Claude CLI exited with code ${code}`));
        }
        resolve();
      });

      child.on('error', (err) => {
        controller.enqueue(sseEvent('error', `Failed to spawn claude CLI: ${err.message}`));
        resolve();
      });

      // Handle abort
      if (params.abortSignal) {
        params.abortSignal.addEventListener('abort', () => {
          child.kill('SIGTERM');
        });
      }
    });
  }
}
