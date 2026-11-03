import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { MCPCapabilityWorkspace } from './MCPCapabilityWorkspace'
import type { MCPServer } from '../../types'

export function MCPCapabilitySheet({ server, autoConnect = false, onRefresh, onClose, onRemove }: { server: MCPServer; autoConnect?: boolean; onRefresh: () => Promise<void>; onClose: () => void; onRemove: () => void }) {
  return <Sheet open onOpenChange={(open) => { if (!open) onClose() }}>
    <SheetContent side="right" className="flex flex-col gap-0 p-0" style={{ width: 'min(1080px, 88vw)', maxWidth: 'none' }}>
      <SheetHeader className="sr-only"><SheetTitle>{server.name} capabilities</SheetTitle><SheetDescription>MCP tools, resources and prompts</SheetDescription></SheetHeader>
      <MCPCapabilityWorkspace server={server} autoConnect={autoConnect} onRefresh={onRefresh} onRemove={onRemove} />
    </SheetContent>
  </Sheet>
}
