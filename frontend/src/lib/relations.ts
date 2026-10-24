export function commandBasename(command?: string): string {
  if (!command) return ''
  const parts = command.replace(/\\/g, '/').split('/')
  return parts[parts.length - 1] || ''
}

export function mcpDependsOnCommand(command: string | undefined, cliName: string): boolean {
  return commandBasename(command) === cliName
}
