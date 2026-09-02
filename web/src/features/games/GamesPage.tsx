import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type FormEvent } from 'react'
import { createPortal } from 'react-dom'
import { ArrowLeft, Bot, Check, ChevronRight, Clock3, Copy, DoorOpen, Eye, Gamepad2, History, LockKeyhole, LogOut, Plus, Radio, ShieldCheck, Sparkles, Trophy, UserPlus, UsersRound, X } from 'lucide-react'
import { initial } from '../../shared/format'
import { request, type ApiError } from '../../shared/api'
import type { SocialUser, User } from '../../shared/types'
import { DaVinciGame } from './DaVinciGame'
import { DrawingGuessGame } from './DrawingGuessGame'
import { DouDizhuGame } from './DouDizhuGame'
import { GameArtwork } from './GameArtwork'
import { CreateRoomDialog, JoinRoomDialog, type RoomDraft } from './RoomDialogs'
import { EMPTY_GAME_STATS, formatPlayTime, type GameID, type GameInvitation, type GameRoom, type GameSessionDetail, type GameStats, type GameStroke, type RoomPlayer } from './gameTypes'
import { DRAWING_WORDS } from './wordBank'

type GamesView = 'lobby' | 'room' | 'playing'
type LobbyTab = 'discover' | 'records'

const GAME_CATALOG: { id: GameID; title: string; description: string; players: string; duration: string; available: boolean }[] = [
  { id: 'drawing-guess', title: '你画我猜', description: '轮流画出秘密词语，让朋友在倒计时结束前猜出答案。', players: '2–8 人', duration: '约 10 分钟', available: true },
  { id: 'dou-dizhu', title: '斗地主', description: '三人叫分争夺地主，用完整牌型规则打完手中的牌。', players: '3 人', duration: '约 8 分钟', available: true },
  { id: 'da-vinci-code', title: '达芬奇密码', description: '观察黑白密码牌的顺序，逐步破解对手隐藏的数字。', players: '2–4 人', duration: '约 12 分钟', available: true },
]

const GAME_NAMES: Record<GameID, string> = { 'drawing-guess': '你画我猜', 'dou-dizhu': '斗地主', 'da-vinci-code': '达芬奇密码' }

export function GamesPage({ user, onToast }: { user: User; onToast: (message: string) => void }) {
  const [view, setView] = useState<GamesView>('lobby')
  const [tab, setTab] = useState<LobbyTab>('discover')
  const [dialog, setDialog] = useState<'create' | 'join' | null>(null)
  const [creatingGame, setCreatingGame] = useState<GameID>('drawing-guess')
  const [statsGame, setStatsGame] = useState<GameID>('drawing-guess')
  const [room, setRoom] = useState<GameRoom | null>(null)
  const [friends, setFriends] = useState<SocialUser[]>([])
  const [stats, setStats] = useState<GameStats>(EMPTY_GAME_STATS)
  const [statsLoading, setStatsLoading] = useState(true)
  const [startingIn, setStartingIn] = useState<number | null>(null)
  const [invitations, setInvitations] = useState<GameInvitation[]>([])
  const [sessionDetail, setSessionDetail] = useState<GameSessionDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState('')
  const exitNotices = useRef(new Set<string>())

  const loadStats = useCallback(async () => {
    setStatsLoading(true)
    try {
      const payload = await request<{ stats: GameStats }>(`/api/games/stats?game=${encodeURIComponent(statsGame)}`)
      setStats({ ...EMPTY_GAME_STATS, ...payload.stats, recent: payload.stats.recent || [] })
    } catch (error) {
      onToast((error as Error).message)
    } finally {
      setStatsLoading(false)
    }
  }, [onToast, statsGame])

  useEffect(() => { void loadStats() }, [loadStats])

  const setRoomFromPayload = useCallback((payload: { room: GameRoom }) => {
    const nextRoom = { ...payload.room, strokes: payload.room.strokes || [], messages: payload.room.messages || [] }
    for (const message of nextRoom.messages) {
      if (!message.text.includes('已退出房间')) continue
      const key = `${nextRoom.code}-${nextRoom.currentRound}-${message.id}-${message.text}`
      if (exitNotices.current.has(key)) continue
      exitNotices.current.add(key)
      onToast(message.text)
    }
    setRoom(nextRoom)
    return nextRoom
  }, [onToast])

  const refreshRoom = useCallback(async (code: string) => {
    const payload = await request<{ room: GameRoom }>(`/api/game-rooms/${encodeURIComponent(code)}`)
    return setRoomFromPayload(payload)
  }, [setRoomFromPayload])

  useEffect(() => {
    if (view !== 'lobby' || room) return
    let active = true
    request<{ room: GameRoom }>('/api/game-rooms/current')
      .then(payload => {
        if (!active) return
        setRoomFromPayload(payload)
      })
      .catch(() => undefined)
    return () => { active = false }
  }, [room, setRoomFromPayload, view])

  useEffect(() => {
    if (view !== 'lobby') return
    let active = true
    const loadInvitations = () => request<{ invitations: GameInvitation[] }>('/api/game-invitations')
      .then(payload => { if (active) setInvitations(payload.invitations || []) })
      .catch(() => undefined)
    void loadInvitations()
    const timer = window.setInterval(loadInvitations, 5000)
    return () => { active = false; window.clearInterval(timer) }
  }, [view])

  useEffect(() => {
    window.scrollTo({ top: 0, behavior: 'auto' })
  }, [view])

  useEffect(() => {
    if (view !== 'room') return
    request<{ users: SocialUser[] }>('/api/social/friends')
      .then(payload => setFriends(payload.users || []))
      .catch(() => setFriends([]))
  }, [view])

  useEffect(() => {
    if (!room?.code) return
    let active = true
    const sync = () => refreshRoom(room.code).catch((value: ApiError) => {
      if (value.status === 403 || value.status === 404) setRoom(null)
    })
    const source = new EventSource(`/api/game-rooms/${encodeURIComponent(room.code)}/events`)
    source.addEventListener('room', () => { if (active) void sync() })
    source.onerror = () => undefined
    const fallback = window.setInterval(sync, 5000)
    return () => {
      active = false
      source.close()
      window.clearInterval(fallback)
    }
  }, [refreshRoom, room?.code])

  useEffect(() => {
    if (view === 'room' && room?.status === 'playing' && startingIn === null) setStartingIn(3)
  }, [room?.status, startingIn, view])

  useEffect(() => {
    if (startingIn === null) return
    if (startingIn === 0) {
      setStartingIn(null)
      setView('playing')
      return
    }
    const timer = window.setTimeout(() => setStartingIn(current => current === null ? null : current - 1), 850)
    return () => window.clearTimeout(timer)
  }, [startingIn])

  const currentPlayer = room?.players.find(player => player.uid === user.uid)
  const minimumPlayers = room?.gameID === 'dou-dizhu' ? 3 : 2
  const everyoneReady = Boolean(room && room.players.length >= minimumPlayers && room.players.every(player => player.ready))

  const availableFriends = useMemo(() => {
    const playerUIDs = new Set(room?.players.map(player => player.uid) || [])
    return friends.filter(friend => !playerUIDs.has(friend.uid))
  }, [friends, room?.players])

  async function createRoom(draft: RoomDraft) {
    const payload = await request<{ room: GameRoom }>('/api/game-rooms', {
      method: 'POST',
      body: JSON.stringify({ game_id: draft.gameID, name: draft.name, max_players: draft.maxPlayers, rounds: draft.rounds, round_seconds: draft.roundSeconds, password: draft.password, record_stats: draft.recordStats }),
    })
    setRoomFromPayload(payload)
    setDialog(null)
    setView('room')
    onToast('房间已创建')
  }

  async function joinRoom(code: string, password: string) {
    const payload = await request<{ room: GameRoom }>('/api/game-rooms/join', {
      method: 'POST',
      body: JSON.stringify({ code, password }),
    })
    setRoomFromPayload(payload)
    setDialog(null)
    setView('room')
    onToast('已加入好友房间')
  }

  async function copyInvite() {
    if (!room) return
    try {
      await navigator.clipboard.writeText(room.code)
      onToast('房间邀请码已复制')
    } catch {
      onToast(`房间邀请码：${room.code}`)
    }
  }

  async function inviteFriend(friend: SocialUser) {
    if (!room) return
    await request(`/api/game-rooms/${encodeURIComponent(room.code)}/invite`, {
      method: 'POST',
      body: JSON.stringify({ uid: friend.uid }),
    })
    onToast(`已邀请 ${friend.username}`)
  }

  async function inviteUID(uid: number) {
    if (!room || !Number.isInteger(uid) || uid <= 0) throw new Error('请输入有效的用户 UID')
    await request(`/api/game-rooms/${encodeURIComponent(room.code)}/invite`, {
      method: 'POST',
      body: JSON.stringify({ uid }),
    })
    onToast(`已向 UID ${uid} 发送邀请`)
  }

  async function addPracticePlayer() {
    if (!room) return
    const payload = await request<{ room: GameRoom }>(`/api/game-rooms/${encodeURIComponent(room.code)}/practice`, { method: 'POST' })
    setRoomFromPayload(payload)
    onToast('练习玩家正在加入')
  }

  async function toggleReady() {
    if (!room) return
    const nextReady = !currentPlayer?.ready
    const payload = await request<{ room: GameRoom }>(`/api/game-rooms/${encodeURIComponent(room.code)}/ready`, {
      method: 'POST', body: JSON.stringify({ ready: nextReady }),
    })
    setRoomFromPayload(payload)
  }

  async function startGame() {
    if (!room) return
    const payload = await request<{ room: GameRoom }>(`/api/game-rooms/${encodeURIComponent(room.code)}/start`, { method: 'POST' })
    setRoomFromPayload(payload)
    setStartingIn(3)
  }

  async function leaveRoom() {
    const currentRoom = room
    setStartingIn(null)
    setRoom(null)
    setView('lobby')
    if (currentRoom) {
      try { await request(`/api/game-rooms/${encodeURIComponent(currentRoom.code)}`, { method: 'DELETE' }) } catch { /* room may already be gone */ }
    }
  }

  function goLobby() {
    setStartingIn(null)
    setView('lobby')
  }

  function enterCurrentRoom() {
    if (!room) return
    setView(room.status === 'playing' || room.status === 'round_complete' ? 'playing' : 'room')
  }

  async function recordMatch(result: { result: 'win' | 'complete'; role: 'drawer' | 'guesser'; durationSeconds: number; score: number }) {
	return recordGameMatch('drawing-guess', { ...result, summary: room?.word || '' })
  }

  async function recordGameMatch(gameID: GameID, result: { result: 'win' | 'complete' | 'lose'; role: 'drawer' | 'guesser' | 'landlord' | 'farmer' | 'codebreaker'; durationSeconds: number; score: number; summary: string }) {
    if (!room?.recordStats) return
    try {
      const payload = await request<{ stats: GameStats }>('/api/games/matches', {
        method: 'POST',
        body: JSON.stringify({
          game_key: gameID,
          room_code: room?.code || '',
          room_name: room?.name || '',
          role: result.role,
          result: result.result,
          round_number: room?.currentRound || 1,
          word: result.summary,
          duration_seconds: result.durationSeconds,
          score: result.score,
        }),
      })
      if (statsGame === gameID) setStats({ ...EMPTY_GAME_STATS, ...payload.stats, recent: payload.stats.recent || [] })
      onToast('本局战绩已保存')
    } catch (error) {
      onToast(`战绩保存失败：${(error as Error).message}`)
    }
  }

  async function openSessionDetail(code: string) {
    setDetailLoading(true)
    setDetailError('')
    setSessionDetail(null)
    try {
      const payload = await request<{ session: GameSessionDetail }>(`/api/games/sessions/${encodeURIComponent(code)}?game=${encodeURIComponent(statsGame)}`)
      setSessionDetail(payload.session)
    } catch (value) {
      setDetailError((value as Error).message)
    } finally {
      setDetailLoading(false)
    }
  }

  async function roomAction(path: string, options: RequestInit = {}) {
    if (!room) return
    const payload = await request<{ room: GameRoom }>(`/api/game-rooms/${encodeURIComponent(room.code)}${path}`, options)
    setRoomFromPayload(payload)
  }

  const addStroke = (stroke: Omit<GameStroke, 'id'>) => roomAction('/strokes', { method: 'POST', body: JSON.stringify(stroke) })
  const undoStroke = () => roomAction('/strokes/undo', { method: 'POST' })
  const clearStrokes = () => roomAction('/strokes', { method: 'DELETE' })
  const sendRoomMessage = (text: string) => roomAction('/messages', { method: 'POST', body: JSON.stringify({ text }) })
  const lockDrawing = () => roomAction('/drawing/finish', { method: 'POST' })
  const finishRound = () => roomAction('/round/finish', { method: 'POST' })
  const nextRound = () => roomAction('/round/next', { method: 'POST' })
  const gameAction = (action: object) => roomAction('/actions', { method: 'POST', body: JSON.stringify(action) })

  if (view === 'playing' && room) {
    if (room.gameID === 'dou-dizhu') return <DouDizhuGame key={`${room.code}-${room.currentRound}`} room={room} user={user} onBack={goLobby} onAction={gameAction} onNextRound={nextRound} onRecord={result => recordGameMatch('dou-dizhu', result)} />
    if (room.gameID === 'da-vinci-code') return <DaVinciGame key={`${room.code}-${room.currentRound}`} room={room} user={user} onBack={goLobby} onAction={gameAction} onNextRound={nextRound} onRecord={result => recordGameMatch('da-vinci-code', result)} />
    return <DrawingGuessGame key={`${room.code}-${room.currentRound}`} room={room} user={user} onBack={goLobby} onRecord={recordMatch} onStroke={addStroke} onUndo={undoStroke} onClear={clearStrokes} onMessage={sendRoomMessage} onLock={lockDrawing} onFinish={finishRound} onNextRound={nextRound} />
  }

  if (view === 'room' && room) {
    return <RoomView
      room={room}
      user={user}
      currentPlayer={currentPlayer}
      availableFriends={availableFriends}
      everyoneReady={everyoneReady}
      startingIn={startingIn}
      onBack={goLobby}
      onLeave={leaveRoom}
      onCopy={copyInvite}
      onInvite={inviteFriend}
      onInviteUID={inviteUID}
      onAddPractice={addPracticePlayer}
      onReady={toggleReady}
      onStart={startGame}
    />
  }

  const winRate = stats.total_games ? Math.round((stats.wins / stats.total_games) * 100) : 0

  return <div className="games-page">
    <section className="games-heading">
      <div><p className="overline">一起玩</p><h1>小游戏</h1><p>叫上好友，在小屋里开一局轻松的游戏。</p></div>
      <div className="games-heading-actions">
        <button className="button secondary" onClick={() => setDialog('join')} disabled={Boolean(room)}><DoorOpen size={16} />加入房间</button>
        <button className="button primary" onClick={() => { setCreatingGame('drawing-guess'); setDialog('create') }} disabled={Boolean(room)}><Plus size={16} />创建房间</button>
      </div>
    </section>

    {room ? <section className="current-room-strip" aria-label="我的当前房间">
      <div className="current-room-icon"><Radio size={18} /></div>
      <div className="current-room-copy"><span>我的房间 · {room.status === 'waiting' ? '等待中' : room.status === 'playing' ? '游戏进行中' : '回合结算中'}</span><strong>{room.name}</strong><small>{room.code} · {room.players.length}/{room.maxPlayers} 人 · {room.recordStats ? '记录战绩' : '娱乐模式'}</small></div>
      <div className="current-room-actions"><button className="button secondary compact" onClick={() => void leaveRoom()}><LogOut size={14} />离开房间</button><button className="button primary compact" onClick={enterCurrentRoom}>返回我的房间<ChevronRight size={15} /></button></div>
    </section> : null}

    {invitations.length ? <section className="game-invitation-strip" aria-label="房间邀请">
      <div><Sparkles size={17} /><span><strong>{invitations[0].from_username}</strong> 邀请你加入「{invitations[0].room_name}」{invitations[0].password_protected ? '（私密房间）' : ''}</span></div>
      <button className="button primary compact" onClick={() => void joinRoom(invitations[0].code, '')} disabled={Boolean(room)}><DoorOpen size={15} />{room ? '已有房间' : '接受邀请'}</button>
    </section> : null}

    <div className="game-tabs" role="tablist" aria-label="小游戏视图">
      <button role="tab" aria-selected={tab === 'discover'} className={tab === 'discover' ? 'active' : ''} onClick={() => setTab('discover')}><Gamepad2 size={16} />游戏大厅</button>
      <button role="tab" aria-selected={tab === 'records'} className={tab === 'records' ? 'active' : ''} onClick={() => setTab('records')}><History size={16} />我的战绩{stats.total_games ? <b>{stats.total_games}</b> : null}</button>
    </div>

    {tab === 'discover' ? <section className="game-catalog" aria-label="小游戏列表">
      {GAME_CATALOG.map((game, index) => <article className={`game-card ${game.available ? 'available' : 'coming'}`} key={game.id} style={{ '--game-index': index } as CSSProperties}>
        <GameArtwork gameID={game.id} />
        <div className="game-card-content">
          <div className="game-card-title"><h2>{game.title}</h2>{game.available ? <span className="live-mark"><i />可游玩</span> : <span>开发中</span>}</div>
          <p>{game.description}</p>
          <div className="game-card-meta"><span><UsersRound size={14} />{game.players}</span><span><Clock3 size={14} />{game.duration}</span>{game.id === 'drawing-guess' ? <span><Sparkles size={14} />{DRAWING_WORDS.length} 个词语</span> : null}</div>
          {game.available ? <button className="game-entry" onClick={() => { setCreatingGame(game.id); setDialog('create') }} disabled={Boolean(room)}>{room ? '已有进行中的房间' : '创建游戏房间'}<ChevronRight size={17} /></button> : <button className="game-entry" disabled>敬请期待</button>}
        </div>
      </article>)}
    </section> : <section className="game-records">
      <div className="record-summary">
        <div><span><Gamepad2 size={17} />已玩局数</span><strong>{statsLoading ? '—' : stats.total_games}</strong></div>
        <div><span><Clock3 size={17} />累计时长</span><strong>{statsLoading ? '—' : formatPlayTime(stats.total_seconds)}</strong></div>
        <div><span><Trophy size={17} />胜率</span><strong>{statsLoading ? '—' : `${winRate}%`}</strong></div>
        <div><span><Sparkles size={17} />最高得分</span><strong>{statsLoading ? '—' : stats.best_score}</strong></div>
      </div>
      <div className="recent-matches">
        <header><div><h2>最近对局</h2><p>你的{GAME_NAMES[statsGame]}游玩记录</p></div><div className="record-game-filter">{GAME_CATALOG.map(game => <button className={statsGame === game.id ? 'active' : ''} key={game.id} onClick={() => setStatsGame(game.id)}>{game.title}</button>)}</div></header>
        {statsLoading ? <div className="record-loading"><span /><span /><span /></div> : stats.recent.length ? <div className="match-list">{stats.recent.map(session => <button className="match-row" key={session.room_code} onClick={() => void openSessionDetail(session.room_code)}>
          <span className={`match-result ${session.wins ? 'win' : ''}`}><Trophy size={16} /></span>
          <div><strong>{session.room_name || `${GAME_NAMES[statsGame]}房间`}</strong><small>{session.rounds_played} 局 · {new Intl.DateTimeFormat('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(new Date(session.ended_at))} · {session.room_code}</small></div>
          <span>{formatPlayTime(session.total_seconds)}</span><b>{session.total_score} 分</b><Eye size={16} />
        </button>)}</div> : <div className="records-empty"><Gamepad2 size={30} strokeWidth={1.3} /><h3>还没有游玩记录</h3><p>完成第一局{GAME_NAMES[statsGame]}后，战绩会保存在这里。</p><button className="button primary" onClick={() => { setCreatingGame(statsGame); setDialog('create') }} disabled={Boolean(room)}><Plus size={15} />创建第一局</button></div>}
      </div>
    </section>}

    {dialog === 'create' ? <CreateRoomDialog username={user.username} initialGameID={creatingGame} onClose={() => setDialog(null)} onCreate={createRoom} /> : null}
    {dialog === 'join' ? <JoinRoomDialog onClose={() => setDialog(null)} onJoin={joinRoom} /> : null}
    {detailLoading || detailError || sessionDetail ? <MatchDetailDialog detail={sessionDetail} loading={detailLoading} error={detailError} onClose={() => { setSessionDetail(null); setDetailError(''); setDetailLoading(false) }} /> : null}
  </div>
}

function RoomView({ room, user, currentPlayer, availableFriends, everyoneReady, startingIn, onBack, onLeave, onCopy, onInvite, onInviteUID, onAddPractice, onReady, onStart }: {
  room: GameRoom
  user: User
  currentPlayer?: RoomPlayer
  availableFriends: SocialUser[]
  everyoneReady: boolean
  startingIn: number | null
  onBack: () => void
  onLeave: () => Promise<void>
  onCopy: () => void
  onInvite: (friend: SocialUser) => Promise<void>
  onInviteUID: (uid: number) => Promise<void>
  onAddPractice: () => Promise<void>
  onReady: () => Promise<void>
  onStart: () => Promise<void>
}) {
  const isHost = room.hostUID === user.uid
  const [targetUID, setTargetUID] = useState('')
  const [inviteError, setInviteError] = useState('')
  const [busyAction, setBusyAction] = useState('')
	const gameName = GAME_NAMES[room.gameID]

  async function submitUID(event: FormEvent) {
    event.preventDefault()
    setInviteError('')
    setBusyAction('invite')
    try {
      await onInviteUID(Number(targetUID))
      setTargetUID('')
    } catch (value) {
      setInviteError((value as Error).message)
    } finally {
      setBusyAction('')
    }
  }

  async function runAction(name: string, action: () => Promise<void>) {
    setBusyAction(name)
    try { await action() } catch (value) { setInviteError((value as Error).message) } finally { setBusyAction('') }
  }
  return <div className="game-room-page">
    <button className="back-button" onClick={onBack}><ArrowLeft size={15} />返回游戏大厅</button>
    <section className="room-heading">
      <div><p className="overline">{gameName}房间</p><h1>{room.name}</h1><p>{room.rounds} {room.gameID === 'drawing-guess' ? '回合' : '局'} · 操作时间 {room.roundSeconds} 秒 · 最多 {room.maxPlayers} 人 · {room.recordStats ? '记录战绩' : '娱乐模式'}</p></div>
      <div className="invite-code-block"><span>房间邀请码{room.passwordProtected ? <LockKeyhole size={12} /> : null}</span><strong>{room.code}</strong><button className="icon-button" onClick={onCopy} aria-label="复制邀请码"><Copy size={17} /></button></div>
    </section>

    <div className="room-layout">
      <section className="room-players">
        <header><div><h2>房间玩家</h2><span>{room.players.length} / {room.maxPlayers}</span></div><UsersRound size={19} /></header>
        <div className="player-list">{room.players.map(player => <div className="room-player" key={player.uid}>
          <span className={`avatar ${player.bot ? 'bot-avatar' : ''}`}>{player.bot ? <Bot size={18} /> : initial(player.username)}</span>
          <div><strong>{player.username}{player.uid === user.uid ? '（你）' : ''}</strong><small>{player.host ? '房主' : player.bot ? '练习玩家' : `UID ${player.uid}`}</small></div>
          <span className={`ready-status ${player.ready ? 'ready' : ''}`}>{player.ready ? <><Check size={14} />已准备</> : '未准备'}</span>
        </div>)}</div>
        {Array.from({ length: Math.min(3, Math.max(0, room.maxPlayers - room.players.length)) }, (_, index) => <div className="empty-seat" key={index}><span><UserPlus size={16} /></span><p>等待玩家加入</p></div>)}
        {room.messages.some(message => message.text.includes('已退出房间')) ? <div className="room-notice-list">{room.messages.filter(message => message.text.includes('已退出房间')).slice(-3).map(message => <p key={message.id}><LogOut size={13} />{message.text}</p>)}</div> : null}
      </section>

      <aside className="room-invite-panel">
        <header><div><h2>邀请好友</h2><p>选择好友加入当前房间</p></div><UserPlus size={19} /></header>
        <div className="friend-invite-list">
          {availableFriends.slice(0, 4).map(friend => <div className="friend-invite-row" key={friend.uid}><span className="avatar">{initial(friend.username)}</span><div><strong>{friend.username}</strong><small>UID {friend.uid}</small></div><button className="button secondary compact" onClick={() => void runAction(`friend-${friend.uid}`, () => onInvite(friend))} disabled={busyAction !== ''}><UserPlus size={14} />邀请</button></div>)}
          {availableFriends.length === 0 ? <div className="no-friends"><UsersRound size={25} /><span>暂无可邀请好友</span></div> : null}
        </div>
        <form className="uid-invite-form" onSubmit={submitUID}><label htmlFor="invite-uid">指定用户 UID</label><div><input id="invite-uid" className="invite-uid-input" inputMode="numeric" value={targetUID} onChange={event => setTargetUID(event.target.value.replace(/\D/g, ''))} placeholder="例如 1024" /><button className="button secondary compact" type="submit" disabled={!targetUID || busyAction !== ''}><UserPlus size={14} />发送邀请</button></div></form>
        {inviteError ? <p className="game-form-error" role="alert">{inviteError}</p> : null}
        {room.players.length < room.maxPlayers ? <button className="practice-player-button" onClick={() => void runAction('practice', onAddPractice)} disabled={busyAction !== ''}><Bot size={17} /><span><strong>加入练习玩家</strong><small>自动补位并完成游戏操作</small></span><ChevronRight size={16} /></button> : null}
      </aside>
    </div>

    <footer className="room-actions">
      <div><span className={`room-status-dot ${everyoneReady ? 'ready' : ''}`} /><p><strong>{everyoneReady ? '全员准备完毕' : '等待所有玩家准备'}</strong><small>{isHost ? '全员准备后即可开始游戏' : '准备后等待房主开始游戏'}</small></p></div>
      <div><button className="button secondary leave-room-button" onClick={() => void runAction('leave', onLeave)} disabled={busyAction !== ''}><LogOut size={15} />离开房间</button><button className={`button ${currentPlayer?.ready ? 'secondary' : 'primary'}`} onClick={() => void runAction('ready', onReady)} disabled={startingIn !== null || busyAction !== ''}>{currentPlayer?.ready ? '取消准备' : '我准备好了'}</button>{isHost ? <button className="button primary start-game-button" onClick={() => void runAction('start', onStart)} disabled={!everyoneReady || startingIn !== null || busyAction !== ''}><Gamepad2 size={16} />{startingIn === null ? '开始游戏' : `${startingIn} 秒后开始`}</button> : null}</div>
    </footer>
    {startingIn !== null ? <div className="start-countdown" role="status" aria-live="assertive">
      <div className="start-countdown-panel">
        <div className="start-countdown-kicker"><Sparkles size={14} /><span>准备就绪</span><i>ROOM {room.code}</i></div>
        <div className={`start-countdown-number ${startingIn === 0 ? 'go' : ''}`} key={startingIn}>{startingIn > 0 ? startingIn : '开局'}</div>
        <div className="start-countdown-copy"><strong>{startingIn > 0 ? '游戏即将开始' : '开始游戏'}</strong><span>{startingIn > 0 ? '请准备好你的第一步' : '祝你手气不错'}</span></div>
        <div className="start-countdown-progress" aria-hidden="true"><i style={{ width: `${((3 - Math.max(startingIn, 0)) / 3) * 100}%` }} /></div>
      </div>
    </div> : null}
  </div>
}

function MatchDetailDialog({ detail, loading, error, onClose }: { detail: GameSessionDetail | null; loading: boolean; error: string; onClose: () => void }) {
  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose() }
    window.addEventListener('keydown', closeOnEscape)
    return () => window.removeEventListener('keydown', closeOnEscape)
  }, [onClose])

  return createPortal(<div className="game-dialog-backdrop" role="presentation" onMouseDown={event => event.target === event.currentTarget && onClose()}>
    <section className="game-dialog match-detail-dialog" role="dialog" aria-modal="true" aria-labelledby="match-detail-title">
      <header><div><h2 id="match-detail-title">对局详情</h2><p>{detail ? `${detail.summary.room_name} · 房间 ${detail.summary.room_code}` : '正在读取这场对局的回合记录'}</p></div><button className="icon-button" onClick={onClose} aria-label="关闭对局详情"><X size={18} /></button></header>
      {loading ? <div className="record-loading detail-loading"><span /><span /><span /></div> : error ? <div className="detail-error"><ShieldCheck size={24} /><p>{error}</p></div> : detail ? <>
        <div className="match-detail-summary"><div><span>局数</span><strong>{detail.summary.rounds_played}</strong></div><div><span>累计时长</span><strong>{formatPlayTime(detail.summary.total_seconds)}</strong></div><div><span>胜局</span><strong>{detail.summary.wins}</strong></div><div><span>总分</span><strong>{detail.summary.total_score}</strong></div></div>
        <div className="round-detail-list">{detail.rounds.map(round => <article key={round.id}><span className={`match-result ${round.result === 'win' ? 'win' : ''}`}><Trophy size={15} /></span><div><strong>第 {round.round_number} 局 · {round.word || '对局完成'}</strong><small>{matchRoleLabel(round.role)} · {round.result === 'win' ? '获胜' : round.result === 'lose' ? '未获胜' : '完成'}</small></div><span>{formatPlayTime(round.duration_seconds)}</span><b>{round.score} 分</b></article>)}</div>
      </> : null}
    </section>
  </div>, document.body)
}

function matchRoleLabel(role: GameSessionDetail['rounds'][number]['role']) {
  return { drawer: '画手', guesser: '猜词玩家', landlord: '地主', farmer: '农民', codebreaker: '破译者' }[role]
}
