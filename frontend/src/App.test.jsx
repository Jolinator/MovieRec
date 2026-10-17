import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import { getSession } from './services/api'

vi.mock('./services/api', () => ({
  getSession: vi.fn(),
  logout: vi.fn(),
  setUnauthorizedHandler: vi.fn(() => vi.fn()),
}))

vi.mock('./pages/Home', () => ({
  default: ({ onMyListClick, onSearchClick, onLoginClick, username }) => (
    <main>
      <h1>Home</h1>
      <p>{username || 'Guest'}</p>
      <button onClick={onMyListClick}>My List</button>
      <button onClick={onSearchClick}>Search Users</button>
      <button onClick={onLoginClick}>Login</button>
    </main>
  ),
}))

vi.mock('./pages/Login', () => ({
  default: ({ onBack, onLoginSuccess }) => (
    <main>
      <h1>Login</h1>
      <button onClick={onBack}>Back</button>
      <button onClick={() => onLoginSuccess('alice')}>Log in as Alice</button>
    </main>
  ),
}))

vi.mock('./pages/MyList', () => ({
  default: () => <h1>My List</h1>,
}))

vi.mock('./pages/UserSearch', () => ({
  default: () => <h1>User Search</h1>,
}))

vi.mock('./pages/UserList', () => ({
  default: () => <h1>User List</h1>,
}))

describe('App', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getSession.mockResolvedValue({ loggedIn: false })
  })

  it('shows the home page for an unauthenticated session', async () => {
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Home' })).toBeInTheDocument()
    expect(screen.getByText('Guest')).toBeInTheDocument()
  })

  it('sends unauthenticated users to login for protected pages', async () => {
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'My List' }))

    expect(await screen.findByRole('heading', { name: 'Login' })).toBeInTheDocument()
  })

  it('stores the username after a successful login', async () => {
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Login' }))
    await user.click(screen.getByRole('button', { name: 'Log in as Alice' }))

    expect(await screen.findByText('alice')).toBeInTheDocument()
  })
})
