import mcpLogo from '@lobehub/icons-static-svg/icons/mcp.svg'
import figma from '@lobehub/icons-static-svg/icons/figma-color.svg'
import github from '@lobehub/icons-static-svg/icons/github.svg'
import notion from '@lobehub/icons-static-svg/icons/notion.svg'
import browserless from '@lobehub/icons-static-svg/icons/browserless.svg'
import cloudflare from '@lobehub/icons-static-svg/icons/cloudflare.svg'
import vercel from '@lobehub/icons-static-svg/icons/vercel.svg'
import exa from '@lobehub/icons-static-svg/icons/exa-color.svg'
import playwright from '../assets/mcp/playwright.svg'
import chrome from '../assets/mcp/chrome.svg'
import context7 from '../assets/mcp/context7.png'
import deepwiki from '../assets/mcp/deepwiki.png'
import supabase from '../assets/mcp/supabase.png'
import linear from '../assets/mcp/linear.jpg'
import sentry from '../assets/mcp/sentry.png'
import stripe from '../assets/mcp/stripe.png'
import neon from '../assets/mcp/neon.png'
import atlassian from '../assets/mcp/atlassian.png'
import huggingface from '@lobehub/icons-static-svg/icons/huggingface-color.svg'
import microsoft from '@lobehub/icons-static-svg/icons/microsoft-color.svg'

const brands: Array<{ pattern: RegExp; icon: string; color?: string }> = [
  { pattern: /figma/, icon: figma },
  { pattern: /github/, icon: github },
  { pattern: /notion/, icon: notion },
  { pattern: /browserless/, icon: browserless },
  { pattern: /cloudflare/, icon: cloudflare },
  { pattern: /vercel/, icon: vercel },
  { pattern: /playwright/, icon: playwright, color: 'brand' },
  { pattern: /chrome-devtools/, icon: chrome, color: 'brand' },
  { pattern: /context7/, icon: context7, color: 'brand' },
  { pattern: /deepwiki/, icon: deepwiki, color: 'brand' },
  { pattern: /exa/, icon: exa, color: 'brand' },
  { pattern: /supabase/, icon: supabase, color: 'brand' },
  { pattern: /linear/, icon: linear, color: 'brand' },
  { pattern: /sentry/, icon: sentry, color: 'brand' },
  { pattern: /stripe/, icon: stripe, color: 'brand' },
  { pattern: /neon/, icon: neon, color: 'brand' },
  { pattern: /atlassian/, icon: atlassian, color: 'brand' },
  { pattern: /huggingface/, icon: huggingface, color: 'brand' },
  { pattern: /microsoft/, icon: microsoft, color: 'brand' },
]

export function MCPIcon({ name = '', command = '', url = '', className = 'size-4' }: { name?: string; command?: string; url?: string; className?: string }) {
  const text = `${name} ${command} ${url}`.toLowerCase()
  const brand = brands.find((item) => item.pattern.test(text))
  if (brand && brand.color) {
    return <img src={brand.icon} alt="" aria-hidden="true" loading="lazy" decoding="async" className={`${className} shrink-0`} />
  }
  if (brand) {
    const mask = `url("${brand.icon}") center / contain no-repeat`
    return <span aria-hidden="true" className={`${className} inline-block shrink-0 bg-current`} style={{ mask, WebkitMask: mask }} />
  }
  const mask = `url("${mcpLogo}") center / contain no-repeat`
  return <span aria-hidden="true" className={`${className} inline-block shrink-0 bg-current`} style={{ mask, WebkitMask: mask }} />
}
