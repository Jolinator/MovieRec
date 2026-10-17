const API_BASE_URL = import.meta.env.VITE_API_BASE_URL.replace(/\/$/, '')
const HEALTH_ENDPOINT = '/movieAPI/health'
const LOGIN_ENDPOINT = '/movieAPI/login'
const SESSION_ENDPOINT = '/movieAPI/session'
const LOGOUT_ENDPOINT = '/movieAPI/logout'
const SIGNUP_ENDPOINT = '/movieAPI/signup'
const MOVIES_ENDPOINT = '/movieAPI/movies'
const GENRES_ENDPOINT = '/movieAPI/genres'
const MY_LIST_ENDPOINT = '/movieAPI/my-list'
const USERS_ENDPOINT = '/movieAPI/users'
const RECOMMENDATIONS_ENDPOINT = '/movieAPI/recommendations/chat'
const PERSONALIZED_RECOMMENDATIONS_ENDPOINT = '/movieAPI/recommendations'

let unauthorizedHandler = null

export function setUnauthorizedHandler(handler) {
  unauthorizedHandler = handler
  return () => {
    if (unauthorizedHandler === handler) {
      unauthorizedHandler = null
    }
  }
}

async function readResponse(response, { handleUnauthorized = false } = {}) {
  const contentType = response.headers.get('content-type') ?? ''
  const body = contentType.includes('application/json')
    ? await response.json()
    : await response.text()

  if (!response.ok) {
    if (response.status === 401 && handleUnauthorized) {
      unauthorizedHandler?.()
    }
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
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ userName: username, password: password }),
  })

  return readResponse(response)
}

export async function getSession() {
  const response = await fetch(`${API_BASE_URL}${SESSION_ENDPOINT}`, {
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })
  return readResponse(response)
}

export async function logout() {
  const response = await fetch(`${API_BASE_URL}${LOGOUT_ENDPOINT}`, {
    method: 'POST',
    credentials: 'include',
    headers: { Accept: 'application/json' },
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

export async function getMovies({ limit = 20, offset = 0, genreId } = {}) {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  })
  if (genreId !== null && genreId !== undefined) {
    params.set('genre', String(genreId))
  }
  const response = await fetch(`${API_BASE_URL}${MOVIES_ENDPOINT}?${params}`, {
    headers: { Accept: 'application/json' },
  })
  return readResponse(response)
}

export async function getGenres() {
  const response = await fetch(`${API_BASE_URL}${GENRES_ENDPOINT}`, {
    headers: { Accept: 'application/json' },
  })
  return readResponse(response)
}

export async function getMyList() {
  const response = await fetch(`${API_BASE_URL}${MY_LIST_ENDPOINT}`, {
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })
  return readResponse(response, { handleUnauthorized: true })
}

export async function addMovieToList(tmdbId) {
  const response = await fetch(`${API_BASE_URL}${MY_LIST_ENDPOINT}`, {
    method: 'POST',
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ tmdbId }),
  })
  return readResponse(response, { handleUnauthorized: true })
}

export async function removeMovieFromList(tmdbId) {
  const response = await fetch(`${API_BASE_URL}${MY_LIST_ENDPOINT}/${tmdbId}`, {
    method: 'DELETE',
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })
  return readResponse(response, { handleUnauthorized: true })
}

export async function searchUsers(query) {
  const params = new URLSearchParams({ search: query })
  const response = await fetch(`${API_BASE_URL}${USERS_ENDPOINT}?${params}`, {
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })
  return readResponse(response, { handleUnauthorized: true })
}

export async function getUserList(userId) {
  const response = await fetch(`${API_BASE_URL}${USERS_ENDPOINT}/${encodeURIComponent(userId)}/list`, {
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })
  return readResponse(response, { handleUnauthorized: true })
}

export async function chatRecommendations(message) {
  const response = await fetch(`${API_BASE_URL}${RECOMMENDATIONS_ENDPOINT}`, {
    method: 'POST', credentials: 'include',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify({ message }),
  })
  return readResponse(response, { handleUnauthorized: true })
}

export async function getPersonalizedRecommendations() {
  const response = await fetch(`${API_BASE_URL}${PERSONALIZED_RECOMMENDATIONS_ENDPOINT}`, {
    credentials: 'include',
    cache: 'no-store',
    headers: { Accept: 'application/json' },
  })
  return readResponse(response, { handleUnauthorized: true })
}
