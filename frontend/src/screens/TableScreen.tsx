import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import * as api from '../api'
import type { PlayerDTO, TableDTO, WsMessage } from '../types'
import Avatar from '../components/Avatar'
import Button from '../components/Button'
import BuyInPicker, { defaultBuyIn } from '../components/BuyInPicker'
import CardView from '../components/CardView'
import Sheet from '../components/Sheet'
import { useApp } from '../store'
import { haptic, hapticNotify } from '../telegram'
import { chips, phaseLabel } from '../utils/chips'

interface TableProps {
  code: string
  onLeave: () => void
}

interface BetSheetState {
  action: 'bet' | 'raise'
  min: number
  max: number
  value: number
}

const BOARD_SLOTS = 5
/** Mirrors the server's TurnLimit; used only to scale the countdown bar. */
const TURN_LIMIT_MS = 30_000
/** Tap-to-add stakes in the bet sheet. */
const BET_STEPS = [10, 25, 50, 100]

export default function TableScreen({ code, onLeave }: TableProps) {
  const { user } = useApp()
  const [table, setTable] = useState<TableDTO | null>(null)
  const [connected, setConnected] = useState(false)
  const [connectionError, setConnectionError] = useState<string | null>(null)
  const [bet, setBet] = useState<BetSheetState | null>(null)
  const [copied, setCopied] = useState(false)
  const [dealing, setDealing] = useState(false)
  const [rebuy, setRebuy] = useState<number | null>(null)
  const [confirmLeave, setConfirmLeave] = useState(false)
  const [turnClock, setTurnClock] = useState<{ receivedAt: number; ms: number } | null>(null)
  // Set optimistically by a top-up and dropped as soon as the server echoes a
  // fresh table state, so the sheet never renders a stale zero bankroll.
  const [walletOverride, setWalletOverride] = useState<number | null>(null)
  const [now, setNow] = useState(() => Date.now())
  const socketRef = useRef<WebSocket | null>(null)

  const me = table?.players.find((p) => p.id === user?.id)
  const isMyTurn =
    table !== null &&
    me !== undefined &&
    table.acting === me.seat &&
    table.phase !== 'waiting' &&
    table.phase !== 'showdown' &&
    table.phase !== 'complete'

  const applyState = useCallback((msg: WsMessage) => {
    if (msg.type === 'table_state' && msg.table) {
      setTable(msg.table)
      setWalletOverride(null)
      // The server sends milliseconds remaining rather than a wall-clock
      // deadline, so the countdown is immune to phone/server clock skew.
      const at = Date.now()
      setTurnClock({ receivedAt: at, ms: msg.table.turn_ms_left })
      // Re-baseline the tick clock too: it only advances while a countdown is
      // running, so on the first hand it can still hold its mount-time value
      // and make the remaining time read high until the next 250ms tick.
      setNow(at)
    }
    if (msg.type === 'error') {
      hapticNotify('error')
      setConnectionError(msg.error ?? 'Action rejected')
    }
  }, [])

  useEffect(() => {
    const token = api.getToken()
    if (!token) {
      setConnectionError('Not signed in')
      return
    }
    const ws = new WebSocket(api.wsTableUrl(code))
    socketRef.current = ws
    ws.onopen = () => {
      setConnected(true)
      setConnectionError(null)
    }
    ws.onmessage = (ev) => {
      try {
        applyState(JSON.parse(ev.data) as WsMessage)
      } catch {
        // ignore malformed frames
      }
    }
    ws.onerror = () => {
      setConnected(false)
      setConnectionError('Connection lost')
    }
    ws.onclose = () => {
      setConnected(false)
    }
    return () => {
      ws.close()
      socketRef.current = null
    }
  }, [code, applyState])

  useEffect(() => {
    if (isMyTurn) haptic('medium')
  }, [isMyTurn])

  // Re-render while a clock is running so the countdown ticks down locally.
  useEffect(() => {
    if (!turnClock || turnClock.ms <= 0) return
    const timer = setInterval(() => setNow(Date.now()), 250)
    return () => clearInterval(timer)
  }, [turnClock])

  const send = useCallback((action: string, amount?: number) => {
    const ws = socketRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    haptic()
    ws.send(JSON.stringify({ type: 'act', action, amount: amount ?? 0 }))
  }, [])

  const startHand = async () => {
    setDealing(true)
    setConnectionError(null)
    try {
      await api.startHand(code)
      haptic('medium')
    } catch (err) {
      hapticNotify('error')
      setConnectionError(apiErrorMessage(err))
    } finally {
      setDealing(false)
    }
  }

  // Leaving must tell the server, otherwise the lobby keeps showing you seated
  // and your stack stays locked at a table you are no longer watching.
  const leaveTable = async () => {
    haptic()
    setConfirmLeave(false)
    try {
      await api.leaveTable(code)
    } catch (err) {
      // Staying put is better than silently leaving the seat occupied: if the
      // server did not release it, the lobby would still show you in the room.
      hapticNotify('error')
      setConnectionError(`Could not leave the table: ${apiErrorMessage(err)}`)
      return
    }
    onLeave()
  }

  const confirmRebuy = async () => {
    if (rebuy === null) return
    try {
      await api.joinTable(code, rebuy)
      haptic('medium')
      setRebuy(null)
    } catch (err) {
      hapticNotify('error')
      setConnectionError(apiErrorMessage(err))
    }
  }

  const claimFreeChips = async () => {
    try {
      const res = await api.topUpWallet()
      setWalletOverride(res.bankroll)
      setRebuy(defaultBuyIn(res.bankroll))
      haptic('medium')
    } catch (err) {
      hapticNotify('error')
      setConnectionError(apiErrorMessage(err))
    }
  }

  const copyCode = async () => {
    try {
      await navigator.clipboard.writeText(code)
      haptic()
      setCopied(true)
      setTimeout(() => setCopied(false), 1600)
    } catch {
      // clipboard unavailable — nothing to do
    }
  }

  // `amount` is the total street bet a player raises *to*, so the ceiling is
  // what is already committed this street plus the remaining stack.
  const toCall = table && me ? Math.max(0, table.current_bet - me.street_bet) : 0
  const minTotal = table
    ? table.current_bet === 0
      ? table.min_raise
      : table.current_bet + table.min_raise
    : 0
  const maxTotal = me ? me.street_bet + me.stack : 0
  const canSize = maxTotal > minTotal

  const openBet = () => {
    if (!table) return
    haptic()
    setBet({ action: toCall <= 0 ? 'bet' : 'raise', min: minTotal, max: maxTotal, value: minTotal })
  }

  // Every sizing control funnels through here so nothing can be sent below the
  // minimum raise or above the stack, whatever combination of taps got there.
  const setBetValue = (value: number) => {
    if (!bet) return
    haptic()
    setBet({ ...bet, value: clamp(Math.round(value), bet.min, bet.max) })
  }

  const halfPot = table
    ? clamp(
        Math.round(table.current_bet + 0.5 * (table.pot + toCall)),
        bet?.min ?? 0,
        bet?.max ?? 0,
      )
    : 0

  const confirmBet = () => {
    if (!bet) return
    if (bet.value >= bet.max) send('allin')
    else send(bet.action, bet.value)
    setBet(null)
  }

  const bankroll = walletOverride ?? table?.bankroll ?? 0
  const revealAll = table?.phase === 'showdown' || table?.phase === 'complete'
  const winners = new Set(table?.winners ?? [])

  // The five cards that actually won, matched by card code so they can be
  // highlighted wherever they appear (board and the winner's hole cards).
  // Null while the hand is live, or when it ended before a flop.
  const winner = table && table.winners.length > 0 ? table.players[table.winners[0]] : undefined
  const winningCards = useMemo(() => {
    if (!revealAll) return null
    const combo = winner?.hand_cards ?? []
    return combo.length > 0 ? new Set(combo) : null
  }, [revealAll, winner])

  const cardState = (code: string) =>
    winningCards ? { highlight: winningCards.has(code), dimmed: !winningCards.has(code) } : {}

  // A finished hand stays in `complete` — the backend never rewinds to
  // `waiting` — so both states are idle and accept a fresh deal.
  const handIdle =
    table !== null &&
    (table.phase === 'waiting' || table.phase === 'showdown' || table.phase === 'complete')
  // The right to deal rotates with the button, so only one player at a time
  // gets the action; everyone else is told who they are waiting on.
  const iAmStarter = table !== null && me !== undefined && table.starter === me.seat
  const canDeal = handIdle && table !== null && table.players.length >= 2 && iAmStarter
  // Chips already committed this hand sit in the pot, not in street_bet, which
  // endStreet zeroes every street. So the amount at risk cannot be quoted as a
  // figure without the server tracking per-hand contribution.
  const inThePot =
    table !== null && me !== undefined && !me.folded && !me.sitting_out && !handIdle

  const starterName =
    table && table.starter >= 0
      ? table.players[table.starter]?.id === user?.id
        ? 'you'
        : shortId(table.players[table.starter]?.id)
      : null

  const msLeft = turnClock ? Math.max(0, turnClock.ms - (now - turnClock.receivedAt)) : 0
  const secsLeft = Math.ceil(msLeft / 1000)
  const clockRunning = msLeft > 0 && table !== null && table.acting >= 0
  const lowClock = clockRunning && secsLeft <= 5

  // Other seats, ordered clockwise starting from the one left of the hero.
  const opponentSeats = useMemo(() => {
    if (!table) return []
    const origin = me ? me.seat : 0
    return Array.from({ length: table.max_seats }, (_, i) => (origin + 1 + i) % table.max_seats).filter(
      (seat) => !me || seat !== me.seat,
    )
  }, [table, me])

  return (
    <main className="table-screen is-fixed">
      <div className="table-topbar">
        <button className="tag tag-code" onClick={copyCode}>
          {copied ? 'COPIED' : code}
        </button>
        <span className={`tag ${connected ? 'tag-live' : 'tag-off'}`}>
          <i className="dot" />
          {connected ? 'LIVE' : 'OFFLINE'}
        </span>
        <div className="table-topbar-meta">
          <span className="num">
            {table
              ? `${chips(Math.max(1, Math.floor(table.big_blind / 2)))}/${chips(table.big_blind)}`
              : '—'}
          </span>
          <button
            className="icon-btn"
            onClick={() => {
              haptic()
              setConfirmLeave(true)
            }}
            aria-label="Leave table"
          >
            ✕
          </button>
        </div>
      </div>

      {connectionError && <div className="error-banner">{connectionError}</div>}

      {/* Opponents, above the board */}
      <div className="opponents">
        {table &&
          opponentSeats.map((seat) => {
            const player = table.players.find((p) => p.seat === seat)
            if (!player) {
              return (
                <div key={seat} className="opponent opponent-empty">
                  <div className="opponent-slot" />
                  <span className="opponent-name">Open</span>
                </div>
              )
            }
            const classes = ['opponent']
            if (table.acting === seat && !revealAll) classes.push('opponent-turn')
            if (player.folded) classes.push('opponent-folded')
            const won = winners.has(table.players.indexOf(player))

            return (
              <div key={seat} className={classes.join(' ')}>
                <div className="opponent-avatar">
                  <Avatar id={player.id} label={shortId(player.id)} size={44} />
                  {table.button === seat && <span className="badge-dealer">D</span>}
                  {won && <span className="badge-win">🏆</span>}
                </div>
                <span className="opponent-name">{shortId(player.id)}</span>
                {player.sitting_out ? (
                  <span className="opponent-stack allin">Sitting out</span>
                ) : player.all_in ? (
                  <span className="opponent-stack allin">All-in</span>
                ) : (
                  <span className="opponent-stack num">{chips(player.stack)}</span>
                )}
                <div className="opponent-cards">
                  {renderOpponentCards(player, revealAll, winningCards)}
                </div>
                {revealAll && player.hand_name && !player.folded && (
                  <span className="opponent-hand">{player.hand_name}</span>
                )}
                {table.acting === seat && clockRunning && (
                  <span className={`turn-bar${lowClock ? ' low' : ''}`}>
                    <i style={{ width: `${Math.min(100, (msLeft / TURN_LIMIT_MS) * 100)}%` }} />
                  </span>
                )}
                {player.street_bet > 0 && (
                  <span className="street-bet">
                    <i className="pot-chip" />
                    <span className="num">{chips(player.street_bet)}</span>
                  </span>
                )}
              </div>
            )
          })}
      </div>

      {/* Board: five community slots */}
      <div className="board-stage">
        {table && table.pot > 0 && (
          // Keyed on the amount so a growing pot remounts and replays the pop.
          <span key={table.pot} className="pot pot-pop">
            <i className="pot-chip" />
            <span className="pot-label">Pot</span>
            <span className="num">{chips(table.pot)}</span>
          </span>
        )}

        <div className="board">
          {Array.from({ length: BOARD_SLOTS }).map((_, i) => {
            const card = table?.board[i]
            return card ? (
              <CardView
                key={`board-${i}`}
                code={card}
                size="md"
                animate
                delay={table?.board.length === 3 ? i * 90 : 0}
                {...cardState(card)}
              />
            ) : (
              <div key={`slot-${i}`} className="board-slot" />
            )
          })}
        </div>

        {table && table.winners.length > 0 ? (
          <div className="winner-banner">
            {table.winners.length > 1 ? 'Split pot' : 'Winner'} ·{' '}
            {table.winners
              .map((i) => (table.players[i]?.id === user?.id ? 'You' : shortId(table.players[i]?.id)))
              .join(', ')}
            {/* Preflop the "combination" is just two loose cards, so naming it
                in the result banner would overstate a hand won by folding. */}
            {winner?.hand_name && table.board.length >= 3 ? ` · ${winner.hand_name}` : ''}
          </div>
        ) : (
          <span className="phase-label">{table ? phaseLabel(table.phase) : 'Connecting…'}</span>
        )}
      </div>

      {/* Hero: cards bottom-left, stack + actions bottom-right */}
      <div className="hero-row">
        <div className="hero-hand">
          <div className="hero-cards">
            {me && me.hole_cards.filter(Boolean).length > 0 ? (
              me.hole_cards.map((c, i) => (
                <CardView
                  key={`me-${c}`}
                  code={c}
                  size="lg"
                  animate
                  delay={i * 110}
                  dimmed={me.folded && !winningCards}
                  {...cardState(c)}
                />
              ))
            ) : (
              <>
                <CardView code="" hidden size="lg" />
                <CardView code="" hidden size="lg" />
              </>
            )}
          </div>
          {me?.hand_name && <span className="hand-label">{me.hand_name}</span>}
        </div>

        <div className="hero-side">
          <div className="hero-stack-row">
            <div>
              <span className="eyebrow">Your stack</span>
              <div className="hero-stack num">{chips(me?.stack ?? 0)}</div>
            </div>
            {isMyTurn && (
              <div className={`hero-tocall${toCall > 0 ? ' owed' : ''}`}>
                {toCall > 0 ? (
                  <>
                    <span className="tocall-label">To call</span>
                    <b className="tocall-amount num">{chips(Math.min(toCall, me?.stack ?? 0))}</b>
                  </>
                ) : (
                  <span className="tocall-label">Your turn</span>
                )}
                {clockRunning && (
                  <span className={`tocall-clock num${lowClock ? ' low' : ''}`}>{secsLeft}s</span>
                )}
              </div>
            )}
          </div>

          {isMyTurn ? (
            <div className="action-bar">
              <Button variant="secondary" onClick={() => send('fold')}>
                Fold
              </Button>
              {toCall <= 0 ? (
                <Button variant="secondary" onClick={() => send('check')}>
                  Check
                </Button>
              ) : (
                <Button variant="secondary" onClick={() => send('call')}>
                  Call
                  <small className="num">{chips(Math.min(toCall, me?.stack ?? 0))}</small>
                </Button>
              )}
              {canSize ? (
                <Button variant="gold" onClick={openBet}>
                  {toCall <= 0 ? 'Bet' : 'Raise'}
                </Button>
              ) : (
                <Button variant="gold" onClick={() => send('allin')}>
                  All-in
                  <small className="num">{chips(me?.stack ?? 0)}</small>
                </Button>
              )}
            </div>
          ) : me && me.stack <= 0 && (handIdle || me.sitting_out) ? (
            <Button
              block
              variant="gold"
              onClick={() => {
                haptic()
                setRebuy(defaultBuyIn(bankroll))
              }}
            >
              Re-buy to keep playing
            </Button>
          ) : canDeal ? (
            <Button block onClick={startHand} disabled={dealing}>
              {dealing ? 'Dealing…' : table?.phase === 'waiting' ? 'Deal first hand' : 'Deal next hand'}
            </Button>
          ) : (
            <div className="waiting-strip">
              {!table
                ? 'Joining table…'
                : handIdle
                  ? table.players.length < 2
                    ? `Waiting for players — code ${code}`
                    : starterName
                      ? `Waiting for ${starterName} to deal`
                      : 'Waiting for a funded player to deal'
                  : me?.folded
                    ? 'You folded this hand'
                    : 'Waiting for other players…'}
            </div>
          )}
        </div>
      </div>

      {confirmLeave && (
        <Sheet title="Leave the table?" onClose={() => setConfirmLeave(false)}>
          <p className="faint" style={{ fontSize: 13.5, margin: '0 0 18px', lineHeight: 1.5 }}>
            {inThePot ? (
              <>
                Everything you have already put in this pot stays in it — the hand finishes
                without you. Only your remaining{' '}
                <b className="num text-gold">{chips(me?.stack ?? 0)}</b> goes back to your
                bankroll, so you forfeit the rest of your buy-in.
              </>
            ) : (
              <>
                Your <b className="num text-gold">{chips(me?.stack ?? 0)}</b> goes back to your
                bankroll and your seat opens up for someone else.
              </>
            )}
          </p>
          <Button size="lg" block variant="danger" onClick={leaveTable}>
            {inThePot ? 'Leave and forfeit the pot' : 'Leave table'}
          </Button>
          <Button
            size="lg"
            block
            variant="ghost"
            className="mt-8"
            onClick={() => setConfirmLeave(false)}
          >
            Stay
          </Button>
        </Sheet>
      )}

      {rebuy !== null && (
        <Sheet title="Re-buy" onClose={() => setRebuy(null)}>
          {bankroll <= 0 ? (
            <>
              <p className="faint" style={{ fontSize: 13.5, textAlign: 'center', margin: '0 0 18px' }}>
                Your bankroll is empty. Free chips are available every 10 minutes.
              </p>
              <Button size="lg" block variant="gold" onClick={claimFreeChips}>
                Claim free chips
              </Button>
            </>
          ) : (
            <>
              <BuyInPicker
                bankroll={bankroll}
                value={rebuy}
                onChange={(amount) => setRebuy(amount)}
              />
              <Button size="lg" block variant="gold" onClick={confirmRebuy} disabled={rebuy <= 0}>
                Add {chips(rebuy)}
              </Button>
            </>
          )}
        </Sheet>
      )}

      {bet && (
        <Sheet title={bet.action === 'bet' ? 'Bet' : 'Raise to'} onClose={() => setBet(null)}>
          <div className="bet-amount num">{chips(bet.value)}</div>
          <div className="bet-amount-sub">
            {bet.value >= bet.max ? 'All-in' : `min ${chips(bet.min)} · max ${chips(bet.max)}`}
          </div>

          <div className="bet-steps">
            {BET_STEPS.map((step) => (
              <Button
                key={step}
                size="sm"
                variant="secondary"
                disabled={bet.value >= bet.max}
                onClick={() => setBetValue(bet.value + step)}
              >
                +{step}
              </Button>
            ))}
          </div>

          <div className="bet-steps">
            <Button
              size="sm"
              variant={bet.value === halfPot ? 'gold' : 'secondary'}
              onClick={() => setBetValue(halfPot)}
            >
              ½ pot
            </Button>
            <Button
              size="sm"
              variant={bet.value >= bet.max ? 'gold' : 'secondary'}
              onClick={() => setBetValue(bet.max)}
            >
              All in
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={bet.value === bet.min}
              onClick={() => setBetValue(bet.min)}
            >
              Min
            </Button>
          </div>

          <Button size="lg" block variant="gold" className="mt-12" onClick={confirmBet}>
            {bet.value >= bet.max
              ? 'All-in'
              : `${bet.action === 'bet' ? 'Bet' : 'Raise to'} ${chips(bet.value)}`}
          </Button>
        </Sheet>
      )}
    </main>
  )
}

function renderOpponentCards(
  player: PlayerDTO,
  revealAll: boolean,
  winningCards: Set<string> | null,
) {
  // The server masks hole cards as empty strings until the hand is over.
  const faces = player.hole_cards.filter(Boolean)
  if (revealAll && faces.length > 0) {
    return faces.map((c, i) => (
      <CardView
        key={`o-${i}`}
        code={c}
        size="xs"
        reveal
        delay={i * 110}
        highlight={winningCards?.has(c) ?? false}
        dimmed={winningCards ? !winningCards.has(c) : player.folded}
      />
    ))
  }
  if (player.sitting_out) {
    return <span className="eyebrow">Out</span>
  }
  if (player.folded) {
    return <span className="eyebrow">Folded</span>
  }
  return [0, 1].map((i) => <CardView key={`o-${i}`} code="" hidden size="xs" />)
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}

function shortId(id?: string): string {
  if (!id) return '—'
  return id.slice(0, 4).toUpperCase()
}

function apiErrorMessage(err: unknown): string {
  if (err instanceof api.ApiError) return err.message
  return err instanceof Error ? err.message : 'Something went wrong'
}
