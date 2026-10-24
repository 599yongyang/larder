import type { ReactNode } from 'react'
import styles from './MarkdownPreview.module.css'

function inline(text: string, key: string): ReactNode[] {
  return text.split(/(\*\*[^*]+\*\*|`[^`]+`|\[[^\]]+\]\([^)]+\))/g).filter(Boolean).map((part, index) => {
    const id = `${key}-${index}`
    if (part.startsWith('**') && part.endsWith('**')) return <strong key={id}>{part.slice(2, -2)}</strong>
    if (part.startsWith('`') && part.endsWith('`')) return <code key={id}>{part.slice(1, -1)}</code>
    const link = /^\[([^\]]+)\]\(([^)]+)\)$/.exec(part)
    if (link) {
      const safe = /^(https?:\/\/|mailto:)/i.test(link[2])
      return safe ? <a key={id} href={link[2]} target="_blank" rel="noreferrer">{link[1]}</a> : <span key={id}>{link[1]}</span>
    }
    return part
  })
}

export function splitFrontmatter(source: string): { meta: Array<[string, string]>; body: string } {
  if (!source.startsWith('---')) return { meta: [], body: source }
  const lines = source.split('\n')
  const closing = lines.findIndex((line, index) => index > 0 && line.trim() === '---')
  if (closing < 0) return { meta: [], body: source }
  const meta: Array<[string, string]> = []
  for (const line of lines.slice(1, closing)) {
    const match = /^([A-Za-z0-9_-]+):\s*(.*)$/.exec(line.trim())
    if (match) meta.push([match[1], match[2].replace(/^["']|["']$/g, '')])
  }
  return { meta, body: lines.slice(closing + 1).join('\n') }
}

export function MarkdownPreview({ source }: { source: string }) {
  const lines = source.split('\n')
  const nodes: ReactNode[] = []
  let index = 0
  const pushParagraph = (buffer: string[]) => { if (buffer.length > 0) nodes.push(<p key={`p-${index}-${nodes.length}`}>{inline(buffer.join(' '), `p-${nodes.length}`)}</p>) }
  let paragraph: string[] = []
  while (index < lines.length) {
    const line = lines[index]
    if (line.trim() === '') { pushParagraph(paragraph); paragraph = []; index++; continue }
    const fence = /^\s*```(.*)$/.exec(line)
    if (fence) {
      pushParagraph(paragraph); paragraph = []
      const block: string[] = []
      index++
      while (index < lines.length && !/^\s*```/.test(lines[index])) { block.push(lines[index]); index++ }
      index++
      nodes.push(<pre key={`code-${index}`}><code>{block.join('\n')}</code></pre>)
      continue
    }
    const heading = /^(#{1,6})\s+(.*)$/.exec(line)
    if (heading) {
      pushParagraph(paragraph); paragraph = []
      const level = heading[1].length
      const content = inline(heading[2], `h-${index}`)
      nodes.push(level === 1 ? <h1 key={`h-${index}`}>{content}</h1> : level === 2 ? <h2 key={`h-${index}`}>{content}</h2> : level === 3 ? <h3 key={`h-${index}`}>{content}</h3> : level === 4 ? <h4 key={`h-${index}`}>{content}</h4> : <h5 key={`h-${index}`}>{content}</h5>)
      index++
      continue
    }
    if (/^(-{3,}|\*{3,})$/.test(line.trim())) { pushParagraph(paragraph); paragraph = []; nodes.push(<hr key={`hr-${index}`} />); index++; continue }
    const bullet = /^\s*[-*+]\s+(.*)$/.exec(line)
    if (bullet) {
      pushParagraph(paragraph); paragraph = []
      const items: string[] = []
      while (index < lines.length) { const item = /^\s*[-*+]\s+(.*)$/.exec(lines[index]); if (!item) break; items.push(item[1]); index++ }
      nodes.push(<ul key={`ul-${index}`}>{items.map((item, itemIndex) => <li key={itemIndex}>{inline(item, `li-${index}-${itemIndex}`)}</li>)}</ul>)
      continue
    }
    const ordered = /^\s*\d+\.\s+(.*)$/.exec(line)
    if (ordered) {
      pushParagraph(paragraph); paragraph = []
      const items: string[] = []
      while (index < lines.length) { const item = /^\s*\d+\.\s+(.*)$/.exec(lines[index]); if (!item) break; items.push(item[1]); index++ }
      nodes.push(<ol key={`ol-${index}`}>{items.map((item, itemIndex) => <li key={itemIndex}>{inline(item, `li-${index}-${itemIndex}`)}</li>)}</ol>)
      continue
    }
    const quote = /^\s*>\s?(.*)$/.exec(line)
    if (quote) { pushParagraph(paragraph); paragraph = []; nodes.push(<blockquote key={`quote-${index}`}>{inline(quote[1], `q-${index}`)}</blockquote>); index++; continue }
    paragraph.push(line.trim())
    index++
  }
  pushParagraph(paragraph)
  return <div className={styles.markdown}>{nodes}</div>
}
