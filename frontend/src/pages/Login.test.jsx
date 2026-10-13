import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Login from './Login'
import { login } from '../services/api'

vi.mock('../services/api', () => ({
  login: vi.fn(),
  signup: vi.fn(),
}))

describe('Login page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('logs in with the submitted credentials', async () => {
    login.mockResolvedValue({ loggedIn: true })
    const onLoginSuccess = vi.fn()
    const user = userEvent.setup()
    render(<Login onBack={vi.fn()} onLoginSuccess={onLoginSuccess} notice="" />)

    await user.type(screen.getByLabelText('Username'), 'alice')
    await user.type(screen.getByLabelText('Password'), 'password123')
    await user.click(screen.getByRole('button', { name: 'Login' }))

    expect(login).toHaveBeenCalledWith({ username: 'alice', password: 'password123' })
    expect(onLoginSuccess).toHaveBeenCalledWith('alice')
  })

  it('shows an error when login fails', async () => {
    login.mockRejectedValue(new Error('Request failed (401)'))
    const user = userEvent.setup()
    render(<Login onBack={vi.fn()} onLoginSuccess={vi.fn()} notice="" />)

    await user.type(screen.getByLabelText('Username'), 'alice')
    await user.type(screen.getByLabelText('Password'), 'wrongpass')
    await user.click(screen.getByRole('button', { name: 'Login' }))

    expect(await screen.findByRole('alertdialog')).toHaveTextContent('Request failed (401)')
  })
})
