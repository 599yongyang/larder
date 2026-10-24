import styles from '../App.module.css'
import { t } from '../i18n'

export function InitialSkeleton() {
  return <section className={styles.ledgerPanel} aria-busy="true" aria-label={t('app.loading')}>
    <div className={styles.metrics}>
      {[0, 1, 2, 3].map((item) => <div key={item} className={styles.metric}><span className={styles.skeletonBar} style={{ width: 48 }} /><strong className={styles.skeletonBar} style={{ width: 34, height: 22 }} /><small className={styles.skeletonBar} style={{ width: 72 }} /></div>)}
    </div>
    <div className={styles.filterBar}>
      <div className={styles.filterLeft}><span className={`${styles.skeletonBar} ${styles.skeletonFilter}`} /><span className={`${styles.skeletonBar} ${styles.skeletonFilterShort}`} /><span className={`${styles.skeletonBar} ${styles.skeletonFilterShort}`} /></div>
    </div>
    <div className={styles.skeletonRows}>
      {[0, 1, 2, 3, 4, 5].map((row) => <div key={row} className={styles.skeletonRow}>
        <span className={styles.skeletonBar} style={{ width: 120, height: 12 }} />
        <span className={styles.skeletonBar} style={{ width: 80 }} />
        <span className={styles.skeletonBar} style={{ width: 74 }} />
        <span className={styles.skeletonBar} style={{ width: 42 }} />
      </div>)}
    </div>
  </section>
}
