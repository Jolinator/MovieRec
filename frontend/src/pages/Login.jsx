import React, { useState } from 'react'
import ErrorPopup from '../components/ErrorPopup'
import { login, signup } from '../services/api'
import styles from './Login.module.css'

export default function Login({ onBack, onLoginSuccess }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  async function handleSubmit(event) {
    event.preventDefault()
    setIsSubmitting(true)
    setSuccess('')

    try {
      const response = await login({ username, password })

      if (response?.loggedIn === true) {
        onLoginSuccess(username)
        return
      }

      setError('Incorrect username or password.')
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Unable to log in.')
    } finally {
      setIsSubmitting(false)
    }
  }

  async function handleSignup(event) {
    if (!event.currentTarget.form.reportValidity()) {
      return
    }

    setIsSubmitting(true)
    setError('')
    setSuccess('')

    try {
      const response = await signup({ username, password })

      if (response?.success === true) {
        setPassword('')
        setSuccess('Account created successfully. You can now log in.')
        return
      }

      setError('Unable to sign up.')
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Unable to sign up.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <main className={styles.login}>
      <button type="button" className={styles.backButton} onClick={onBack}>Back</button>
      <form className={styles.form} onSubmit={handleSubmit}>
        <h1>Login</h1>

        <label>
          Username
          <input
            type="text"
            name="username"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            autoComplete="username"
            required
          />
        </label>

        <label>
          Password
          <input
            type="password"
            name="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete="current-password"
            required
          />
        </label>

        <button type="submit" disabled={isSubmitting}>
          {isSubmitting ? 'Logging in…' : 'Login'}
        </button>
        <button
          type="button"
          className={styles.signUpButton}
          onClick={handleSignup}
          disabled={isSubmitting}
        >
          {isSubmitting ? 'Signing up…' : 'Sign up'}
        </button>
      </form>

      {success && <p role="status">{success}</p>}
      {error && <ErrorPopup message={error} onClose={() => setError('')} />}
    </main>
  )
}
