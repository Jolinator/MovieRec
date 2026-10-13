import { afterEach, describe, expect, it, vi } from 'vitest'
import { getMovies, login } from './api'

afterEach(() => {
  vi.restoreAllMocks()
})

function jsonResponse(body, options = {}) {
  return new Response(JSON.stringify(body), {
    headers: { 'content-type': 'application/json' },
    ...options,
  })
}

describe('API service', () => {
  it('requests movies with pagination and genre filters', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse([]))

    await getMovies({ limit: 10, offset: 20, genreId: 28 })

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost/movieAPI/movies?limit=10&offset=20&genre=28',
      { headers: { Accept: 'application/json' } },
    )
  })

  it('sends login credentials as JSON with cookies enabled', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      jsonResponse({ loggedIn: true }),
    )

    await login({ username: 'alice', password: 'password123' })

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost/movieAPI/login',
      {
        method: 'POST',
        credentials: 'include',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ userName: 'alice', password: 'password123' }),
      },
    )
  })

  it('includes API error details in the thrown error', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      jsonResponse({ error: 'invalid credentials' }, { status: 401 }),
    )

    await expect(login({ username: 'alice', password: 'wrongpass' }))
      .rejects.toThrow('Request failed (401): {"error":"invalid credentials"}')
  })
})
