import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { DoorOpen, LockKeyhole, Plus, Trophy, X } from 'lucide-react'
import type { GameID, GameRoom } from './gameTypes'

export type RoomDraft = Pick<GameRoom, 'name' | 'maxPlayers' | 'rounds' | 'roundSeconds' | 'recordStats'> & { gameID: GameID; password: string }

const GAME_OPTIONS: Record<GameID, { title: string; room: string; note: string; players: number[]; rounds: number[] }> = {
  'drawing-guess': { title: '你画我猜', room: '画室', note: '轮流画画和猜词，适合轻松聚会。', players: [2, 4, 6, 8], rounds: [3, 5, 7] },
  'dou-dizhu': { title: '斗地主', room: '牌局', note: '标准三人叫分与出牌，支持练习玩家。', players: [3], rounds: [1, 3, 5] },
  'da-vinci-code': { title: '达芬奇密码', room: '密码局', note: '观察牌序并破解对手隐藏的数字。', players: [2, 3, 4], rounds: [1, 3, 5] },
}

function DialogShell({ title, note, onClose, children }: { title: string; note: string; onClose: () => void; children: ReactNode }) {
  useEffect(() => {
    function closeOnEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', closeOnEscape)
    return () => window.removeEventListener('keydown', closeOnEscape)
  }, [onClose])

  return createPortal(<div className="game-dialog-backdrop" role="presentation" onMouseDown={event => event.target === event.currentTarget && onClose()}>
    <section className="game-dialog" role="dialog" aria-modal="true" aria-labelledby="game-dialog-title">
      <header>
        <div><h2 id="game-dialog-title">{title}</h2><p>{note}</p></div>
        <button className="icon-button" type="button" onClick={onClose} aria-label="关闭弹窗"><X size={18} /></button>
      </header>
      {children}
    </section>
  </div>, document.body)
}

export function ScoringRules({ compact = false }: { compact?: boolean }) {
  return <section className={`scoring-rules ${compact ? 'compact' : ''}`}>
    <header><Trophy size={16} /><div><strong>积分规则</strong><span>速度越快，双方奖励越高</span></div></header>
    <div className="scoring-rule-grid">
      <p><b>猜中玩家</b><span>100 + 剩余秒数 × 10</span></p>
      <p><b>本回合画手</b><span>有人猜中：60 + 剩余秒数 × 5</span></p>
      <p><b>其他猜词玩家</b><span>有人猜中时获得 20 分</span></p>
      <p><b>无人猜中</b><span>画手 30 分，猜词玩家 10 分</span></p>
    </div>
  </section>
}

export function GameRulesDialog({ onClose }: { onClose: () => void }) {
  return <DialogShell title="你画我猜积分规则" note="每一回合独立结算，并累计为本场总分。" onClose={onClose}>
    <ScoringRules />
    <footer className="rules-dialog-footer"><button className="button primary" type="button" onClick={onClose}>知道了</button></footer>
  </DialogShell>
}

export function CreateRoomDialog({ username, initialGameID, onClose, onCreate }: { username: string; initialGameID: GameID; onClose: () => void; onCreate: (draft: RoomDraft) => Promise<void> }) {
  const [draft, setDraft] = useState<RoomDraft>({
    gameID: initialGameID,
    name: `${username}的${GAME_OPTIONS[initialGameID].room}`,
    maxPlayers: GAME_OPTIONS[initialGameID].players[0],
    rounds: GAME_OPTIONS[initialGameID].rounds[0],
    roundSeconds: 60,
    password: '',
    recordStats: true,
  })
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  async function submit(event: FormEvent) {
    event.preventDefault()
    setSubmitting(true)
    setError('')
    try {
      await onCreate({ ...draft, name: draft.name.trim() || `${username}的${GAME_OPTIONS[draft.gameID].room}` })
    } catch (value) {
      setError((value as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  const game = GAME_OPTIONS[draft.gameID]

  function changeGame(gameID: GameID) {
    const next = GAME_OPTIONS[gameID]
    setDraft(current => ({ ...current, gameID, name: `${username}的${next.room}`, maxPlayers: next.players[0], rounds: next.rounds[0], roundSeconds: 60 }))
  }

  return <DialogShell title={`创建${game.title}房间`} note={game.note} onClose={onClose}>
    <form className="game-form" onSubmit={submit}>
      <label>选择游戏<select value={draft.gameID} onChange={event => changeGame(event.target.value as GameID)}><option value="drawing-guess">你画我猜</option><option value="dou-dizhu">斗地主</option><option value="da-vinci-code">达芬奇密码</option></select></label>
      <label>房间名称<input value={draft.name} maxLength={24} onChange={event => setDraft(current => ({ ...current, name: event.target.value }))} autoFocus /></label>
      <div className="game-form-grid">
        <label>房间人数<select value={draft.maxPlayers} onChange={event => setDraft(current => ({ ...current, maxPlayers: Number(event.target.value) }))}>{game.players.map(value => <option value={value} key={value}>{value} 人</option>)}</select></label>
        <label>对局数量<select value={draft.rounds} onChange={event => setDraft(current => ({ ...current, rounds: Number(event.target.value) }))}>{game.rounds.map(value => <option value={value} key={value}>{value} {draft.gameID === 'drawing-guess' ? '回合' : '局'}</option>)}</select></label>
        <label>操作时间<select value={draft.roundSeconds} onChange={event => setDraft(current => ({ ...current, roundSeconds: Number(event.target.value) }))}><option value={30}>30 秒</option><option value={45}>45 秒</option><option value={60}>60 秒</option><option value={90}>90 秒</option><option value={120}>2 分钟</option><option value={180}>3 分钟</option></select></label>
        <label>房间密码<span className="input-with-icon"><LockKeyhole size={14} /><input type="password" value={draft.password} maxLength={16} placeholder="可不填" onChange={event => setDraft(current => ({ ...current, password: event.target.value }))} /></span></label>
      </div>
      <label className="record-setting"><input type="checkbox" checked={draft.recordStats} onChange={event => setDraft(current => ({ ...current, recordStats: event.target.checked }))} /><span className="record-switch" aria-hidden="true"><i /></span><span><strong>记录本场战绩</strong><small>默认开启；关闭后本场积分不会进入个人战绩。</small></span></label>
      {draft.gameID === 'drawing-guess' ? <ScoringRules compact /> : <section className="scoring-rules compact"><header><Trophy size={16} /><div><strong>{draft.gameID === 'dou-dizhu' ? '牌局计分' : '破解计分'}</strong><span>{draft.gameID === 'dou-dizhu' ? '叫分、炸弹、王炸和春天会影响倍率' : '每猜中一张得 10 分，最终胜者额外得 100 分'}</span></div></header></section>}
      {error ? <p className="game-form-error" role="alert">{error}</p> : null}
      <footer><button className="button secondary" type="button" onClick={onClose} disabled={submitting}>取消</button><button className="button primary" type="submit" disabled={submitting}><Plus size={15} />{submitting ? '创建中…' : '创建房间'}</button></footer>
    </form>
  </DialogShell>
}

export function JoinRoomDialog({ onClose, onJoin }: { onClose: () => void; onJoin: (code: string, password: string) => Promise<void> }) {
  const [code, setCode] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    const normalized = code.trim().toUpperCase()
    if (normalized.length !== 6) {
      setError('邀请码应为 6 位字符')
      return
    }
    setSubmitting(true)
    setError('')
    try {
      await onJoin(normalized, password)
    } catch (value) {
      setError((value as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  return <DialogShell title="加入好友房间" note="输入好友发来的邀请码；私密房间还需要密码。" onClose={onClose}>
    <form className="game-form" onSubmit={submit}>
      <label>房间邀请码<input className="room-code-input" value={code} maxLength={6} placeholder="例如 SAKURA" onChange={event => { setCode(event.target.value.replace(/[^a-z0-9]/gi, '')); setError('') }} autoFocus /></label>
      <label>房间密码<span className="input-with-icon"><LockKeyhole size={14} /><input type="password" value={password} maxLength={16} placeholder="公开房间可不填" onChange={event => setPassword(event.target.value)} /></span></label>
      {error ? <p className="game-form-error" role="alert">{error}</p> : null}
      <footer><button className="button secondary" type="button" onClick={onClose} disabled={submitting}>取消</button><button className="button primary" type="submit" disabled={submitting}><DoorOpen size={15} />{submitting ? '加入中…' : '进入房间'}</button></footer>
    </form>
  </DialogShell>
}
