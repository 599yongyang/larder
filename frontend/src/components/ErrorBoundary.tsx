import { Component, type ErrorInfo, type ReactNode } from 'react'

export class ErrorBoundary extends Component<{ children: ReactNode }, { error?: Error }> {
  state: { error?: Error } = {}

  static getDerivedStateFromError(error: Error) { return { error } }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('UI crashed', error, info)
  }

  render() {
    if (this.state.error) {
      return <div style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', padding: 32, background: '#f6f5f2', color: '#222' }}>
        <div style={{ maxWidth: 640 }}>
          <h1 style={{ margin: '0 0 10px', fontSize: 22 }}>界面出错了</h1>
          <p style={{ margin: '0 0 14px', lineHeight: 1.6 }}>页面没有丢失数据。把下面的错误信息发给我，我会继续修。</p>
          <pre style={{ maxHeight: 260, overflow: 'auto', padding: 14, borderRadius: 8, background: '#fff', border: '1px solid #ddd', fontSize: 12, whiteSpace: 'pre-wrap' }}>{this.state.error.stack || this.state.error.message}</pre>
          <button type="button" onClick={() => window.location.reload()} style={{ marginTop: 14, padding: '8px 14px', borderRadius: 6, border: '1px solid #bbb', background: '#fff', cursor: 'pointer' }}>重新加载</button>
        </div>
      </div>
    }
    return this.props.children
  }
}
