export interface UserDTO {
  /** The player's Telegram id, as a string so it compares against PlayerDTO.id. */
  id: string
  username: string
  first_name: string
}

export interface LoginResult {
  token: string
  user: UserDTO
}

export interface PlayerDTO {
  id: string
  seat: number
  stack: number
  folded: boolean
  all_in: boolean
  street_bet: number
  has_acted: boolean
  sitting_out: boolean
  hole_cards: string[]
  /** Best five-card hand, present only when the cards are visible to you. */
  hand_name?: string
  hand_cards?: string[]
}

export interface TableDTO {
  code: string
  max_seats: number
  big_blind: number
  players: PlayerDTO[]
  board: string[]
  pot: number
  phase: string
  current_bet: number
  min_raise: number
  acting: number
  button: number
  winners: number[]
  /** The viewer's off-table chip balance. Zero on unauthenticated views. */
  bankroll: number
  /** Seat allowed to deal the next hand; rotates each round. -1 when none can. */
  starter: number
  /** Milliseconds left on the acting player's clock at the time this was sent. */
  turn_ms_left: number
}

export interface WsMessage {
  type: string
  code?: string
  action?: string
  amount?: number
  table?: TableDTO
  error?: string
}
