export type GameID = 'drawing-guess' | 'dou-dizhu' | 'da-vinci-code'

export type RoomPlayer = {
  uid: number
  username: string
  ready: boolean
  host?: boolean
  bot?: boolean
}

export type GameRoom = {
  code: string
  name: string
  gameID: GameID
  hostUID: number
  maxPlayers: number
  rounds: number
  roundSeconds: number
  passwordProtected: boolean
  recordStats: boolean
  status: 'waiting' | 'playing' | 'round_complete' | 'finished'
  currentRound: number
  drawerUID: number
  secondsLeft: number
  word: string
  wordCategory: string
  wordHint: string
  wordLength: number
  roundComplete: boolean
  winnerUID: number
  roundScore: number
  totalScore: number
  drawingLocked: boolean
  players: RoomPlayer[]
  strokes: GameStroke[]
  messages: GameRoomMessage[]
  douDizhu?: DouDizhuState
  daVinci?: DaVinciState
}

export type PlayingCard = {
  id: number
  rank: string
  value: number
  suit: 'spade' | 'heart' | 'club' | 'diamond' | 'joker'
}

export type DouDizhuPlayer = {
  uid: number
  username: string
  role?: 'landlord' | 'farmer'
  hand_count: number
  hand?: PlayingCard[]
  bid: number
  score: number
}

export type DouDizhuState = {
  phase: 'bidding' | 'playing' | 'round_complete'
  current_uid: number
  turn_seconds_left: number
  landlord_uid?: number
  highest_bid: number
  highest_uid?: number
  bottom_cards: PlayingCard[]
  last_play?: { uid: number; cards: PlayingCard[]; kind: string; kind_name: string }
  multiplier: number
  winner_side?: 'landlord' | 'farmers'
  players: DouDizhuPlayer[]
  logs: { uid: number; text: string }[]
  allowed_bids: number[]
  can_pass: boolean
}

export type DaVinciTile = {
  id: number
  color: 'black' | 'white'
  number?: number
  revealed: boolean
}

export type DaVinciPlayer = {
  uid: number
  username: string
  tiles: DaVinciTile[]
  hidden_count: number
  correct_count: number
  eliminated: boolean
  score: number
}

export type DaVinciState = {
  phase: 'guessing' | 'round_complete'
  current_uid: number
  drawn_tile?: DaVinciTile
  deck_count: number
  can_stop: boolean
  last_correct: boolean
  winner_uid?: number
  players: DaVinciPlayer[]
  logs: { uid: number; text: string }[]
}

export type GamePoint = { x: number; y: number }

export type DrawingTool = 'pen' | 'marker' | 'eraser'

export type GameStroke = { id?: number; color: string; width: number; tool: DrawingTool; opacity: number; points: GamePoint[] }

export type GameRoomMessage = {
  id: number
  author_uid: number
  author: string
  text: string
  kind?: 'system' | 'correct'
  created_at: string
}

export type GameInvitation = {
  code: string
  room_name: string
  from_uid: number
  from_username: string
  password_protected: boolean
  created_at: string
  game_id: GameID
}

export type MatchRecord = {
  id: number
  game_key: GameID
  room_code: string
  room_name: string
  round_number: number
  word: string
  played_at: string
  duration_seconds: number
  result: 'win' | 'complete' | 'lose'
  role: 'drawer' | 'guesser' | 'landlord' | 'farmer' | 'codebreaker'
  score: number
}

export type GameSessionSummary = {
  game_key: GameID
  room_code: string
  room_name: string
  rounds_played: number
  total_seconds: number
  wins: number
  total_score: number
  started_at: string
  ended_at: string
}

export type GameSessionDetail = {
  summary: GameSessionSummary
  rounds: MatchRecord[]
}

export type GameStats = {
  total_games: number
  total_seconds: number
  wins: number
  best_score: number
  recent: GameSessionSummary[]
}

export const EMPTY_GAME_STATS: GameStats = {
  total_games: 0,
  total_seconds: 0,
  wins: 0,
  best_score: 0,
  recent: [],
}

export function formatPlayTime(seconds: number) {
  if (seconds < 60) return `${seconds} 秒`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} 分钟`
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  return rest ? `${hours} 小时 ${rest} 分` : `${hours} 小时`
}
