const API_BASE_URL = import.meta.env.VITE_API_BASE_URL.replace(/\/$/, '')
const HEALTH_ENDPOINT = '/movieAPI/health'
const LOGIN_ENDPOINT = '/movieAPI/login'
const SIGNUP_ENDPOINT = '/movieAPI/signup'
const MOVIES_ENDPOINT = '/movieAPI/movies'



async function readResponse(response) {
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

export async function getHealth() {
  const response = await fetch(`${API_BASE_URL}${HEALTH_ENDPOINT}`, {
    headers: { Accept: 'application/json, text/plain;q=0.9, */*;q=0.8' },
  })

  return readResponse(response)
}

export async function login({ username, password }) {
  const response = await fetch(`${API_BASE_URL}${LOGIN_ENDPOINT}`, {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ userName: username, password: password }),
  })

  return readResponse(response)
}

export async function signup({ username, password }) {
  const response = await fetch(`${API_BASE_URL}${SIGNUP_ENDPOINT}`, {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ userName: username, password: password }),
  })

  return readResponse(response)
}

export async function getMovies({ limit = 20, offset = 0 } = {}) {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  })
  const response = await fetch(`${API_BASE_URL}${MOVIES_ENDPOINT}?${params}`, {
    headers: { Accept: 'application/json' },
  })
  return readResponse(response)
}
