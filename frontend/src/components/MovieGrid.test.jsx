import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import MovieGrid from './MovieGrid'

const movie = {
  tmdbId: 1,
  title: 'Arrival',
  averageRating: 8.0,
  synopsis: 'A linguist studies an alien language.',
  posterPath: null,
}

describe('MovieGrid', () => {
  it('shows an empty state when there are no movies', () => {
    render(<MovieGrid movies={[]} onSelect={vi.fn()} />)

    expect(screen.getByText('No movies available.')).toBeInTheDocument()
  })

  it('renders movies and selects one when clicked', async () => {
    const onSelect = vi.fn()
    render(<MovieGrid movies={[movie]} onSelect={onSelect} />)

    await userEvent.click(screen.getByRole('button', { name: /arrival/i }))

    expect(onSelect).toHaveBeenCalledWith(movie)
  })
})
