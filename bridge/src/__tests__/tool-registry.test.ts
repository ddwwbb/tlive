import { describe, it, expect } from 'vitest';
import { getToolIcon, getToolTitle, getToolResultPreview, TOOL_RESULT_MAX_LINES } from '../engine/tool-registry.js';

describe('tool-registry', () => {
  describe('getToolIcon', () => {
    it('returns known icons for standard tools', () => {
      expect(getToolIcon('Read')).toBe('📖');
      expect(getToolIcon('Edit')).toBe('✏️');
      expect(getToolIcon('Write')).toBe('📝');
      expect(getToolIcon('Bash')).toBe('🖥️');
      expect(getToolIcon('Grep')).toBe('🔍');
      expect(getToolIcon('Glob')).toBe('📂');
      expect(getToolIcon('Agent')).toBe('🤖');
      expect(getToolIcon('WebSearch')).toBe('🌐');
      expect(getToolIcon('WebFetch')).toBe('🌐');
    });

    it('returns fallback for unknown tools', () => {
      expect(getToolIcon('CustomTool')).toBe('🔧');
    });
  });

  describe('getToolTitle', () => {
    it('extracts file name for Read/Edit/Write', () => {
      expect(getToolTitle('Read', { file_path: '/home/user/project/src/main.ts' })).toBe('Read(main.ts)');
      expect(getToolTitle('Edit', { file_path: '/tmp/bar.ts' })).toBe('Edit(bar.ts)');
      expect(getToolTitle('Write', { file_path: '/a/b/c.json' })).toBe('Write(c.json)');
    });

    it('extracts pattern for Grep/Glob', () => {
      expect(getToolTitle('Grep', { pattern: 'TODO', path: 'src/' })).toBe('Grep("TODO" in src/)');
      expect(getToolTitle('Glob', { pattern: '**/*.ts' })).toBe('Glob(**/*.ts)');
    });

    it('extracts command for Bash (truncated)', () => {
      expect(getToolTitle('Bash', { command: 'npm test' })).toBe('Bash(npm test)');
      const longCmd = 'find . -name "*.ts" -type f -exec grep -l "pattern" {} \\; | sort | head -20 | while read f; do echo "$f"; done';
      const title = getToolTitle('Bash', { command: longCmd });
      expect(title.length).toBeLessThanOrEqual(90);
    });

    it('shows agent description', () => {
      expect(getToolTitle('Agent', { description: 'Explore codebase' })).toBe('Agent(Explore codebase)');
    });

    it('returns just tool name for unknown or empty input', () => {
      expect(getToolTitle('CustomTool', {})).toBe('CustomTool');
      expect(getToolTitle('Read', {})).toBe('Read');
    });
  });

  describe('getToolResultPreview', () => {
    it('returns empty for no-result tools (Read, Glob)', () => {
      expect(getToolResultPreview('Read', 'file content here')).toBe('');
      expect(getToolResultPreview('Glob', 'file1.ts\nfile2.ts')).toBe('');
    });

    it('truncates long Bash output with line count', () => {
      const lines = Array.from({ length: 30 }, (_, i) => `line ${i + 1}`);
      const result = getToolResultPreview('Bash', lines.join('\n'));
      expect(result).toContain('line 1');
      expect(result).toContain(`+${30 - TOOL_RESULT_MAX_LINES} lines`);
    });

    it('shows short Bash output in full', () => {
      expect(getToolResultPreview('Bash', 'OK')).toBe('OK');
    });

    it('shows error results for any tool', () => {
      const result = getToolResultPreview('Read', 'File not found', true);
      expect(result).toContain('❌');
      expect(result).toContain('File not found');
    });
  });
});
