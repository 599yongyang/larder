import { Search } from 'lucide-react'
import { Input } from '@/components/ui/input'
import styles from '../App.module.css'
import { ariaShortcut } from '../lib/platform'

export function AssetSearch({ value, onChange, placeholder, label, width = 180 }: { value: string; onChange: (value: string) => void; placeholder: string; label: string; width?: number }) {
  return <div className={styles.assetSearch} style={{ width }}><Search size={13} aria-hidden="true" /><Input data-asset-search aria-keyshortcuts={ariaShortcut('F')} className={styles.assetSearchInput} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} aria-label={label} /></div>
}
