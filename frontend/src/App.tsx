import { useEffect, useState } from 'react'
import LoginScreen from './screens/LoginScreen'
import LobbyScreen from './screens/LobbyScreen'
import TableScreen from './screens/TableScreen'
import Spinner from './components/Spinner'
import ErrorBoundary from './components/ErrorBoundary'
import { useApp } from './store'
import { applyTheme, getStartParam } from './telegram'

export default function App() {
  const { token, booting } = useApp()
  const [tableCode, setTableCode] = useState<string | null>(null)
  const [autoJoin, setAutoJoin] = useState<string | null>(null)

  useEffect(() => {
    applyTheme()
    const param = getStartParam()
    if (param) {
      setAutoJoin(param)
    }
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

  return <LobbyScreen onOpen={setTableCode} autoJoin={autoJoin} />
}
