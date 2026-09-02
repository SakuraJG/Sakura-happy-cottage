import type { PlayingCard } from './gameTypes'

export type DouDizhuCombo = {
  kind: string
  main: number
  size: number
}

const sortNumbers = (values: number[]) => [...values].sort((a, b) => a - b)

function consecutive(values: number[]) {
  for (let index = 1; index < values.length; index += 1) {
    if (values[index] !== values[index - 1] + 1) return false
  }
  return true
}

export function classifyDouDizhu(cards: PlayingCard[]): DouDizhuCombo | null {
  if (!cards.length) return null
  const counts = new Map<number, number>()
  for (const card of cards) counts.set(card.value, (counts.get(card.value) || 0) + 1)
  const values = sortNumbers([...counts.keys()])
  const size = cards.length

  if (size === 2 && counts.get(16) === 1 && counts.get(17) === 1) return { kind: 'rocket', main: 17, size }
  if (values.length === 1) {
    const kind = ({ 1: 'single', 2: 'pair', 3: 'triple', 4: 'bomb' } as Record<number, string>)[size]
    if (kind) return { kind, main: values[0], size }
  }
  if (size === 4 && values.some(value => counts.get(value) === 3)) return { kind: 'triple_single', main: values.find(value => counts.get(value) === 3) || 0, size }
  if (size === 5) {
    const triple = values.find(value => counts.get(value) === 3)
    const pair = values.find(value => counts.get(value) === 2)
    if (triple !== undefined && pair !== undefined) return { kind: 'triple_pair', main: triple, size }
  }
  if (size >= 5 && values.length === size && consecutive(values) && values[values.length - 1] <= 14) {
    return { kind: 'straight', main: values[values.length - 1], size }
  }
  if (size >= 6 && size % 2 === 0 && values.length === size / 2 && consecutive(values) && values[values.length - 1] <= 14 && values.every(value => counts.get(value) === 2)) {
    return { kind: 'pair_straight', main: values[values.length - 1], size }
  }
  if (size === 6 || size === 8) {
    const bomb = values.find(value => counts.get(value) === 4)
    if (bomb !== undefined) {
      if (size === 6) return { kind: 'four_two_single', main: bomb, size }
      const pairs = values.filter(value => value !== bomb && counts.get(value) === 2)
      if (pairs.length === 2) return { kind: 'four_two_pair', main: bomb, size }
    }
  }

  for (let triples = 2; triples <= 6; triples += 1) {
    for (let start = 3; start + triples - 1 <= 14; start += 1) {
      const remaining = new Map(counts)
      let valid = true
      for (let value = start; value < start + triples; value += 1) {
        if ((remaining.get(value) || 0) < 3) {
          valid = false
          break
        }
        remaining.set(value, (remaining.get(value) || 0) - 3)
      }
      if (!valid) continue
      let left = 0
      let wings = 0
      let pairOnly = true
      for (const count of remaining.values()) {
        left += count
        if (count > 0) {
          if (count !== 2) pairOnly = false
          wings += 1
        }
      }
      const main = start + triples - 1
      if (left === 0 && size === triples * 3) return { kind: 'airplane', main, size }
      if (left === triples && size === triples * 4) return { kind: 'airplane_single', main, size }
      if (left === triples * 2 && pairOnly && wings === triples && size === triples * 5) return { kind: 'airplane_pair', main, size }
    }
  }
  return null
}

export function canBeatDouDizhu(next: DouDizhuCombo, previous: DouDizhuCombo | null) {
  if (!previous) return true
  if (next.kind === 'rocket') return previous.kind !== 'rocket'
  if (previous.kind === 'rocket') return false
  if (next.kind === 'bomb' && previous.kind !== 'bomb') return true
  if (next.kind !== previous.kind || next.size !== previous.size) return false
  return next.main > previous.main
}

export function comboLabel(kind: string) {
  return ({
    single: '单张', pair: '对子', triple: '三张', triple_single: '三带一', triple_pair: '三带二',
    straight: '顺子', pair_straight: '连对', airplane: '飞机', airplane_single: '飞机带单',
    airplane_pair: '飞机带对', four_two_single: '四带二', four_two_pair: '四带两对', bomb: '炸弹', rocket: '王炸',
  } as Record<string, string>)[kind] || '牌型'
}
