import { useEffect, useMemo, useRef, useState, type MouseEvent, type PointerEvent } from 'react'
import { ArrowLeft, Crown, Layers3, Play, RotateCcw, Shield, Timer, Trophy } from 'lucide-react'
import type { User } from '../../shared/types'
import type { GameRoom, PlayingCard } from './gameTypes'
import { canBeatDouDizhu, classifyDouDizhu, comboLabel, type DouDizhuCombo } from './douDizhuRules'

type GameAction = { type: string; bid?: number; cards?: number[] }
type MatchResult = { result: 'win' | 'lose'; role: 'landlord' | 'farmer'; durationSeconds: number; score: number; summary: string }

const SUITS: Record<PlayingCard['suit'], string> = { spade: '♠', heart: '♥', club: '♣', diamond: '♦', joker: '★' }

function Card({ card, selected = false, onClick, onPointerDown, onPointerEnter }: {
  card: PlayingCard
  selected?: boolean
  onClick?: (event: MouseEvent<HTMLButtonElement>) => void
  onPointerDown?: (event: PointerEvent<HTMLButtonElement>) => void
  onPointerEnter?: () => void
}) {
  const red = card.suit === 'heart' || card.suit === 'diamond' || card.rank === '大王'
  return <button type="button" data-card-id={card.id} className={`playing-card ${red ? 'red' : ''} ${selected ? 'selected' : ''}`} onClick={onClick} onPointerDown={onPointerDown} onPointerEnter={onPointerEnter} disabled={!onClick} aria-label={`${card.rank}${SUITS[card.suit]}`}>
    <span>{card.rank}</span><b>{SUITS[card.suit]}</b>
  </button>
}

export function DouDizhuGame({ room, user, onBack, onAction, onNextRound, onRecord }: {
  room: GameRoom
  user: User
  onBack: () => void
  onAction: (action: GameAction) => Promise<void>
  onNextRound: () => Promise<void>
  onRecord: (result: MatchResult) => Promise<void>
}) {
  const game = room.douDizhu
  const recorded = useRef(false)
  const [selected, setSelected] = useState<number[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const handRef = useRef<HTMLDivElement>(null)
  const selectingRef = useRef(false)
  const selectionModeRef = useRef<'select' | 'deselect'>('select')
  const visitedCardsRef = useRef(new Set<number>())
  const turnKeyRef = useRef('')
  const [turnSecondsLeft, setTurnSecondsLeft] = useState(game?.turn_seconds_left || 0)
  const me = game?.players.find(player => player.uid === user.uid)
  const others = useMemo(() => game?.players.filter(player => player.uid !== user.uid) || [], [game?.players, user.uid])
  const myTurn = game?.current_uid === user.uid
  const selectedCards = useMemo(() => (me?.hand || []).filter(card => selected.includes(card.id)), [me?.hand, selected])
  const selectedCombo = useMemo(() => classifyDouDizhu(selectedCards), [selectedCards])
  const previousCombo = useMemo(() => game?.last_play ? classifyDouDizhu(game.last_play.cards) : null, [game?.last_play])
  const canPlay = Boolean(myTurn && !busy && game?.phase === 'playing' && selectedCombo && canBeatDouDizhu(selectedCombo, game.last_play?.uid === user.uid ? null : previousCombo))
  const playStatus = getPlayStatus(game?.phase, myTurn, selectedCards.length, selectedCombo, previousCombo, game?.last_play?.uid === user.uid, busy, canPlay)
  const turnKey = `${game?.phase}:${game?.current_uid}:${game?.last_play?.uid || 0}:${game?.last_play?.cards.map(card => card.id).join('.') || ''}`

  useEffect(() => setSelected([]), [game?.current_uid, game?.last_play?.uid, game?.phase])
  useEffect(() => {
    const handIDs = new Set((me?.hand || []).map(card => card.id))
    setSelected(current => {
      const next = current.filter(id => handIDs.has(id))
      return next.length === current.length ? current : next
    })
  }, [me?.hand])
  useEffect(() => {
    if (!game) return
    if (turnKeyRef.current !== turnKey) {
      turnKeyRef.current = turnKey
      setTurnSecondsLeft(game.turn_seconds_left || 0)
    }
    const timer = window.setInterval(() => setTurnSecondsLeft(current => Math.max(0, current - 1)), 1000)
    return () => window.clearInterval(timer)
  }, [game, game?.turn_seconds_left, turnKey])
  useEffect(() => {
    if (!room.roundComplete || !game || !me || recorded.current) return
    recorded.current = true
    const won = game.winner_side === (me.role === 'landlord' ? 'landlord' : 'farmers')
    void onRecord({
      result: won ? 'win' : 'lose', role: me.role || 'farmer', durationSeconds: 60,
      score: Math.max(0, room.roundScore), summary: `${me.role === 'landlord' ? '地主' : '农民'} · ${game.multiplier} 倍`,
    })
  }, [game, me, onRecord, room.roundComplete, room.roundScore])

  if (!game || !me) return <div className="game-state-error">斗地主状态还没有准备好</div>

  async function act(action: GameAction) {
    setBusy(true)
    setError('')
    try {
      await onAction(action)
      return true
    } catch (value) {
      setError((value as Error).message)
      return false
    } finally {
      setBusy(false)
    }
  }

  function toggleCard(id: number) {
    if (!myTurn || game.phase !== 'playing' || busy) return
    setSelected(current => current.includes(id) ? current.filter(item => item !== id) : [...current, id])
  }

  function applyCardSelection(id: number, shouldSelect: boolean) {
    if (!myTurn || game.phase !== 'playing' || busy) return
    setSelected(current => {
      const exists = current.includes(id)
      if (exists === shouldSelect) return current
      return shouldSelect ? [...current, id] : current.filter(item => item !== id)
    })
  }

  function beginCardSelection(id: number, event: PointerEvent<HTMLButtonElement>) {
    if (!myTurn || game.phase !== 'playing' || busy) return
    event.preventDefault()
    selectingRef.current = true
    selectionModeRef.current = selected.includes(id) ? 'deselect' : 'select'
    visitedCardsRef.current = new Set([id])
    applyCardSelection(id, selectionModeRef.current === 'select')
  }

  function continueCardSelection(id: number) {
    if (!selectingRef.current || visitedCardsRef.current.has(id)) return
    visitedCardsRef.current.add(id)
    applyCardSelection(id, selectionModeRef.current === 'select')
  }

  function continueCardSelectionAtPoint(event: PointerEvent<HTMLDivElement>) {
    if (!selectingRef.current) return
    const target = document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>('[data-card-id]')
    if (!target || !handRef.current?.contains(target)) return
    const id = Number(target.dataset.cardId)
    if (Number.isInteger(id)) continueCardSelection(id)
  }

  function endCardSelection() {
    selectingRef.current = false
    visitedCardsRef.current.clear()
  }

  function playSelected() {
    if (!canPlay) return
    void act({ type: 'play_cards', cards: selectedCards.map(card => card.id) }).then(ok => { if (ok) setSelected([]) })
  }

  const winner = game.players.find(player => player.uid === room.winnerUID)

  return <div className="table-game-page dou-dizhu-page">
    <header className="game-session-header">
      <button className="back-button" onClick={onBack}><ArrowLeft size={15} />返回游戏大厅</button>
      <div className="session-title"><span className="session-round">第 {room.currentRound} / {room.rounds} 局 · {game.multiplier} 倍</span><strong>{room.name}</strong></div>
      <div className="table-game-score"><Trophy size={17} /><strong>{me.score}</strong><span>累计分</span></div>
    </header>

    <section className="card-table">
      <div className="opponent-row">
        {others.map(player => <article className={`card-player ${game.current_uid === player.uid ? 'active' : ''}`} key={player.uid}>
          <span className="card-player-avatar">{player.role === 'landlord' ? <Crown size={19} /> : <Shield size={18} />}</span>
          <div><strong>{player.username}</strong><small>{player.role === 'landlord' ? '地主' : player.role === 'farmer' ? '农民' : player.bid ? `叫 ${player.bid} 分` : '等待叫分'}</small></div>
          <b>{player.hand_count} 张</b>
          <div className="card-back-stack" aria-hidden="true"><i /><i /><i /></div>
          {game.phase === 'playing' && game.current_uid === player.uid ? <span className="card-player-timer"><Timer size={12} />{turnSecondsLeft}s</span> : null}
        </article>)}
      </div>

      <div className="table-center">
        <div className="landlord-cards"><span><Layers3 size={14} />地主牌</span><div>{game.bottom_cards.length ? game.bottom_cards.map(card => <Card card={card} key={card.id} />) : <><i /><i /><i /></>}</div></div>
        <div className={`last-play-zone ${game.last_play ? 'has-play' : ''}`}>
          {game.last_play ? <><span><strong>{game.players.find(player => player.uid === game.last_play?.uid)?.username}</strong> · {game.last_play.kind_name}</span><div>{game.last_play.cards.map(card => <Card card={card} key={card.id} />)}</div></> : <><RotateCcw size={24} /><p>{game.phase === 'bidding' ? `当前最高 ${game.highest_bid} 分` : '等待本轮首家出牌'}</p></>}
        </div>
        <div className={`turn-note ${myTurn ? 'your-turn' : ''}`}><span>{myTurn ? '轮到你了' : `等待 ${game.players.find(player => player.uid === game.current_uid)?.username || '玩家'} 操作`}</span>{game.phase === 'playing' ? <b><Timer size={12} />{turnSecondsLeft}s</b> : null}</div>
      </div>

      <div className="my-seat">
        <div className="my-seat-meta"><span className="card-player-avatar">{me.role === 'landlord' ? <Crown size={19} /> : <Shield size={18} />}</span><div><strong>{me.username}（你）</strong><small>{me.role === 'landlord' ? '地主' : me.role === 'farmer' ? '农民' : '等待叫分'}</small></div><b>{me.hand_count} 张</b></div>
        <div className={`my-hand-shell ${myTurn && game.phase === 'playing' ? 'active' : ''}`}>
          <div className="my-hand-label"><span>我的手牌</span>{myTurn && game.phase === 'playing' ? <b><Timer size={13} />{turnSecondsLeft}s</b> : null}</div>
          <div
            ref={handRef}
            className="my-hand"
            aria-label="我的手牌"
            onPointerMove={continueCardSelectionAtPoint}
            onPointerUp={endCardSelection}
            onPointerCancel={endCardSelection}
          >
            {(me.hand || []).map(card => <Card
              card={card}
              key={card.id}
              selected={selected.includes(card.id)}
              onPointerDown={event => beginCardSelection(card.id, event)}
              onPointerEnter={() => continueCardSelection(card.id)}
              onClick={event => { if (event.detail === 0) toggleCard(card.id) }}
            />)}
          </div>
        </div>
        <div className="card-actions">
          {game.phase === 'bidding' ? game.allowed_bids.map(bid => <button className={bid === 3 ? 'button primary' : 'button secondary'} key={bid} disabled={!myTurn || busy} onClick={() => void act({ type: 'bid', bid })}>{bid === 0 ? '不叫' : `叫 ${bid} 分`}</button>) : <>
            <button className="button secondary" disabled={!myTurn || busy || !game.can_pass} onClick={() => void act({ type: 'pass' })}>不出</button>
            <button className={`button ${canPlay ? 'primary' : 'secondary'}`} disabled={!canPlay} onClick={playSelected} title={playStatus}><Play size={15} />出牌</button>
          </>}
        </div>
        {game.phase === 'playing' && myTurn ? <p className={`play-validation ${canPlay ? 'valid' : ''}`} role="status">{playStatus}</p> : null}
        {error ? <p className="table-game-error" role="alert">{error}</p> : null}
      </div>

      <aside className="table-log"><strong>对局动态</strong>{game.logs.slice(-5).map((log, index) => <p key={`${index}-${log.text}`}>{log.uid ? game.players.find(player => player.uid === log.uid)?.username : '系统'}：{log.text}</p>)}</aside>

      {room.roundComplete ? <div className="table-result-overlay"><span><Trophy size={30} /></span><h2>{winner?.username || '本局玩家'}获胜</h2><p>{game.winner_side === 'landlord' ? '地主获胜' : '农民获胜'} · 最终 {game.multiplier} 倍 · 本局 {room.roundScore >= 0 ? '+' : ''}{room.roundScore} 分</p>{room.currentRound < room.rounds && room.hostUID === user.uid ? <button className="button primary" onClick={() => void onNextRound()}>下一局</button> : room.currentRound < room.rounds ? <button className="button secondary" disabled>等待房主开始下一局</button> : <button className="button primary" onClick={onBack}>返回游戏大厅</button>}</div> : null}
    </section>
  </div>
}

function getPlayStatus(
  phase: string | undefined,
  myTurn: boolean,
  selectedCount: number,
  combo: DouDizhuCombo | null,
  previous: DouDizhuCombo | null,
  previousFromMe: boolean,
  busy: boolean,
  canPlay: boolean,
) {
  if (phase !== 'playing') return '当前不是出牌阶段'
  if (!myTurn) return '还没轮到你'
  if (busy) return '正在提交出牌'
  if (!selectedCount) return '请选择要出的牌'
  if (!combo) return '当前选择不是合法牌型'
  if (!canBeatDouDizhu(combo, previousFromMe ? null : previous)) return '当前牌型不能压过上一手'
  return `可以出牌：${comboLabel(combo.kind)}`
}
