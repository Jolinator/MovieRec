import React, { useEffect, useState } from 'react'
import Home from './pages/Home'
import Login from './pages/Login'
import MyList from './pages/MyList'
import UserList from './pages/UserList'
import UserSearch from './pages/UserSearch'
import { getSession, logout, setUnauthorizedHandler } from './services/api'

export default function App() {
  const [page, setPage] = useState('home')
  const [username, setUsername] = useState('')
  const [loginNotice, setLoginNotice] = useState('')
  const [selectedUser, setSelectedUser] = useState(null)
  const [isSessionLoading, setIsSessionLoading] = useState(true)

  useEffect(() => setUnauthorizedHandler(() => {
    setUsername('')
    setSelectedUser(null)
    setLoginNotice('Your session has expired. Please log in again.')
    setPage('login')
  }), [])

  useEffect(() => {
    let isCurrent = true
    getSession().then((session) => {
      if (isCurrent && session?.loggedIn === true) {
        setUsername(session.username)
      }
    }).catch(() => {
      // An unauthenticated session is the normal initial state.
    }).finally(() => {
      if (isCurrent) {
        setIsSessionLoading(false)
      }
    })
    return () => {
      isCurrent = false
    }
  }, [])

  if (isSessionLoading) {
    return null
  }

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
        onSearchClick={() => username ? setPage('user-search') : setPage('login')}
      />
    )
  }

  if (page === 'user-search') {
    return (
      <UserSearch
        onHomeClick={() => setPage('home')}
        onMyListClick={() => username ? setPage('my-list') : setPage('login')}
        onSearchClick={() => setPage('user-search')}
        onLoginClick={() => setPage('login')}
        onLogout={handleLogout}
        onUserSelect={(user) => {
          setSelectedUser(user)
          setPage('user-list')
        }}
        username={username}
      />
    )
  }

  if (page === 'user-list' && selectedUser) {
    return (
      <UserList
        user={selectedUser}
        onHomeClick={() => setPage('home')}
        onMyListClick={() => username ? setPage('my-list') : setPage('login')}
        onSearchClick={() => setPage('user-search')}
        onBackToSearch={() => setPage('user-search')}
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
      onSearchClick={() => {
        if (username) {
          setPage('user-search')
        } else {
          setPage('login')
        }
      }}
      username={username}
    />
  )
}
