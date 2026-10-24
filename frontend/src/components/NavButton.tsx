import type { ReactNode } from 'react'
import { SidebarMenuBadge, SidebarMenuButton, SidebarMenuItem } from '@/components/ui/sidebar'

export function NavButton({ active, disabled, onClick, icon, label, count, hint }: { active?: boolean; disabled?: boolean; onClick?: () => void; icon: ReactNode; label: string; count?: number; hint?: string }) {
  return <SidebarMenuItem>
    <SidebarMenuButton isActive={active} disabled={disabled} onClick={onClick} tooltip={label} aria-label={label} aria-current={active ? 'page' : undefined} className="h-[30px] gap-2.5 px-2 text-[13px]">
      <span className="flex w-4 shrink-0 justify-center">{icon}</span>
      <span>{label}</span>
      {hint && <small className="ml-auto text-[9.5px] text-muted-foreground">{hint}</small>}
    </SidebarMenuButton>
    {count !== undefined && <SidebarMenuBadge>{count}</SidebarMenuBadge>}
  </SidebarMenuItem>
}
