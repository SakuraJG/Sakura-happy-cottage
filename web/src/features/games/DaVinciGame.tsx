import { useEffect, useRef, useState } from 'react'
import { ArrowLeft, Eye, EyeOff, Fingerprint, Layers3, LockKeyhole, Trophy } from 'lucide-react'
import type { User } from '../../shared/types'
import type { DaVinciTile, GameRoom } from './gameTypes'

type GameAction = { type: string; target_uid?: number; tile_id?: number; number?: number }
type MatchResult = { result: 'win' | 'lose'; role: 'codebreaker'; durationSeconds: number; score: number; summary: string }

function CodeTile({ tile, target, selected, onSelect }: { tile: DaVinciTile; target?: boolean; selected?: boolean; onSelect?: () => void }) {
  return <button type="button" className={`code-tile ${tile.color} ${tile.revealed ? 'revealed' : ''} ${selected ? 'selected' : ''}`} disabled={!onSelect} onClick={onSelect}>
    <span>{tile.number ?? (tile.revealed ? '?' : <LockKeyhole size={17} />)}</span>
    <small>{tile.color === 'black' ? 'BLACK' : 'WHITE'}</small>
    {target && !tile.revealed ? <i><Eye size={12} /></i> : null}
  </button>
}

export function DaVinciGame({ room, user, onBack, onAction, onNextRound, onRecord }: {
  room: GameRoom
  user: User
  onBack: () => void
  onAction: (action: GameAction) => Promise<void>
  onNextRound: () => Promise<void>
  onRecord: (result: MatchResult) => Promise<void>
}) {
  const game = room.daVinci
  const recorded = useRef(false)
  const [target, setTarget] = useState<{ uid: number; tileID: number } | null>(null)
  const [number, setNumber] = useState<number | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const me = game?.players.find(player => player.uid === user.uid)
  const myTurn = game?.current_uid === user.uid

  useEffect(() => { setTarget(null); setNumber(null) }, [game?.current_uid, game?.last_correct])
  useEffect(() => {
    if (!room.roundComplete || !game || !me || recorded.current) return
    recorded.current = true
    void onRecord({
      result: game.winner_uid === user.uid ? 'win' : 'lose', role: 'codebreaker', durationSeconds: 60,
      score: Math.max(0, room.roundScore), summary: `猜中 ${me.correct_count} 张密码牌`,
    })
  }, [game, me, onRecord, room.roundComplete, room.roundScore, user.uid])

  if (!game || !me) return <div className="game-state-error">达芬奇密码状态还没有准备好</div>

  async function act(action: GameAction) {
    setBusy(true)
    setError('')
    try { await onAction(action) } catch (value) { setError((value as Error).message) } finally { setBusy(false) }
  }

  async function submitGuess() {
    if (!target || number === null) return
    await act({ type: 'guess_tile', target_uid: target.uid, tile_id: target.tileID, number })
  }

  const winner = game.players.find(player => player.uid === game.winner_uid)

  return <div className="table-game-page da-vinci-page">
    <header className="game-session-header">
      <button className="back-button" onClick={onBack}><ArrowLeft size={15} />返回游戏大厅</button>
      <div className="session-title"><span className="session-round">第 {room.currentRound} / {room.rounds} 局 · 牌库剩余 {game.deck_count}</span><strong>{room.name}</strong></div>
      <div className="table-game-score"><Fingerprint size={17} /><strong>{me.score}</strong><span>累计分</span></div>
    </header>

    <section className="code-table">
      <div className="code-opponents">
        {game.players.filter(player => player.uid !== user.uid).map(player => <article className={`code-player ${game.current_uid === player.uid ? 'active' : ''} ${player.eliminated ? 'eliminated' : ''}`} key={player.uid}>
          <header><div><span>{player.eliminated ? <EyeOff size={17} /> : <Eye size={17} />}</span><strong>{player.username}</strong></div><small>{player.hidden_count} 张暗牌 · 猜中 {player.correct_count}</small></header>
          <div className="code-rack">{player.tiles.map(tile => <CodeTile tile={tile} target={myTurn} selected={target?.tileID === tile.id} key={tile.id} onSelect={myTurn && !tile.revealed && !room.roundComplete ? () => { setTarget({ uid: player.uid, tileID: tile.id }); setNumber(null) } : undefined} />)}</div>
        </article>)}
      </div>

      <div className="code-control-panel">
        <div className="drawn-code"><span><Layers3 size={15} />本回合摸牌</span>{game.drawn_tile ? <CodeTile tile={game.drawn_tile} /> : <p>牌库已空，直接猜测对手密码</p>}</div>
        <div className="guess-console">
          <div><strong>{myTurn ? target ? '选择你猜测的数字' : '先选择一张对手暗牌' : `等待 ${game.players.find(player => player.uid === game.current_uid)?.username || '玩家'} 猜测`}</strong><small>{target ? '目标牌已标记，提交后由服务器判定' : '数字范围 0–11'}</small></div>
          <div className="number-pad">{Array.from({ length: 12 }, (_, value) => <button className={number === value ? 'active' : ''} type="button" key={value} onClick={() => setNumber(value)} disabled={!target || !myTurn || busy}>{value}</button>)}</div>
          <div className="guess-actions"><button className="button secondary" disabled={!myTurn || !game.can_stop || busy} onClick={() => void act({ type: 'stop_guessing' })}>见好就收</button><button className="button primary" disabled={!myTurn || !target || number === null || busy} onClick={() => void submitGuess()}><Eye size={15} />确认猜测</button></div>
          {error ? <p className="table-game-error" role="alert">{error}</p> : null}
        </div>
      </div>

      <article className={`code-player my-code ${myTurn ? 'active' : ''}`}>
        <header><div><span><Fingerprint size={17} /></span><strong>{me.username}（你）</strong></div><small>{me.hidden_count} 张暗牌 · 已猜中 {me.correct_count}</small></header>
        <div className="code-rack">{me.tiles.map(tile => <CodeTile tile={tile} key={tile.id} />)}</div>
      </article>

      <aside className="table-log"><strong>推理记录</strong>{game.logs.slice(-5).map((log, index) => <p key={`${index}-${log.text}`}>{log.uid ? game.players.find(player => player.uid === log.uid)?.username : '系统'}：{log.text}</p>)}</aside>

      {room.roundComplete ? <div className="table-result-overlay"><span><Trophy size={30} /></span><h2>{winner?.username || '本局玩家'}破解成功</h2><p>你猜中 {me.correct_count} 张密码牌 · 本局 +{room.roundScore} 分</p>{room.currentRound < room.rounds && room.hostUID === user.uid ? <button className="button primary" onClick={() => void onNextRound()}>下一局</button> : room.currentRound < room.rounds ? <button className="button secondary" disabled>等待房主开始下一局</button> : <button className="button primary" onClick={onBack}>返回游戏大厅</button>}</div> : null}
    </section>
  </div>
}
