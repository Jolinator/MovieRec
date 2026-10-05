import { useCallback, useEffect, useState } from 'react'
import { getHealth } from '../services/api'

function formatResponse(response) {
  return typeof response === 'string' ? response : JSON.stringify(response, null, 2)
}

export default function Home() {
  const [state, setState] = useState({ status: 'loading', response: '', error: '' })

  const checkHealth = useCallback(async () => {
    setState({ status: 'loading', response: '', error: '' })

    try {
      const response = await getHealth()
      setState({ status: 'success', response: formatResponse(response), error: '' })
    } catch (error) {
      setState({
        status: 'error',
        response: '',
        error: error instanceof Error ? error.message : 'Unable to reach the backend.',
      })
    }
  }, [])

  useEffect(() => {
    checkHealth()
  }, [checkHealth])

  const label = state.status === 'loading' ? 'Checking backend…' : state.status === 'success' ? 'Backend responded' : 'Request failed'

  return (
    <main className="health-check">
      <section className="health-card" aria-live="polite">
        <p className="eyebrow">Backend connection</p>
        <h1>Health check</h1>
        <div className={`status status--${state.status}`}>
          <span className="status__dot" aria-hidden="true" />
          {label}
        </div>

        {state.status === 'success' && <pre className="response">{state.response}</pre>}
        {state.status === 'error' && <p className="error-message">{state.error}</p>}

        <button type="button" onClick={checkHealth} disabled={state.status === 'loading'}>
          {state.status === 'loading' ? 'Checking…' : 'Check again'}
        </button>
      </section>
    </main>
  )
}
