import React, { useState } from 'react'
import Home from './pages/Home'
import Login from './pages/Login'
import MyList from './pages/MyList'
import { logout } from './services/api'

export default function App() {
  const [page, setPage] = useState('home')
  const [username, setUsername] = useState('')
  const [loginNotice, setLoginNotice] = useState('')

  if (page === 'login') {
    return (
      <Login
        onBack={() => setPage('home')}
        notice={loginNotice}
        onLoginSuccess={(loggedInUsername) => {
          setUsername(loggedInUsername)
          setPage('home')
          setLoginNotice('')
        }}
      />
    )
  }

  async function handleLogout() {
    await logout()
    setUsername('')
    setPage('home')
  }

  if (page === 'my-list') {
    return (
      <MyList
        onHomeClick={() => setPage('home')}
        onLoginClick={() => setPage('login')}
        onLogout={handleLogout}
        username={username}
      />
    )
  }

  return (
    <Home
      onHomeClick={() => setPage('home')}
      onLoginClick={() => setPage('login')}
      onRequireLogin={() => { setLoginNotice('Log in to use MovieRec features.'); setPage('login') }}
      onLogout={handleLogout}
      onMyListClick={() => {
        if (username) {
          setPage('my-list')
        } else {
          setPage('login')
        }
      }}
      username={username}
    />
  )
}
