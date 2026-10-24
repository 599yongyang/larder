import { AppWindow, Terminal, type LucideIcon } from 'lucide-react'
import homebrew from '../assets/cli-icons/homebrew.svg'
import npm from '../assets/cli-icons/npm.svg'
import anaconda from '../assets/cli-icons/anaconda.svg'
import rust from '../assets/cli-icons/rust.svg'
import bun from '../assets/cli-icons/bun.svg'
import uv from '../assets/cli-icons/uv.svg'
import go from '../assets/cli-icons/go.svg'

type Brand = { icon: string; color: string }
const brands: Record<string, Brand> = {
  Homebrew: { icon: homebrew, color: 'bg-amber-500' },
  npm: { icon: npm, color: 'bg-red-600' },
  Conda: { icon: anaconda, color: 'bg-green-600' },
  Cargo: { icon: rust, color: 'bg-foreground' },
  Bun: { icon: bun, color: 'bg-amber-400' },
  uv: { icon: uv, color: 'bg-fuchsia-500' },
  Go: { icon: go, color: 'bg-sky-500' },
}

export function CLIIcon({ owner, className = 'size-6' }: { owner: string; className?: string }) {
  const brand = brands[owner]
  if (brand) {
    const mask = `url("${brand.icon}") center / contain no-repeat`
    return <span aria-hidden="true" className={`${className} inline-block shrink-0 ${brand.color}`} style={{ mask, WebkitMask: mask }} />
  }
  const Icon: LucideIcon = owner === 'Standalone' || owner === 'App 内置' ? AppWindow : Terminal
  return <span className={`${className} inline-flex shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground`} aria-hidden="true"><Icon className="size-[58%]" strokeWidth={1.8} /></span>
}
