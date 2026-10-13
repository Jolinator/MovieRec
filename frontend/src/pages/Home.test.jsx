import React from 'react'
import { act, render, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Home from './Home'
import {
  addMovieToList,
  getGenres,
  getMovies,
  getMyList,
  getPersonalizedRecommendations,
  removeMovieFromList,
} from '../services/api'

vi.mock('../services/api', () => ({
  addMovieToList: vi.fn(),
  getGenres: vi.fn(),
  getMovies: vi.fn(),
  getMyList: vi.fn(),
  getPersonalizedRecommendations: vi.fn(),
  removeMovieFromList: vi.fn(),
}))

vi.mock('../components/GenreFilter', () => ({
  default: () => <div />,
}))

vi.mock('../components/Sidebar', () => ({
  default: () => <div />,
}))

vi.mock('../components/UserBadge', () => ({
  default: () => <div />,
}))

vi.mock('../components/MovieGrid', () => ({
  default: ({ movies }) => <div data-testid="movie-count">{movies.length}</div>,
}))

vi.mock('../components/ErrorPopup', () => ({
  default: () => <div />,
}))

vi.mock('../components/LogoutBanner', () => ({
  default: () => <div />,
}))

vi.mock('../components/MovieDetails', () => ({
  default: () => <div />,
}))

vi.mock('../components/RecommendationChat', () => ({
  default: () => <div />,
}))

let intersectionCallback

beforeEach(() => {
  vi.clearAllMocks()
  getGenres.mockResolvedValue([])
  getMyList.mockResolvedValue([])
  getPersonalizedRecommendations.mockResolvedValue({ movies: [], status: 'empty' })
  getMovies.mockResolvedValue([])
  globalThis.IntersectionObserver = class {
    constructor(callback) {
      intersectionCallback = callback
    }

    observe() {}

    disconnect() {}
  }
})

function movie(id) {
  return {
    tmdbId: id,
    title: `Movie ${id}`,
    averageRating: 8,
    synopsis: 'Synopsis',
  }
}

describe('Home infinite scrolling', () => {
  it('loads another page when the sentinel intersects', async () => {
    const firstPage = Array.from({ length: 99 }, (_, index) => movie(index + 1))
    getMovies
      .mockResolvedValueOnce(firstPage)
      .mockResolvedValueOnce([movie(100)])

    render(
      <Home
        onLoginClick={vi.fn()}
        onRequireLogin={vi.fn()}
        onLogout={vi.fn()}
        onHomeClick={vi.fn()}
        onMyListClick={vi.fn()}
        onSearchClick={vi.fn()}
        username=""
      />,
    )

    await waitFor(() => expect(getMovies).toHaveBeenCalledWith({ limit: 99, genreId: null }))
    await waitFor(() => expect(intersectionCallback).toEqual(expect.any(Function)))

    await act(async () => {
      intersectionCallback([{ isIntersecting: true }])
    })

    await waitFor(() => expect(getMovies).toHaveBeenLastCalledWith({
      limit: 99,
      offset: 99,
      genreId: null,
    }))
  })

  it('does not load another page when the sentinel is not visible', async () => {
    getMovies.mockResolvedValueOnce(Array.from({ length: 99 }, (_, index) => movie(index + 1)))

    render(
      <Home
        onLoginClick={vi.fn()}
        onRequireLogin={vi.fn()}
        onLogout={vi.fn()}
        onHomeClick={vi.fn()}
        onMyListClick={vi.fn()}
        onSearchClick={vi.fn()}
        username=""
      />,
    )

    await waitFor(() => expect(getMovies).toHaveBeenCalledTimes(1))

    await act(async () => {
      intersectionCallback([{ isIntersecting: false }])
    })

    expect(getMovies).toHaveBeenCalledTimes(1)
  })
})
