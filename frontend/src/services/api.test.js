import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  addMovieToList,
  getGenres,
  getMovies,
  getMyList,
  getSession,
  getUserList,
  login,
  logout,
  removeMovieFromList,
  searchUsers,
  signup,
} from './api'

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

  it('uses default pagination when loading movies', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse([]))

    await getMovies()

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost/movieAPI/movies?limit=20&offset=0',
      { headers: { Accept: 'application/json' } },
    )
  })

  it('loads genres as JSON', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse([{ id: 28, name: 'Action' }]))

    await expect(getGenres()).resolves.toEqual([{ id: 28, name: 'Action' }])
  })

  it('gets the current session with cookies enabled', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      jsonResponse({ loggedIn: true, username: 'alice' }),
    )

    await getSession()

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost/movieAPI/session',
      { credentials: 'include', headers: { Accept: 'application/json' } },
    )
  })

  it('logs out with a cookie-enabled POST request', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse({}))

    await logout()

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost/movieAPI/logout',
      {
        method: 'POST',
        credentials: 'include',
        headers: { Accept: 'application/json' },
      },
    )
  })

  it('sends signup credentials as JSON', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      jsonResponse({ success: true }),
    )

    await signup({ username: 'alice', password: 'password123' })

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost/movieAPI/signup',
      {
        method: 'POST',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ userName: 'alice', password: 'password123' }),
      },
    )
  })

  it('adds a movie to the authenticated user list', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse({}))

    await addMovieToList(42)

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost/movieAPI/my-list',
      {
        method: 'POST',
        credentials: 'include',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ tmdbId: 42 }),
      },
    )
  })

  it('removes a movie from the authenticated user list', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse({}))

    await removeMovieFromList(42)

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost/movieAPI/my-list/42',
      {
        method: 'DELETE',
        credentials: 'include',
        headers: { Accept: 'application/json' },
      },
    )
  })

  it('searches users with an encoded query', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse([]))

    await searchUsers('alice smith')

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost/movieAPI/users?search=alice+smith',
      {
        credentials: 'include',
        headers: { Accept: 'application/json' },
      },
    )
  })

  it('loads another user list with an encoded user id', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse([]))

    await getUserList('alice/smith')

    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost/movieAPI/users/alice%2Fsmith/list',
      {
        credentials: 'include',
        headers: { Accept: 'application/json' },
      },
    )
  })
})
