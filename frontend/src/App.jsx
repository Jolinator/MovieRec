import React, { useState } from 'react'
import Home from './pages/Home'
import Login from './pages/Login'
import { logout } from './services/api'

export default function App() {
  const [page, setPage] = useState('home')
  const [username, setUsername] = useState('')

  if (page === 'login') {
    return (
      <Login
        onBack={() => setPage('home')}
        onLoginSuccess={(loggedInUsername) => {
          setUsername(loggedInUsername)
          setPage('home')
        }}
      />
    )
  }

  async function handleLogout() {
    await logout()
    setUsername('')
  }

  return (
    <Home
      onLoginClick={() => setPage('login')}
      onLogout={handleLogout}
      username={username}
    />
  )
}
