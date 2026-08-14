import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react'
import { Check, Circle, Paperclip, Plus, Search, Trash2 } from 'lucide-react'
import { request } from '../../shared/api'
import { formatBytes, formatDateTime } from '../../shared/format'
import type { Memo } from '../../shared/types'

type Filter = 'all' | 'open' | 'done'

export function MemoPage({ onToast }: { onToast: (message: string) => void }) {
  const [memos, setMemos] = useState<Memo[]>([])
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [files, setFiles] = useState<File[]>([])
  const [filter, setFilter] = useState<Filter>('all')
  const [search, setSearch] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    const payload = await request<{ memos: Memo[] }>('/api/memos')
    setMemos(payload.memos || [])
  }, [])

  useEffect(() => { load().catch(error => onToast(error.message)) }, [load, onToast])

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = event.currentTarget
    setBusy(true)
    try {
      const data = new FormData()
      data.append('title', title); data.append('description', description); data.append('status', 'open')
      files.forEach(file => data.append('attachments', file))
      const memo = await request<Memo>('/api/memos', { method: 'POST', body: data })
      setMemos(current => [memo, ...current]); setTitle(''); setDescription(''); setFiles([])
      form.reset()
      onToast('备忘录已保存')
    } catch (error) { onToast((error as Error).message) } finally { setBusy(false) }
  }

  async function toggle(memo: Memo) {
    const status = memo.status === 'done' ? 'open' : 'done'
    try {
      const next = await request<Memo>(`/api/memos/${memo.id}`, { method: 'PATCH', body: JSON.stringify({ title: memo.title, description: memo.description, status }) })
      setMemos(current => current.map(item => item.id === memo.id ? next : item))
      onToast(status === 'done' ? '已标记完成' : '已恢复进行中')
    } catch (error) { onToast((error as Error).message) }
  }

  async function remove(memo: Memo) {
    if (!window.confirm(`确定删除“${memo.title}”吗？`)) return
    try { await request(`/api/memos/${memo.id}`, { method: 'DELETE' }); setMemos(current => current.filter(item => item.id !== memo.id)); onToast('备忘录已删除') }
    catch (error) { onToast((error as Error).message) }
  }

  const visible = useMemo(() => memos.filter(memo => (filter === 'all' || memo.status === filter) && (!search || `${memo.title} ${memo.description}`.toLowerCase().includes(search.toLowerCase()))), [memos, filter, search])

  return <div className="memo-page">
    <section className="page-heading"><div><p className="overline">今天</p><h1>我的备忘录</h1></div><time>{new Intl.DateTimeFormat('zh-CN', { dateStyle: 'full' }).format(new Date())}</time></section>
    <form className="composer" onSubmit={create}>
      <div className="composer-head"><span className="composer-icon"><Plus size={17} /></span><div><h2>新建备忘录</h2><span>把正在发生的事放进小屋</span></div></div>
      <input className="title-input" placeholder="标题" value={title} onChange={event => setTitle(event.target.value)} maxLength={120} required />
      <textarea placeholder="具体描述…" value={description} onChange={event => setDescription(event.target.value)} rows={3} />
      {files.length ? <div className="selected-files">{files.map(file => <span key={`${file.name}-${file.size}`}>{file.name} · {formatBytes(file.size)}</span>)}</div> : null}
      <div className="composer-footer"><label className="file-button"><Paperclip size={14} />添加附件<input type="file" multiple onChange={event => setFiles([...event.target.files || []])} /></label><button className="button primary" disabled={busy}>{busy ? '保存中…' : '保存备忘录'}</button></div>
    </form>
    <section>
      <div className="list-toolbar"><div><h2>备忘录列表</h2><div className="tabs">{(['all', 'open', 'done'] as const).map(key => <button type="button" key={key} className={filter === key ? 'active' : ''} onClick={() => setFilter(key)}>{key === 'all' ? '全部' : key === 'open' ? '进行中' : '已完成'} <b>{key === 'all' ? memos.length : memos.filter(memo => memo.status === key).length}</b></button>)}</div></div><label className="search-field"><Search size={15} /><input className="search-input" placeholder="搜索标题或描述" value={search} onChange={event => setSearch(event.target.value)} /></label></div>
      <div className="memo-list">{visible.map(memo => <article className={`memo-card ${memo.status === 'done' ? 'done' : ''}`} key={memo.id}><div className="memo-top"><h3>{memo.title}</h3><button className="status-toggle" title={memo.status === 'done' ? '恢复进行中' : '标记完成'} onClick={() => toggle(memo)}>{memo.status === 'done' ? <Check size={14} /> : <Circle size={14} />}</button></div><p>{memo.description || '无补充描述'}</p>{memo.attachments?.length ? <div className="attachment-list">{memo.attachments.map(file => <a key={file.id} href={`/api/attachments/${file.id}`} target="_blank" rel="noreferrer">{file.original_name}</a>)}</div> : null}<div className="memo-bottom"><time>{formatDateTime(memo.created_at)}</time><button className="small-action" title="删除" onClick={() => remove(memo)}><Trash2 size={14} /></button></div></article>)}</div>
      {visible.length === 0 ? <div className="empty-state"><Circle size={34} strokeWidth={1.2} /><h3>{memos.length ? '没有匹配的备忘录' : '还没有备忘录'}</h3><p>{memos.length ? '换一个关键词或状态试试。' : '先写下今天的第一件事。'}</p></div> : null}
    </section>
  </div>
}
