import { useCallback, useEffect, useState } from 'react'
import * as api from '../api'
import type { TableDTO } from '../types'
import Avatar from '../components/Avatar'
import Button from '../components/Button'
import BuyInPicker, { defaultBuyIn } from '../components/BuyInPicker'
import Sheet from '../components/Sheet'
import Spinner from '../components/Spinner'
import { useApp } from '../store'
import { getTelegramName, haptic, hapticNotify } from '../telegram'
import { chips, countdown, shortId } from '../utils/chips'

interface LobbyProps {
  onOpen: (code: string) => void
  autoJoin?: string | null
}

const SEAT_MIN = 2
const SEAT_MAX = 6
const BLIND_OPTIONS = [5, 10, 25, 50, 100]
/** Assumed bankroll before the wallet has been read; the server caps the real buy-in. */
const ASSUMED_BANKROLL = 1000
/** Table-list refresh. Kept well under the server's 30 req/min budget so that
 *  idling in the lobby still leaves room for joining, leaving and dealing. */
const POLL_MS = 5000

type SheetKind = 'create' | 'code' | 'buyin' | 'profile' | null

export default function LobbyScreen({ onOpen, autoJoin }: LobbyProps) {
  const { user } = useApp()
  const [tables, setTables] = useState<TableDTO[]>([])
  // null means "not read yet" — distinct from a known-empty bankroll, so a
  // failed wallet fetch never masquerades as being broke and locks the lobby.
  const [bankroll, setBankroll] = useState<number | null>(null)
  // Captured with the moment it was read, so the countdown ticks locally rather
  // than needing the server polled every second.
  const [bonus, setBonus] = useState<{ readAt: number; readyInMs: number; amount: number } | null>(null)
  const [now, setNow] = useState(() => Date.now())
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [sheet, setSheet] = useState<SheetKind>(null)
  const [joinCode, setJoinCode] = useState('')
  const [pendingCode, setPendingCode] = useState<string | null>(null)
  const [seats, setSeats] = useState(6)
  const [blind, setBlind] = useState(10)
  const [buyIn, setBuyIn] = useState(0)

  const displayName = getTelegramName() ?? user?.first_name ?? user?.username ?? 'You'
  // Only a confirmed zero blocks play; an unread bankroll must not.
  const broke = bankroll !== null && bankroll <= 0
  const spendable = bankroll ?? ASSUMED_BANKROLL

  const load = useCallback(async () => {
    try {
      setTables(await api.listTables())
      setError(null)
    } catch (err) {
      setError(apiErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }, [])

  // The bankroll only moves on join/leave/top-up, and each of those returns the
  // new balance, so it is read once per mount instead of on every poll. The
  // server allows 30 requests/min per user across all routes — polling two
  // endpoints every 3s blew through that while simply sitting in the lobby.
  useEffect(() => {
    void load()
    api
      .getWallet()
      .then((w) => {
        setBankroll(w.bankroll)
        setBonus({ readAt: Date.now(), readyInMs: w.bonus_ready_in_ms, amount: w.bonus_amount })
      })
      .catch(() => setBankroll(null))
    const timer = setInterval(() => {
      void load()
    }, POLL_MS)
    return () => clearInterval(timer)
  }, [load])

  const sitDown = useCallback(
    async (code: string, amount: number) => {
      setBusy(true)
      setError(null)
      try {
        await api.joinTable(code, amount)
        haptic('medium')
        setSheet(null)
        onOpen(code)
      } catch (err) {
        hapticNotify('error')
        setError(apiErrorMessage(err))
      } finally {
        setBusy(false)
      }
    },
    [onOpen],
  )

  useEffect(() => {
    if (autoJoin && !loading) {
      void sitDown(autoJoin, defaultBuyIn(spendable))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoJoin, loading])

  const openBuyIn = (code: string) => {
    haptic()
    setPendingCode(code)
    setBuyIn(defaultBuyIn(spendable))
    setSheet('buyin')
  }

  const createAndSit = async () => {
    setBusy(true)
    setError(null)
    try {
      const res = await api.createTable(seats, blind)
      await api.joinTable(res.table.code, buyIn)
      haptic('medium')
      setSheet(null)
      onOpen(res.table.code)
    } catch (err) {
      hapticNotify('error')
      setError(apiErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const claimFreeChips = async () => {
    setBusy(true)
    setError(null)
    try {
      const res = await api.topUpWallet()
      setBankroll(res.bankroll)
      setBonus({ readAt: Date.now(), readyInMs: res.bonus_ready_in_ms, amount: res.bonus_amount })
      haptic('medium')
    } catch (err) {
      hapticNotify('error')
      setError(apiErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const bonusIn = bonus ? Math.max(0, bonus.readyInMs - (now - bonus.readAt)) : 0
  const bonusReady = bonus !== null && bonusIn <= 0

  useEffect(() => {
    if (bonusIn <= 0) return
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [bonusIn])

  const liveCount = tables.reduce((sum, t) => sum + t.players.length, 0)
  // A seat you already hold needs no buy-in — your chips are already on that
  // table. Without this the sheet would demand a buy-in you cannot afford,
  // because the very chips it wants are the ones sitting in that seat.
  const mySeat = (t: TableDTO) => t.players.find((p) => p.id === user?.id)
  // Holding chips means no buy-in is needed; merely holding a seat is enough to
  // keep the table reachable, even after busting out of it.
  const mySeatIn = (t: TableDTO) => t.players.find((p) => p.id === user?.id && p.stack > 0)
  const seatedTable = tables.find((t) => mySeatIn(t) !== undefined)

  const openTable = (t: TableDTO) => {
    if (mySeatIn(t)) {
      void sitDown(t.code, 0)
      return
    }
    openBuyIn(t.code)
  }

  return (
    <main>
      <div className="lobby-header">
        <div>
          <h1 className="lobby-title">Tables</h1>
          <div className="lobby-sub">
            {loading ? 'Looking for games…' : `${tables.length} open · ${liveCount} playing`}
          </div>
        </div>
        <button
          className="avatar-btn"
          onClick={() => {
            haptic()
            setSheet('profile')
          }}
          aria-label="Your profile"
        >
          <Avatar id={user?.id ?? displayName} label={displayName} size={38} />
        </button>
      </div>

      <div className="space-between" style={{ marginBottom: 14 }}>
        <span className="bankroll-pill">
          <i className="pot-chip" />
          <span className="num">{bankroll === null ? '…' : chips(bankroll)}</span>
        </span>
        <span className="eyebrow">Your bankroll</span>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {seatedTable ? (
        <div className="broke-banner">
          <p>
            <b>You are still in a hand</b>
            {chips(mySeatIn(seatedTable)?.stack ?? 0)} of yours is at table {seatedTable.code}.
          </p>
          <Button
            size="sm"
            variant="gold"
            onClick={() => sitDown(seatedTable.code, 0)}
            disabled={busy}
          >
            Rejoin
          </Button>
        </div>
      ) : (
        bonus && (
          <div className="broke-banner">
            <p>
              <b>{broke ? 'You are out of chips' : `Free ${chips(bonus.amount)} chips`}</b>
              {bonusReady
                ? `Claim ${chips(bonus.amount)} now, then again every 10 minutes.`
                : `Next claim in ${countdown(bonusIn)}.`}
            </p>
            <Button
              size="sm"
              variant="gold"
              onClick={claimFreeChips}
              disabled={busy || !bonusReady}
            >
              {bonusReady ? 'Claim' : countdown(bonusIn)}
            </Button>
          </div>
        )
      )}

      <div className="row" style={{ gap: 9 }}>
        <Button
          block
          size="lg"
          onClick={() => {
            haptic()
            setBuyIn(defaultBuyIn(spendable))
            setSheet('create')
          }}
          disabled={busy}
        >
          New table
        </Button>
        <Button
          variant="secondary"
          size="lg"
          onClick={() => {
            haptic()
            setSheet('code')
          }}
          disabled={busy}
        >
          Code
        </Button>
      </div>

      <div className="section-title">Open tables</div>

      {loading && <Spinner label="Loading tables" />}

      {!loading && tables.length === 0 && (
        <div className="empty-state">
          No tables running right now.
          <br />
          Start one and share the code with friends.
        </div>
      )}

      {tables.map((t) => {
        const mine = mySeat(t)
        // A full table is only a dead end for someone without a seat at it —
        // your own seat is always reachable, full or not.
        const full = t.players.length >= t.max_seats && mine === undefined
        return (
        <button
          key={t.code}
          className="table-item"
          disabled={busy || full}
          onClick={() => openTable(t)}
        >
          <div className="table-item-badge">
            <b className="num">{t.code}</b>
            <span>TABLE</span>
          </div>
          <div className="table-item-main">
            <div className="table-item-name">
              {t.max_seats}-max · {chips(Math.max(1, Math.floor(t.big_blind / 2)))}/
              {chips(t.big_blind)}
              {mine && <span className="seat-tag">Your seat</span>}
            </div>
            <div className="table-item-meta">
              <span className="seat-pips">
                {Array.from({ length: t.max_seats }).map((_, i) => (
                  <span key={i} className={`seat-pip${i < t.players.length ? ' on' : ''}`} />
                ))}
              </span>
              <span className={`seat-count num${full ? ' full' : ''}`}>
                {t.players.length}/{t.max_seats}
              </span>
              <span>·</span>
              <span>{t.phase === 'waiting' ? 'Open' : 'In hand'}</span>
            </div>
          </div>
          <span className="chev">›</span>
        </button>
        )
      })}

      {sheet === 'profile' && (
        <Sheet title="Profile" onClose={() => setSheet(null)}>
          <div className="profile-head">
            <Avatar id={user?.id ?? displayName} label={displayName} size={58} />
            <div>
              <div className="profile-name">{displayName}</div>
              {user?.username && <div className="profile-handle">@{user.username}</div>}
            </div>
          </div>

          <div className="profile-rows">
            {/* The four-character tag is all opponents ever see of you, so it is
                worth showing here — it is how someone at a table refers to you. */}
            <div className="profile-row">
              <span>Player tag</span>
              <b className="num">{shortId(user?.id)}</b>
            </div>
            <div className="profile-row">
              <span>Bankroll</span>
              <b className="num">{bankroll === null ? '…' : chips(bankroll)}</b>
            </div>
            {seatedTable && (
              <div className="profile-row">
                <span>In play at {seatedTable.code}</span>
                <b className="num">{chips(mySeatIn(seatedTable)?.stack ?? 0)}</b>
              </div>
            )}
            {bonus && (
              <div className="profile-row">
                <span>Free chips</span>
                <b className="num">
                  {bonusReady ? `${chips(bonus.amount)} ready` : `in ${countdown(bonusIn)}`}
                </b>
              </div>
            )}
          </div>
        </Sheet>
      )}

      {sheet === 'create' && (
        <Sheet title="New table" onClose={() => setSheet(null)}>
          <div className="field" style={{ marginBottom: 16 }}>
            <span className="field-label">Players</span>
            <div className="stepper">
              <button onClick={() => setSeats((s) => Math.max(SEAT_MIN, s - 1))} disabled={seats <= SEAT_MIN}>
                −
              </button>
              <span className="stepper-value num">{seats}</span>
              <button onClick={() => setSeats((s) => Math.min(SEAT_MAX, s + 1))} disabled={seats >= SEAT_MAX}>
                +
              </button>
            </div>
          </div>

          <div className="field" style={{ marginBottom: 16 }}>
            <span className="field-label">Big blind</span>
            <div className="segmented">
              {BLIND_OPTIONS.map((n) => (
                <button
                  key={n}
                  className={n === blind ? 'on' : ''}
                  onClick={() => {
                    haptic()
                    setBlind(n)
                  }}
                >
                  {n}
                </button>
              ))}
            </div>
          </div>

          <BuyInPicker bankroll={spendable} value={buyIn} onChange={setBuyIn} />

          <Button size="lg" block onClick={createAndSit} disabled={busy || buyIn <= 0}>
            {busy ? 'Creating…' : `Create & sit with ${chips(buyIn)}`}
          </Button>
        </Sheet>
      )}

      {sheet === 'code' && (
        <Sheet title="Join with code" onClose={() => setSheet(null)}>
          <input
            className="input input-code num"
            placeholder="CODE"
            value={joinCode}
            maxLength={8}
            autoFocus
            autoCapitalize="characters"
            autoCorrect="off"
            spellCheck={false}
            onChange={(e) => setJoinCode(e.target.value.toUpperCase())}
          />
          <p className="faint" style={{ fontSize: 12.5, textAlign: 'center', margin: '10px 0 18px' }}>
            Ask the host for their table code.
          </p>
          <Button
            size="lg"
            block
            onClick={() => openBuyIn(joinCode.trim().toUpperCase())}
            disabled={busy || !joinCode.trim()}
          >
            Continue
          </Button>
        </Sheet>
      )}

      {sheet === 'buyin' && pendingCode && (
        <Sheet title={`Sit at ${pendingCode}`} onClose={() => setSheet(null)}>
          <BuyInPicker bankroll={spendable} value={buyIn} onChange={setBuyIn} />
          <Button
            size="lg"
            block
            onClick={() => sitDown(pendingCode, buyIn)}
            disabled={busy || buyIn <= 0}
          >
            {busy ? 'Sitting down…' : `Sit with ${chips(buyIn)}`}
          </Button>
        </Sheet>
      )}
    </main>
  )
}

function apiErrorMessage(err: unknown): string {
  if (err instanceof api.ApiError) return err.message
  return err instanceof Error ? err.message : 'Something went wrong'
}
