const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '')

export async function getHealth() {
  const response = await fetch(`${API_BASE_URL}/health`, {
    headers: { Accept: 'application/json, text/plain;q=0.9, */*;q=0.8' },
  })

  const contentType = response.headers.get('content-type') ?? ''
  const body = contentType.includes('application/json')
    ? await response.json()
    : await response.text()

  if (!response.ok) {
    const detail = typeof body === 'string' ? body : JSON.stringify(body)
    throw new Error(`Request failed (${response.status})${detail ? `: ${detail}` : ''}`)
  }

  return body
}
