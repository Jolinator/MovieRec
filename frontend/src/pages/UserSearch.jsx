import React, { useCallback, useEffect, useRef, useState } from 'react'
import ErrorPopup from '../components/ErrorPopup'
import LogoutBanner from '../components/LogoutBanner'
import Sidebar from '../components/Sidebar'
import RecommendationChat from '../components/RecommendationChat'
import UserBadge from '../components/UserBadge'
import UserSearchBar from '../components/UserSearchBar'
import UserSearchResult from '../components/UserSearchResult'
import { addMovieToList, getMyList, removeMovieFromList, searchUsers } from '../services/api'
import styles from './Home.module.css'

export default function UserSearch({ onHomeClick, onMyListClick, onSearchClick, onLoginClick, onLogout, onUserSelect, onRecommendationClick, username }) {
  const [query, setQuery] = useState('')
  const [users, setUsers] = useState([])
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [isLogoutBannerOpen, setIsLogoutBannerOpen] = useState(false)
  const [hasSearched, setHasSearched] = useState(false)
  const [isChatOpen, setIsChatOpen] = useState(false)
  const [myListMovieIds, setMyListMovieIds] = useState(() => new Set())
  const [isUpdatingList, setIsUpdatingList] = useState(false)
  const searchRequestId = useRef(0)
  const debounceTimer = useRef(null)
  const queryValue = useRef('')

  const runSearch = useCallback(async (searchQuery) => {
    const normalizedQuery = searchQuery.trim()
    if (!normalizedQuery) {
      setUsers([])
      setHasSearched(false)
      setIsLoading(false)
      return
    }
    if (normalizedQuery !== queryValue.current.trim()) {
      return
    }

    const requestId = ++searchRequestId.current
    setIsLoading(true)
    setError('')
    setHasSearched(true)
    try {
      const results = await searchUsers(normalizedQuery)
      if (requestId === searchRequestId.current) {
        setUsers(results)
      }
    } catch (requestError) {
      if (requestId === searchRequestId.current) {
        setUsers([])
        setError(requestError instanceof Error ? requestError.message : 'Unable to search users.')
      }
    } finally {
      if (requestId === searchRequestId.current) {
        setIsLoading(false)
      }
    }
  }, [])

  useEffect(() => {
    const normalizedQuery = query.trim()
    clearTimeout(debounceTimer.current)

    if (!normalizedQuery) {
      setUsers([])
      setHasSearched(false)
      setIsLoading(false)
      setError('')
      return undefined
    }

    debounceTimer.current = setTimeout(() => runSearch(normalizedQuery), 300)
    return () => {
      clearTimeout(debounceTimer.current)
      searchRequestId.current += 1
    }
  }, [query, runSearch])

  useEffect(() => () => {
    clearTimeout(debounceTimer.current)
    searchRequestId.current += 1
  }, [])

  useEffect(() => {
    let isCurrentRequest = true
    if (!username) {
      setMyListMovieIds(new Set())
      return () => {
        isCurrentRequest = false
      }
    }

    getMyList().then((movies) => {
      if (isCurrentRequest) {
        setMyListMovieIds(new Set(movies.map((movie) => movie.tmdbId)))
      }
    }).catch((requestError) => {
      if (isCurrentRequest) {
        setError(requestError instanceof Error ? requestError.message : 'Unable to load your list.')
      }
    })

    return () => {
      isCurrentRequest = false
    }
  }, [username])

  function openRecommendations() {
    setIsChatOpen(true)
  }

  function handleSearch(event) {
    event.preventDefault()
    clearTimeout(debounceTimer.current)
    runSearch(query)
  }

  function handleQueryChange(nextQuery) {
    searchRequestId.current += 1
    queryValue.current = nextQuery
    setQuery(nextQuery)
    setUsers([])
    setHasSearched(false)
    setIsLoading(false)
    setError('')
  }

  async function handleToggleList(movie) {
    if (!username) return false

    const isInList = myListMovieIds.has(movie.tmdbId)
    setIsUpdatingList(true)
    try {
      if (isInList) {
        await removeMovieFromList(movie.tmdbId)
        setMyListMovieIds((currentIds) => {
          const nextIds = new Set(currentIds)
          nextIds.delete(movie.tmdbId)
          return nextIds
        })
        return false
      }

      await addMovieToList(movie.tmdbId)
      setMyListMovieIds((currentIds) => new Set(currentIds).add(movie.tmdbId))
      return true
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Unable to update your list.')
      throw requestError
    } finally {
      setIsUpdatingList(false)
    }
  }

  async function confirmLogout() {
    try {
      await onLogout()
      setIsLogoutBannerOpen(false)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Unable to log out.')
    }
  }

  return (
    <div className={styles.shell}>
      <header className={styles.topbar}>
        <button type="button" className={styles.titleButton} onClick={onHomeClick}>MovieRec</button>
        <UserBadge username={username} />
      </header>
      <Sidebar
        isLoggedIn={Boolean(username)}
        onHomeClick={onHomeClick}
        onMyListClick={onMyListClick}
        onSearchClick={onSearchClick}
        onRecommendationClick={openRecommendations}
        onNavigate={() => setIsChatOpen(false)}
        onAuthClick={() => username ? setIsLogoutBannerOpen(true) : onLoginClick()}
      />
      <main className={styles.content}>
        <section className={styles.intro}>
          <p className={styles.eyebrow}>Community</p>
          <h2>Search users</h2>
          <p>Find people by username. Results update as you type.</p>
        </section>
        <UserSearchBar
          value={query}
          onChange={handleQueryChange}
          onSubmit={handleSearch}
          isLoading={isLoading}
        />
        {query.trim() && isLoading && <p role="status">Searching users...</p>}
        {query.trim() && !isLoading && hasSearched && users.length > 0 && (
          <p role="status">Found {users.length} {users.length === 1 ? 'user' : 'users'}.</p>
        )}
        {hasSearched && users.length === 0 && !isLoading && !error && <p>No users found.</p>}
        {users.map((user) => (
          <UserSearchResult key={user.id} user={user} onClick={onUserSelect} />
        ))}
        {error && <ErrorPopup message={error} onClose={() => setError('')} />}
        {isLogoutBannerOpen && (
          <LogoutBanner onCancel={() => setIsLogoutBannerOpen(false)} onConfirm={confirmLogout} />
        )}
        {isChatOpen && (
          <RecommendationChat
            onClose={() => setIsChatOpen(false)}
            onToggleList={username ? handleToggleList : undefined}
            isAddingToList={isUpdatingList}
            myListMovieIds={myListMovieIds}
          />
        )}
      </main>
    </div>
  )
}
