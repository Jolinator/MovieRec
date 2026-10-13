import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import ConfirmationPopup from './ConfirmationPopup'
import GenreFilter from './GenreFilter'
import LogoutBanner from './LogoutBanner'
import Sidebar from './Sidebar'
import UserBadge from './UserBadge'
import UserSearchBar from './UserSearchBar'
import UserSearchResult from './UserSearchResult'

afterEach(() => {
  vi.useRealTimers()
})

describe('frontend controls', () => {
  it('opens the genre menu and selects a genre', async () => {
    const onSelect = vi.fn()
    const user = userEvent.setup()
    render(<GenreFilter genres={[{ id: 28, name: 'Action' }]} selectedGenreId={null} onSelect={onSelect} />)

    await user.click(screen.getByRole('button', { name: /filter/i }))
    await user.click(screen.getByRole('button', { name: 'Action' }))

    expect(onSelect).toHaveBeenCalledWith(28)
    expect(screen.queryByRole('button', { name: 'Action' })).not.toBeInTheDocument()
  })

  it('shows the selected genre in the filter label', () => {
    render(<GenreFilter genres={[{ id: 28, name: 'Action' }]} selectedGenreId={28} onSelect={vi.fn()} />)

    expect(screen.getByRole('button', { name: /Filter : Action/ })).toBeInTheDocument()
  })

  it('selects all genres', async () => {
    const onSelect = vi.fn()
    const user = userEvent.setup()
    render(<GenreFilter genres={[{ id: 28, name: 'Action' }]} selectedGenreId={28} onSelect={onSelect} />)

    await user.click(screen.getByRole('button', { name: /Filter : Action/ }))
    await user.click(screen.getByRole('button', { name: 'All' }))

    expect(onSelect).toHaveBeenCalledWith(null)
  })

  it('disables an empty user search', () => {
    render(<UserSearchBar value=" " onChange={vi.fn()} onSubmit={vi.fn()} isLoading={false} />)

    expect(screen.getByRole('button', { name: 'Search' })).toBeDisabled()
  })

  it('submits a non-empty user search', async () => {
    const onChange = vi.fn()
    const onSubmit = vi.fn((event) => event.preventDefault())
    const user = userEvent.setup()
    render(<UserSearchBar value="alice" onChange={onChange} onSubmit={onSubmit} isLoading={false} />)

    await user.click(screen.getByRole('button', { name: 'Search' }))

    expect(onSubmit).toHaveBeenCalled()
  })

  it('shows the searching state and disables the search button', () => {
    render(<UserSearchBar value="alice" onChange={vi.fn()} onSubmit={vi.fn()} isLoading />)

    expect(screen.getByRole('button', { name: 'Searching...' })).toBeDisabled()
  })

  it('calls the selected user callback', async () => {
    const user = { username: 'alice' }
    const onClick = vi.fn()
    render(<UserSearchResult user={user} onClick={onClick} />)

    await userEvent.click(screen.getByRole('button', { name: 'alice' }))

    expect(onClick).toHaveBeenCalledWith(user)
  })

  it('renders signed-in and signed-out user badges', () => {
    const { rerender } = render(<UserBadge username="alice" />)
    expect(screen.getByText('alice')).toBeInTheDocument()

    rerender(<UserBadge username="" />)
    expect(screen.getByText('Not signed in')).toBeInTheDocument()
  })

  it('routes sidebar actions to their callbacks', async () => {
    const callbacks = {
      onAuthClick: vi.fn(),
      onHomeClick: vi.fn(),
      onMyListClick: vi.fn(),
      onRecommendationClick: vi.fn(),
      onSearchClick: vi.fn(),
    }
    const user = userEvent.setup()
    render(<Sidebar isLoggedIn={false} {...callbacks} />)

    await user.click(screen.getByRole('button', { name: 'Home' }))
    await user.click(screen.getByRole('button', { name: 'My List' }))
    await user.click(screen.getByRole('button', { name: 'Recommendation Chat' }))
    await user.click(screen.getByRole('button', { name: 'Search Users' }))
    await user.click(screen.getByRole('button', { name: 'Login' }))

    expect(callbacks.onHomeClick).toHaveBeenCalled()
    expect(callbacks.onMyListClick).toHaveBeenCalled()
    expect(callbacks.onRecommendationClick).toHaveBeenCalled()
    expect(callbacks.onSearchClick).toHaveBeenCalled()
    expect(callbacks.onAuthClick).toHaveBeenCalled()
  })

  it('auto-dismisses a confirmation message after three seconds', async () => {
    vi.useFakeTimers()
    const onDismiss = vi.fn()
    render(<ConfirmationPopup message="Added to My List" onDismiss={onDismiss} />)

    expect(screen.getByRole('status')).toHaveTextContent('Added to My List')
    await vi.advanceTimersByTimeAsync(3000)
    expect(onDismiss).toHaveBeenCalled()
  })

  it('confirms or cancels logout', async () => {
    const onCancel = vi.fn()
    const onConfirm = vi.fn()
    const user = userEvent.setup()
    render(<LogoutBanner onCancel={onCancel} onConfirm={onConfirm} />)

    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    await user.click(screen.getByRole('button', { name: 'Log out' }))

    expect(onCancel).toHaveBeenCalled()
    expect(onConfirm).toHaveBeenCalled()
  })
})
