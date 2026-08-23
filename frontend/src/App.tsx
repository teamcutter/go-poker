import { useEffect, useState } from 'react'
import LoginScreen from './screens/LoginScreen'
import LobbyScreen from './screens/LobbyScreen'
import TableScreen from './screens/TableScreen'
import Spinner from './components/Spinner'
import ErrorBoundary from './components/ErrorBoundary'
import { useApp } from './store'
import { applyTheme, markInviteUsed, pendingInvite } from './telegram'

export default function App() {
  const { token, booting } = useApp()
  const [tableCode, setTableCode] = useState<string | null>(null)
  // Read once at mount rather than via an effect: the deep-link start param is
  // fixed for a given launch, so storing it in state costs an extra render.
  // pendingInvite, not getStartParam: a reload must not re-open the invite.
  const [autoJoin, setAutoJoin] = useState<string | null>(() => pendingInvite() || null)

  useEffect(() => {
    applyTheme()
  }, [])

  if (booting) {
    return (
      <main className="center" style={{ justifyContent: 'center' }}>
        <Spinner label="Loading" />
      </main>
    )
  }

  if (!token) {
    return <LoginScreen />
  }

  if (tableCode) {
    return (
      <ErrorBoundary>
        <TableScreen code={tableCode} onLeave={() => setTableCode(null)} />
      </ErrorBoundary>
    )
  }

  // The deep link is good for one journey. Leaving a table unmounts TableScreen
  // and remounts the lobby, which would otherwise act on the link a second time
  // and drag the player straight back into the table they just left. Spending it
  // is written down as well as dropped from state, so a reload of this same
  // launch does not resurrect it.
  return (
    <LobbyScreen
      onOpen={setTableCode}
      autoJoin={autoJoin}
      onAutoJoinSpent={() => {
        if (autoJoin) markInviteUsed(autoJoin)
        setAutoJoin(null)
      }}
    />
  )
}
