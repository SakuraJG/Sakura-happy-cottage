import { useEffect, useMemo, useState, type ChangeEvent, type CSSProperties, type FormEvent } from 'react'
import {
  ArrowRight, CalendarDays, Check, CheckCircle2, ChevronDown, ChevronLeft,
  ChevronUp, ClipboardList, Clock3, FileText, GitBranch, Image as ImageIcon,
  Inbox, Paperclip, Pencil, Plus, Search, SlidersHorizontal, Sparkles, Tag, Trash2, X, Zap,
} from 'lucide-react'

type Priority = 'high' | 'medium' | 'low'
type TaskStatus = 'todo' | 'doing' | 'done'

type TaskAttachment = {
  id: string
  name: string
  type: string
  size: number
  data?: string
}

type Workflow = {
  id: string
  name: string
  color: string
  description: string
}

type WorkflowTask = {
  id: string
  title: string
  description: string
  priority: Priority
  status: TaskStatus
  workflowId: string
  dueDate: string
  order: number
  tags: string[]
  attachments: TaskAttachment[]
}

type TaskDraft = Omit<WorkflowTask, 'id' | 'order'> & { id?: string }
type ViewFilter = 'all' | 'today' | 'high'

const TASKS_KEY = 'sakura-workflow-tasks-v1'
const WORKFLOWS_KEY = 'sakura-workflow-workflows-v1'
const workflowColors = ['#cb6e53', '#628d78', '#7a87ae', '#bf8a3e', '#8c789d']

const defaultWorkflows: Workflow[] = [
  { id: 'inbox', name: '收集箱', color: '#9aa7a0', description: '还没有决定归属的事情' },
  { id: 'personal', name: '个人计划', color: '#cb6e53', description: '日常与生活安排' },
  { id: 'project', name: '项目推进', color: '#628d78', description: '需要持续推进的目标' },
]

const defaultTasks: WorkflowTask[] = [
  { id: 'task-1', title: '确定秋季旅行目的地', description: '比较交通、住宿和天气，周末前做出决定。', priority: 'high', status: 'doing', workflowId: 'personal', dueDate: '2026-08-27', order: 0, tags: ['生活', '决策'], attachments: [] },
  { id: 'task-2', title: '整理作品集首页', description: '更新自我介绍，并挑选三件最满意的作品。', priority: 'high', status: 'todo', workflowId: 'project', dueDate: '2026-08-29', order: 0, tags: ['作品集'], attachments: [] },
  { id: 'task-3', title: '给妈妈打电话', description: '问问最近的睡眠和周末是否有空。', priority: 'medium', status: 'todo', workflowId: 'personal', dueDate: '2026-08-26', order: 1, tags: ['家庭'], attachments: [] },
  { id: 'task-4', title: '阅读《设计中的设计》', description: '完成第二章，记下三个对当前项目有启发的观点。', priority: 'low', status: 'doing', workflowId: 'personal', dueDate: '2026-08-30', order: 2, tags: ['阅读'], attachments: [] },
  { id: 'task-5', title: '提交项目周报', description: '汇总本周进展、风险和下周计划。', priority: 'medium', status: 'done', workflowId: 'project', dueDate: '2026-08-25', order: 0, tags: ['工作'], attachments: [] },
  { id: 'task-6', title: '整理桌面文件', description: '将下载文件夹归档，删除重复素材。', priority: 'low', status: 'done', workflowId: 'inbox', dueDate: '2026-08-24', order: 0, tags: ['整理'], attachments: [] },
]

const statusColumns: { key: TaskStatus; label: string; hint: string }[] = [
  { key: 'todo', label: '待处理', hint: '下一步要做什么' },
  { key: 'doing', label: '进行中', hint: '正在推进的事情' },
  { key: 'done', label: '已完成', hint: '已经落地的成果' },
]

function readStored<T>(key: string, fallback: T): T {
  try {
    const value = window.localStorage.getItem(key)
    return value ? JSON.parse(value) as T : fallback
  } catch {
    return fallback
  }
}

function formatBytes(size: number) {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${Math.round(size / 1024)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

function formatDueDate(date: string) {
  const today = new Date().toISOString().slice(0, 10)
  const tomorrow = new Date(Date.now() + 86400000).toISOString().slice(0, 10)
  if (date === today) return '今天'
  if (date === tomorrow) return '明天'
  return new Intl.DateTimeFormat('zh-CN', { month: 'short', day: 'numeric' }).format(new Date(`${date}T12:00:00`))
}

function makeDraft(task?: WorkflowTask, workflowId = 'personal'): TaskDraft {
  return task
    ? { ...task, tags: [...task.tags], attachments: [...task.attachments] }
    : { title: '', description: '', priority: 'medium', status: 'todo', workflowId, dueDate: new Date().toISOString().slice(0, 10), tags: [], attachments: [] }
}

function isToday(date: string) {
  return date === new Date().toISOString().slice(0, 10)
}

export function WorkflowPage({ onToast }: { onToast: (message: string) => void }) {
  const [tasks, setTasks] = useState<WorkflowTask[]>(() => readStored(TASKS_KEY, defaultTasks))
  const [workflows, setWorkflows] = useState<Workflow[]>(() => readStored(WORKFLOWS_KEY, defaultWorkflows))
  const [selectedWorkflow, setSelectedWorkflow] = useState('all')
  const [filter, setFilter] = useState<ViewFilter>('all')
  const [search, setSearch] = useState('')
  const [sort, setSort] = useState<'order' | 'priority' | 'due'>('order')
  const [editorOpen, setEditorOpen] = useState(false)
  const [draft, setDraft] = useState<TaskDraft | null>(null)
  const [preview, setPreview] = useState<TaskAttachment | null>(null)
  const [newWorkflowOpen, setNewWorkflowOpen] = useState(false)
  const [newWorkflowName, setNewWorkflowName] = useState('')

  useEffect(() => {
    try { window.localStorage.setItem(TASKS_KEY, JSON.stringify(tasks)) } catch { /* Storage may be full when many files are attached. */ }
  }, [tasks])
  useEffect(() => {
    try { window.localStorage.setItem(WORKFLOWS_KEY, JSON.stringify(workflows)) } catch { /* Keep the current session usable. */ }
  }, [workflows])
  useEffect(() => {
    if (!editorOpen && !preview) return
    const close = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      if (preview) setPreview(null)
      else { setEditorOpen(false); setDraft(null) }
    }
    window.addEventListener('keydown', close)
    return () => window.removeEventListener('keydown', close)
  }, [editorOpen, preview])

  const counts = useMemo(() => ({
    total: tasks.length,
    open: tasks.filter(task => task.status !== 'done').length,
    high: tasks.filter(task => task.priority === 'high' && task.status !== 'done').length,
    done: tasks.filter(task => task.status === 'done').length,
  }), [tasks])

  const visibleTasks = useMemo(() => {
    const query = search.trim().toLowerCase()
    const result = tasks.filter(task => {
      const inWorkflow = selectedWorkflow === 'all' || task.workflowId === selectedWorkflow
      const inFilter = filter === 'all' || (filter === 'today' && isToday(task.dueDate)) || (filter === 'high' && task.priority === 'high')
      const inSearch = !query || `${task.title} ${task.description} ${task.tags.join(' ')}`.toLowerCase().includes(query)
      return inWorkflow && inFilter && inSearch
    })
    return result.sort((a, b) => {
      if (sort === 'priority') return ({ high: 0, medium: 1, low: 2 }[a.priority] - { high: 0, medium: 1, low: 2 }[b.priority]) || a.order - b.order
      if (sort === 'due') return a.dueDate.localeCompare(b.dueDate) || a.order - b.order
      return a.order - b.order
    })
  }, [filter, search, selectedWorkflow, sort, tasks])

  function openEditor(task?: WorkflowTask) {
    setDraft(makeDraft(task, selectedWorkflow === 'all' ? 'personal' : selectedWorkflow))
    setEditorOpen(true)
  }

  function updateStatus(task: WorkflowTask, status: TaskStatus) {
    const maxOrder = Math.max(-1, ...tasks.filter(item => item.workflowId === task.workflowId && item.status === status).map(item => item.order))
    setTasks(current => current.map(item => item.id === task.id ? { ...item, status, order: maxOrder + 1 } : item))
    onToast(status === 'done' ? '任务已完成' : status === 'doing' ? '任务已移至进行中' : '任务已移回待处理')
  }

  function moveTask(task: WorkflowTask, direction: -1 | 1) {
    const siblings = tasks.filter(item => item.workflowId === task.workflowId && item.status === task.status).sort((a, b) => a.order - b.order)
    const index = siblings.findIndex(item => item.id === task.id)
    const other = siblings[index + direction]
    if (!other) return
    setTasks(current => current.map(item => item.id === task.id ? { ...item, order: other.order } : item.id === other.id ? { ...item, order: task.order } : item))
  }

  function saveTask(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!draft?.title.trim()) return
    if (draft.id) {
      setTasks(current => current.map(task => task.id === draft.id ? { ...task, ...draft, title: draft.title.trim() } as WorkflowTask : task))
      onToast('任务详情已更新')
    } else {
      const next: WorkflowTask = { ...draft, id: `task-${Date.now()}`, title: draft.title.trim(), order: Math.max(-1, ...tasks.filter(task => task.workflowId === draft.workflowId && task.status === draft.status).map(task => task.order)) + 1 } as WorkflowTask
      setTasks(current => [next, ...current])
      onToast('新任务已加入工作流')
    }
    setEditorOpen(false)
    setDraft(null)
  }

  function removeTask(task: WorkflowTask) {
    if (!window.confirm(`确定删除“${task.title}”吗？`)) return
    setTasks(current => current.filter(item => item.id !== task.id))
    onToast('任务已删除')
  }

  function addWorkflow(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const name = newWorkflowName.trim()
    if (!name) return
    const workflow: Workflow = { id: `workflow-${Date.now()}`, name, color: workflowColors[workflows.length % workflowColors.length], description: '自定义工作流' }
    setWorkflows(current => [...current, workflow])
    setSelectedWorkflow(workflow.id)
    setNewWorkflowName('')
    setNewWorkflowOpen(false)
    onToast(`已创建「${name}」工作流`)
  }

  async function attachFiles(event: ChangeEvent<HTMLInputElement>) {
    if (!draft || !event.target.files?.length) return
    const selected = Array.from(event.target.files)
    const accepted = selected.filter(file => file.size <= 5 * 1024 * 1024)
    if (accepted.length !== selected.length) onToast('单个附件不能超过 5 MB')
    const files = await Promise.all(accepted.map(file => new Promise<TaskAttachment>(resolve => {
      const base = { id: `${file.name}-${file.lastModified}-${Math.random()}`, name: file.name, type: file.type || 'application/octet-stream', size: file.size }
      if (!file.type.startsWith('image/')) { resolve(base); return }
      const reader = new FileReader()
      reader.onload = () => resolve({ ...base, data: typeof reader.result === 'string' ? reader.result : undefined })
      reader.onerror = () => resolve(base)
      reader.readAsDataURL(file)
    })))
    setDraft(current => current ? { ...current, attachments: [...current.attachments, ...files] } : current)
    event.target.value = ''
  }

  function addTag() {
    const tag = window.prompt('输入标签名称')?.trim()
    if (tag && draft && !draft.tags.includes(tag)) setDraft({ ...draft, tags: [...draft.tags, tag] })
  }

  return <div className="workflow-page">
    <section className="page-heading workflow-heading">
      <div><p className="overline">计划与执行</p><h1>我的工作流</h1><p className="heading-note">把目标拆成清晰的下一步，让重要的事自然向前。</p></div>
      <div className="workflow-heading-actions"><span className="workflow-sync"><span className="sync-dot" />已在本机保存</span><button className="button primary" onClick={() => openEditor()}><Plus size={16} />新建任务</button></div>
    </section>

    <section className="workflow-summary" aria-label="工作流概览">
      <div className="workflow-stat"><span className="stat-icon stat-icon-accent"><Zap size={16} /></span><div><strong>{counts.open}</strong><span>进行中的任务</span></div><small>保持节奏</small></div>
      <div className="workflow-stat"><span className="stat-icon stat-icon-sage"><ClipboardList size={16} /></span><div><strong>{counts.total}</strong><span>全部任务</span></div><small>{workflows.length} 个工作流</small></div>
      <div className="workflow-stat"><span className="stat-icon stat-icon-blue"><Clock3 size={16} /></span><div><strong>{counts.high}</strong><span>高优先级</span></div><small>需要优先处理</small></div>
      <div className="workflow-stat"><span className="stat-icon stat-icon-gold"><CheckCircle2 size={16} /></span><div><strong>{counts.total ? Math.round(counts.done / counts.total * 100) : 0}%</strong><span>完成率</span></div><small>{counts.done} 项已完成</small></div>
    </section>

    <div className="workflow-layout">
      <aside className="workflow-sidebar">
        <div className="sidebar-label"><span>我的工作流</span><button className="icon-button" title="新建工作流" onClick={() => setNewWorkflowOpen(current => !current)}><Plus size={15} /></button></div>
        {newWorkflowOpen ? <form className="workflow-create" onSubmit={addWorkflow}><input autoFocus placeholder="例如：健康计划" value={newWorkflowName} onChange={event => setNewWorkflowName(event.target.value)} maxLength={30} /><button type="submit" title="创建"><Check size={14} /></button></form> : null}
        <div className="workflow-nav-list">
          <button className={`workflow-nav-item ${selectedWorkflow === 'all' ? 'active' : ''}`} onClick={() => setSelectedWorkflow('all')}><span className="workflow-nav-mark all-mark"><Inbox size={14} /></span><span>所有任务</span><b>{tasks.length}</b></button>
          {workflows.map(workflow => <button className={`workflow-nav-item ${selectedWorkflow === workflow.id ? 'active' : ''}`} key={workflow.id} onClick={() => setSelectedWorkflow(workflow.id)}><span className="workflow-nav-mark" style={{ background: workflow.color }}><GitBranch size={13} /></span><span>{workflow.name}</span><b>{tasks.filter(task => task.workflowId === workflow.id && task.status !== 'done').length || ''}</b></button>)}
        </div>
        <div className="workflow-tip"><Sparkles size={15} /><div><strong>把流程变成习惯</strong><span>先写下下一步，完成它，再移动到下一列。</span></div></div>
      </aside>

      <main className="workflow-main">
        <div className="workflow-toolbar">
          <div className="workflow-filters"><button className={filter === 'all' ? 'active' : ''} onClick={() => setFilter('all')}>全部 <b>{counts.total}</b></button><button className={filter === 'today' ? 'active' : ''} onClick={() => setFilter('today')}><CalendarDays size={14} />今天</button><button className={filter === 'high' ? 'active' : ''} onClick={() => setFilter('high')}><Zap size={14} />高优先级</button></div>
          <div className="workflow-tools"><label className="search-field"><Search size={14} /><input className="search-input" placeholder="搜索任务" value={search} onChange={event => setSearch(event.target.value)} /></label><label className="sort-select"><SlidersHorizontal size={14} /><select value={sort} onChange={event => setSort(event.target.value as typeof sort)}><option value="order">自定义顺序</option><option value="priority">按优先级</option><option value="due">按截止日期</option></select><ChevronDown size={13} /></label></div>
        </div>
        <div className="workflow-board">
          {statusColumns.map(column => {
            const columnTasks = visibleTasks.filter(task => task.status === column.key)
            return <section className={`workflow-column column-${column.key}`} key={column.key}><header className="workflow-column-head"><div><span className="column-dot" /><h2>{column.label}</h2><small>{column.hint}</small></div><b>{columnTasks.length}</b></header><div className="workflow-column-body">{columnTasks.map((task, index) => <TaskCard key={task.id} task={task} index={index} workflow={workflows.find(item => item.id === task.workflowId)} onEdit={() => openEditor(task)} onDelete={() => removeTask(task)} onStatus={status => updateStatus(task, status)} onMove={direction => moveTask(task, direction)} onPreview={setPreview} />)}{columnTasks.length === 0 ? <button className="column-empty" onClick={() => { setDraft(makeDraft(undefined, selectedWorkflow === 'all' ? 'personal' : selectedWorkflow)); setEditorOpen(true) }}><Plus size={15} /><span>添加任务</span></button> : <button className="column-add" onClick={() => { setDraft(makeDraft(undefined, selectedWorkflow === 'all' ? 'personal' : selectedWorkflow)); setEditorOpen(true) }}><Plus size={14} />添加任务</button>}</div></section>
          })}
        </div>
      </main>
    </div>

    {editorOpen && draft ? <div className="workflow-overlay" onMouseDown={event => { if (event.target === event.currentTarget) { setEditorOpen(false); setDraft(null) } }}><aside className="task-drawer"><div className="drawer-head"><div><p className="overline">任务详情</p><h2>{draft.id ? '编辑任务' : '新建任务'}</h2></div><button className="icon-button" title="关闭" onClick={() => { setEditorOpen(false); setDraft(null) }}><X size={18} /></button></div><form className="task-form" onSubmit={saveTask}><label className="task-title-label">任务名称<input autoFocus value={draft.title} onChange={event => setDraft({ ...draft, title: event.target.value })} placeholder="例如：准备周五的分享" maxLength={120} required /></label><label>任务描述<textarea value={draft.description} onChange={event => setDraft({ ...draft, description: event.target.value })} placeholder="补充背景、验收标准或下一步…" rows={4} /></label><div className="field-grid"><label>所属工作流<select value={draft.workflowId} onChange={event => setDraft({ ...draft, workflowId: event.target.value })}>{workflows.map(workflow => <option value={workflow.id} key={workflow.id}>{workflow.name}</option>)}</select></label><label>截止日期<input type="date" value={draft.dueDate} onChange={event => setDraft({ ...draft, dueDate: event.target.value })} /></label></div><fieldset className="priority-picker"><legend>优先级</legend><div>{(['high', 'medium', 'low'] as Priority[]).map(priority => <button type="button" key={priority} className={`priority-choice ${priority} ${draft.priority === priority ? 'active' : ''}`} onClick={() => setDraft({ ...draft, priority })}><span />{priority === 'high' ? '高' : priority === 'medium' ? '中' : '低'}</button>)}</div></fieldset><label>开始状态<select value={draft.status} onChange={event => setDraft({ ...draft, status: event.target.value as TaskStatus })}><option value="todo">待处理</option><option value="doing">进行中</option><option value="done">已完成</option></select></label><div className="task-tags-field"><div className="label-row"><span>标签</span><button type="button" onClick={addTag}><Plus size={13} />添加标签</button></div><div className="draft-tags">{draft.tags.map(tag => <span key={tag}><Tag size={11} />{tag}<button type="button" title="移除标签" onClick={() => setDraft({ ...draft, tags: draft.tags.filter(item => item !== tag) })}><X size={10} /></button></span>)}{draft.tags.length === 0 ? <em>用标签帮助你快速找到任务</em> : null}</div></div><div className="task-attachments-field"><div className="label-row"><span>附件</span><label className="attach-button"><Paperclip size={14} />添加文件<input type="file" multiple onChange={attachFiles} /></label></div>{draft.attachments.length ? <div className="draft-attachments">{draft.attachments.map(file => <div key={file.id} className="draft-attachment"><span className="attachment-thumb">{file.type.startsWith('image/') && file.data ? <img src={file.data} alt="" /> : <FileText size={16} />}</span><span><strong>{file.name}</strong><small>{formatBytes(file.size)}</small></span><button type="button" title="移除附件" onClick={() => setDraft({ ...draft, attachments: draft.attachments.filter(item => item.id !== file.id) })}><Trash2 size={14} /></button></div>)}</div> : <div className="attachment-drop"><Paperclip size={18} /><span>添加图片、文档或其他文件</span><small>单个文件最大 5 MB</small></div>}</div><div className="drawer-footer"><button type="button" className="button secondary" onClick={() => { setEditorOpen(false); setDraft(null) }}>取消</button><button className="button primary"><Check size={15} />保存任务</button></div></form></aside></div> : null}
    {preview ? <div className="image-viewer" role="dialog" aria-label="图片预览" onClick={() => setPreview(null)}><button className="image-viewer-close" title="关闭预览" onClick={() => setPreview(null)}><X size={20} /></button><div className="image-viewer-content" onClick={event => event.stopPropagation()}>{preview.data ? <img src={preview.data} alt={preview.name} /> : <div className="preview-fallback"><ImageIcon size={32} /><p>暂时无法预览这张图片</p></div>}<span>{preview.name}</span></div></div> : null}
  </div>
}

function TaskCard({ task, workflow, index, onEdit, onDelete, onStatus, onMove, onPreview }: { task: WorkflowTask; workflow?: Workflow; index: number; onEdit: () => void; onDelete: () => void; onStatus: (status: TaskStatus) => void; onMove: (direction: -1 | 1) => void; onPreview: (attachment: TaskAttachment) => void }) {
  return <article className={`workflow-task-card priority-${task.priority} ${task.status === 'done' ? 'is-done' : ''}`} style={{ '--task-index': index } as CSSProperties}><div className="task-card-top"><span className={`priority-label ${task.priority}`}><span />{task.priority === 'high' ? '高优先级' : task.priority === 'medium' ? '中优先级' : '低优先级'}</span><div className="task-card-menu"><button className="small-action" title="编辑任务" onClick={onEdit}><Pencil size={14} /></button><button className="small-action danger" title="删除任务" onClick={onDelete}><Trash2 size={14} /></button></div></div><button className="task-card-title" onClick={onEdit}>{task.title}</button>{task.description ? <p>{task.description}</p> : null}<div className="task-card-tags">{workflow ? <span className="task-workflow"><span style={{ background: workflow.color }} />{workflow.name}</span> : null}{task.tags.slice(0, 2).map(tag => <span className="task-tag" key={tag}><Tag size={10} />{tag}</span>)}</div>{task.attachments.length ? <div className="task-card-attachments">{task.attachments.slice(0, 3).map(file => file.type.startsWith('image/') && file.data ? <button className="attachment-preview" title={`预览 ${file.name}`} key={file.id} onClick={() => onPreview(file)}><img src={file.data} alt={file.name} /></button> : <span key={file.id} title={file.name}><FileText size={13} /></span>)}{task.attachments.length > 3 ? <small>+{task.attachments.length - 3}</small> : null}</div> : null}<div className="task-card-bottom"><span className={`task-due ${isToday(task.dueDate) ? 'today' : ''}`}><CalendarDays size={12} />{formatDueDate(task.dueDate)}</span><div className="task-card-actions">{task.status !== 'todo' ? <button className="mini-action" title="移回上一列" onClick={() => onStatus(task.status === 'done' ? 'doing' : 'todo')}><ChevronLeft size={14} /></button> : null}<span className="task-order"><button className="order-button" title="上移" onClick={() => onMove(-1)}><ChevronUp size={12} /></button>{index + 1}<button className="order-button" title="下移" onClick={() => onMove(1)}><ChevronDown size={12} /></button></span>{task.status !== 'done' ? <button className="advance-action" title="移至下一列" onClick={() => onStatus(task.status === 'todo' ? 'doing' : 'done')}><ArrowRight size={13} /></button> : <span className="done-mark"><Check size={13} /></span>}</div></div></article>
}
