export type AssistantActivity =
  | { kind: 'thinking', content: string, running: boolean }
  | { kind: 'tool', id: string, name: string, args: string, output: string, status: 'running' | 'completed' | 'failed' | 'interrupted' }
