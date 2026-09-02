import { useCallback, useEffect, useRef, useState, type CSSProperties, type FormEvent, type PointerEvent as ReactPointerEvent } from 'react'
import { ArrowLeft, Brush, Check, CircleHelp, Clock3, Eraser, Highlighter, Info, Lightbulb, Palette, RotateCcw, Send, Sparkles, Trophy, Undo2 } from 'lucide-react'
import type { User } from '../../shared/types'
import { GameRulesDialog } from './RoomDialogs'
import type { DrawingTool, GameRoom, GameStroke } from './gameTypes'

type MatchResult = { result: 'win' | 'complete'; role: 'drawer' | 'guesser'; durationSeconds: number; score: number }

const COLORS = ['#202925', '#cb6e53', '#517966', '#e6a84f', '#5590a8', '#956d9d']

function drawStroke(context: CanvasRenderingContext2D, stroke: GameStroke, width: number, height: number) {
  const [first, ...rest] = stroke.points
  if (!first) return
  context.beginPath()
  context.save()
  context.lineCap = 'round'
  context.lineJoin = 'round'
  context.globalCompositeOperation = stroke.tool === 'eraser' ? 'destination-out' : 'source-over'
  context.globalAlpha = stroke.opacity || 1
  context.strokeStyle = stroke.color
  context.lineWidth = stroke.width * Math.max(0.65, width / 800)
  context.moveTo(first.x * width, first.y * height)
  for (const point of rest) context.lineTo(point.x * width, point.y * height)
  context.stroke()
  context.restore()
}

export function DrawingGuessGame({ room, user, onBack, onRecord, onStroke, onUndo, onClear, onMessage, onLock, onFinish, onNextRound }: {
  room: GameRoom
  user: User
  onBack: () => void
  onRecord: (result: MatchResult) => Promise<void>
  onStroke: (stroke: Omit<GameStroke, 'id'>) => Promise<void>
  onUndo: () => Promise<void>
  onClear: () => Promise<void>
  onMessage: (text: string) => Promise<void>
  onLock: () => Promise<void>
  onFinish: () => Promise<void>
  onNextRound: () => Promise<void>
}) {
  const isDrawer = room.drawerUID === user.uid
  const showAnswer = isDrawer || room.roundComplete
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const drawingRef = useRef<GameStroke | null>(null)
  const recorded = useRef(false)
  const finishRequested = useRef(false)
  const [strokes, setStrokes] = useState<GameStroke[]>(room.strokes || [])
  const [color, setColor] = useState(COLORS[0])
  const [lineWidth, setLineWidth] = useState(7)
  const [tool, setTool] = useState<DrawingTool>('pen')
  const [secondsLeft, setSecondsLeft] = useState(room.secondsLeft || room.roundSeconds)
  const [message, setMessage] = useState('')
  const [result, setResult] = useState<MatchResult | null>(null)
  const [saving, setSaving] = useState(false)
  const [showRules, setShowRules] = useState(false)

  useEffect(() => setStrokes(room.strokes || []), [room.strokes])
  useEffect(() => {
    if (room.roundComplete || Math.abs((room.secondsLeft || room.roundSeconds) - secondsLeft) > 2) setSecondsLeft(room.secondsLeft || 0)
  }, [room.roundComplete, room.secondsLeft, room.roundSeconds, secondsLeft])

  const renderCanvas = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const rect = canvas.getBoundingClientRect()
    const ratio = Math.min(window.devicePixelRatio || 1, 2)
    canvas.width = Math.max(1, Math.floor(rect.width * ratio))
    canvas.height = Math.max(1, Math.floor(rect.height * ratio))
    const context = canvas.getContext('2d')
    if (!context) return
    context.setTransform(ratio, 0, 0, ratio, 0, 0)
    context.clearRect(0, 0, rect.width, rect.height)
    for (const stroke of strokes) drawStroke(context, stroke, rect.width, rect.height)
  }, [strokes])

  useEffect(() => {
    renderCanvas()
    const observer = new ResizeObserver(renderCanvas)
    if (canvasRef.current) observer.observe(canvasRef.current)
    return () => observer.disconnect()
  }, [renderCanvas])

  useEffect(() => {
    if (room.roundComplete || result) return
    const timer = window.setInterval(() => setSecondsLeft(current => Math.max(0, current - 1)), 1000)
    return () => window.clearInterval(timer)
  }, [result, room.roundComplete])

  useEffect(() => {
    if (secondsLeft !== 0 || room.roundComplete || finishRequested.current) return
    finishRequested.current = true
    void onFinish().catch(() => undefined)
  }, [onFinish, room.roundComplete, secondsLeft])

  useEffect(() => {
    if (!room.roundComplete || recorded.current) return
    recorded.current = true
    const durationSeconds = Math.max(1, room.roundSeconds - Math.max(0, room.secondsLeft))
    const next: MatchResult = {
      result: room.winnerUID === user.uid ? 'win' : 'complete',
      role: isDrawer ? 'drawer' : 'guesser',
      durationSeconds,
      score: room.roundScore || 40,
    }
    setResult(next)
    setSaving(true)
    void onRecord(next).finally(() => setSaving(false))
  }, [isDrawer, onRecord, room.roundComplete, room.roundScore, room.roundSeconds, room.secondsLeft, room.winnerUID, user.uid])

  function canvasPoint(event: ReactPointerEvent<HTMLCanvasElement>) {
    const rect = event.currentTarget.getBoundingClientRect()
    return { x: Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width)), y: Math.min(1, Math.max(0, (event.clientY - rect.top) / rect.height)) }
  }

  function startStroke(event: ReactPointerEvent<HTMLCanvasElement>) {
    if (!isDrawer || room.roundComplete || room.drawingLocked) return
    event.currentTarget.setPointerCapture(event.pointerId)
    drawingRef.current = { color, width: lineWidth, tool, opacity: tool === 'marker' ? 0.34 : 1, points: [canvasPoint(event)] }
  }

  function continueStroke(event: ReactPointerEvent<HTMLCanvasElement>) {
    const stroke = drawingRef.current
    const canvas = canvasRef.current
    if (!stroke || !canvas) return
    const point = canvasPoint(event)
    const previous = stroke.points[stroke.points.length - 1]
    stroke.points.push(point)
    const context = canvas.getContext('2d')
    if (!context || !previous) return
    const rect = canvas.getBoundingClientRect()
    const ratio = Math.min(window.devicePixelRatio || 1, 2)
    context.save()
    context.setTransform(ratio, 0, 0, ratio, 0, 0)
    drawStroke(context, { ...stroke, points: [previous, point] }, rect.width, rect.height)
    context.restore()
  }

  function endStroke(event: ReactPointerEvent<HTMLCanvasElement>) {
    const stroke = drawingRef.current
    if (!stroke) return
    event.currentTarget.releasePointerCapture(event.pointerId)
    drawingRef.current = null
    if (stroke.points.length > 1) {
      setStrokes(current => [...current, stroke])
      void onStroke(stroke).catch(() => undefined)
    }
  }

  function submitMessage(event: FormEvent) {
    event.preventDefault()
    const text = message.trim()
    if (!text || room.roundComplete) return
    setMessage('')
    void onMessage(text).catch(() => undefined)
  }

  const handleNextRound = async () => {
    setSaving(true)
    try { await onNextRound() } finally { setSaving(false) }
  }

  function chooseTool(nextTool: DrawingTool) {
    setTool(nextTool)
    if (nextTool === 'marker') setLineWidth(current => Math.max(current, 16))
    if (nextTool === 'eraser') setLineWidth(current => Math.max(current, 22))
    if (nextTool === 'pen') setLineWidth(current => Math.min(current, 12))
  }

  return <div className="drawing-game-page">
    <header className="game-session-header">
      <button className="back-button" onClick={onBack}><ArrowLeft size={15} />返回游戏大厅</button>
      <div className="session-title"><span className="session-round">第 {room.currentRound} / {room.rounds} 回合 · 累计 {room.totalScore || 0} 分</span><strong>{room.name}</strong></div>
      <div className="session-tools"><button className="icon-button" onClick={() => setShowRules(true)} aria-label="查看积分规则" title="积分规则"><Info size={17} /></button><div className={`round-timer ${secondsLeft <= 10 ? 'urgent' : ''}`}><Clock3 size={17} /><strong>{secondsLeft}</strong><span>秒</span></div></div>
    </header>

    <div className="game-session-layout">
      <section className="drawing-stage">
        <div className="word-strip">
          <span>{showAnswer ? <Brush size={16} /> : <CircleHelp size={16} />}{showAnswer ? '本回合题目' : '猜猜画的是什么'}</span>
          <strong>{showAnswer ? room.word : `${room.wordLength || 0} 个字`}</strong>
          <small>{showAnswer ? `${room.wordCategory || '生活'} · 回合已揭晓` : `提示：${room.wordHint || '观察画面中的线索'}`}</small>
        </div>

        <div className={`canvas-wrap ${isDrawer ? '' : 'view-only'} ${room.drawingLocked ? 'drawing-locked' : ''}`}>
          <canvas ref={canvasRef} onPointerDown={startStroke} onPointerMove={continueStroke} onPointerUp={endStroke} onPointerCancel={endStroke} aria-label={isDrawer ? '绘画画布' : '画手的画布'} />
          {strokes.length === 0 ? <div className="canvas-empty"><Brush size={31} strokeWidth={1.35} /><span>{isDrawer ? '在这里开始画' : '等待画手落笔'}</span></div> : null}
          {result ? <div className="round-result-overlay">
            <span className="result-icon">{result.result === 'win' ? <Trophy size={30} /> : <Check size={30} />}</span>
            <p>{result.result === 'win' ? '猜对了' : '本回合完成'}</p>
            <h2>{result.score} 分</h2>
            {room.currentRound < room.rounds && room.hostUID === user.uid ? <button className="button primary" onClick={() => void handleNextRound()} disabled={saving}>{saving ? '正在切换回合' : '下一回合'}</button> : room.currentRound < room.rounds ? <button className="button secondary" disabled>等待房主开始下一回合</button> : <button className="button primary" onClick={onBack} disabled={saving}>{saving ? '正在保存战绩' : '返回游戏大厅'}</button>}
          </div> : null}
        </div>

        {isDrawer && !room.drawingLocked ? <div className="drawing-toolbar" aria-label="绘画工具">
          <div className="tool-modes" role="group" aria-label="画笔类型"><button className={tool === 'pen' ? 'active' : ''} onClick={() => chooseTool('pen')} title="画笔" aria-label="画笔"><Brush size={16} /></button><button className={tool === 'marker' ? 'active' : ''} onClick={() => chooseTool('marker')} title="马克笔" aria-label="马克笔"><Highlighter size={16} /></button><button className={tool === 'eraser' ? 'active' : ''} onClick={() => chooseTool('eraser')} title="橡皮擦" aria-label="橡皮擦"><Eraser size={16} /></button></div>
          <div className="tool-colors"><Palette size={16} />{COLORS.map(option => <button key={option} className={color === option ? 'active' : ''} style={{ '--swatch': option } as CSSProperties} onClick={() => { setColor(option); if (tool === 'eraser') chooseTool('pen') }} aria-label={`选择颜色 ${option}`} />)}<label className="custom-color" title="自定义颜色"><input type="color" value={color} onChange={event => { setColor(event.target.value); chooseTool('pen') }} /><span>+</span></label></div>
          <label className="brush-size"><span><Brush size={15} />粗细 {lineWidth}</span><input type="range" min={2} max={40} value={lineWidth} onChange={event => setLineWidth(Number(event.target.value))} /></label>
          <div className="tool-actions">
            <button className="icon-button" onClick={() => void onUndo().catch(() => undefined)} disabled={strokes.length === 0 || room.roundComplete} aria-label="撤销"><Undo2 size={17} /></button>
            <button className="icon-button" onClick={() => void onClear().catch(() => undefined)} disabled={strokes.length === 0 || room.roundComplete} aria-label="清空画布"><Eraser size={17} /></button>
            <button className="button secondary compact" onClick={() => void onLock().catch(() => undefined)} disabled={room.roundComplete}><Check size={15} />完成作画</button>
          </div>
        </div> : <div className="guessing-status"><Sparkles size={16} /><span>{room.drawingLocked ? '画板已固定，等待猜中或倒计时结束' : '观察线条，尽快猜中题目'}</span><RotateCcw size={15} /></div>}
      </section>

      <aside className="game-chat">
        <div className="chat-heading"><div><h2>房间消息</h2><span>{room.players.length} 人正在游戏</span></div><Lightbulb size={19} /></div>
        <div className="chat-messages" aria-live="polite">
          {(room.messages || []).map(item => <div className={`chat-message ${item.kind || ''}`} key={item.id}><span>{item.kind === 'system' ? <Sparkles size={13} /> : item.author.slice(0, 1)}</span><p><strong>{item.author}</strong>{item.text}</p></div>)}
        </div>
        <form className="chat-composer" onSubmit={submitMessage}>
          <input value={message} onChange={event => setMessage(event.target.value)} placeholder={isDrawer ? '给大家一点提示' : '输入你的答案'} maxLength={60} disabled={room.roundComplete} />
          <button type="submit" disabled={!message.trim() || room.roundComplete} aria-label="发送"><Send size={17} /></button>
        </form>
      </aside>
    </div>
    {showRules ? <GameRulesDialog onClose={() => setShowRules(false)} /> : null}
  </div>
}
