import { useEffect, useRef, useState } from 'react'
import { AgentIcon } from './AgentIcon'
import styles from '../App.module.css'
import { t } from '../i18n'
import type { Root } from '../types'

const AVATAR_SIZE = 22
const AVATAR_STEP = 16 // 22px avatar with -6px overlap
const MAX_VISIBLE = 8

export function AgentAvatarGroup({ roots }: { roots: Root[] }) {
  const ref = useRef<HTMLDivElement>(null)
  const [visibleCount, setVisibleCount] = useState(4)

  useEffect(() => {
    const node = ref.current
    if (!node) return
    const measure = () => {
      const width = node.clientWidth
      if (width <= 0) return
      const fitsAll = AVATAR_SIZE + Math.max(0, roots.length - 1) * AVATAR_STEP <= width
      if (fitsAll) {
        setVisibleCount(Math.min(roots.length, MAX_VISIBLE))
        return
      }
      const fits = Math.floor((width - AVATAR_SIZE) / AVATAR_STEP)
      setVisibleCount(Math.max(1, Math.min(MAX_VISIBLE, fits)))
    }
    measure()
    if (typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver(measure)
    observer.observe(node)
    return () => observer.disconnect()
  }, [roots.length])

  if (roots.length === 0) return <span className={styles.visibilityNone}>—</span>
  const shown = roots.slice(0, visibleCount)
  const rest = roots.length - shown.length
  return <div ref={ref} className={styles.agentAvatarGroup} title={roots.map((root) => root.label).join('、')} aria-label={t('common.discoverable', { list: roots.map((root) => root.label).join('、') })}>
    {shown.map((root) => <span key={root.id} className={styles.agentAvatar}><AgentIcon name={root.logicalAgent || root.label} className="size-3.5" /></span>)}
    {rest > 0 && <span className={styles.agentAvatarMore}>+{rest}</span>}
  </div>
}
