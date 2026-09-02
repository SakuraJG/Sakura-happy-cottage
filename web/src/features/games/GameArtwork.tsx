import { Brush, Crown, KeyRound, Sparkles } from 'lucide-react'
import type { GameID } from './gameTypes'

export function GameArtwork({ gameID }: { gameID: GameID }) {
  if (gameID === 'drawing-guess') {
    return <div className="game-art game-art-drawing" aria-hidden="true">
      <span className="art-sun" />
      <span className="art-paper"><span className="art-house"><i /><b /></span></span>
      <span className="art-pencil"><Brush size={25} strokeWidth={1.9} /></span>
      <span className="art-spark art-spark-one"><Sparkles size={15} /></span>
      <span className="art-spark art-spark-two"><Sparkles size={11} /></span>
    </div>
  }

  if (gameID === 'dou-dizhu') {
    return <div className="game-art game-art-cards" aria-hidden="true">
      <span className="art-playing-card card-left"><b>Q</b><i>♥</i></span>
      <span className="art-playing-card card-center"><b>A</b><i>♠</i></span>
      <span className="art-playing-card card-right"><b>2</b><i>♦</i></span>
      <span className="art-card-crown"><Crown size={27} /></span>
    </div>
  }

  return <div className="game-art game-art-code" aria-hidden="true">
    <span className="art-code-key"><KeyRound size={31} strokeWidth={1.5} /></span>
    <span className="art-code-tile code-black">?</span>
    <span className="art-code-tile code-white">7</span>
    <span className="art-code-tile code-black-two">3</span>
  </div>
}
