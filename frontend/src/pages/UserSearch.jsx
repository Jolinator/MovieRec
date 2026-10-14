import React, { useState } from 'react'
import ErrorPopup from '../components/ErrorPopup'
import LogoutBanner from '../components/LogoutBanner'
import Sidebar from '../components/Sidebar'
import RecommendationChat from '../components/RecommendationChat'
import UserBadge from '../components/UserBadge'
import UserSearchBar from '../components/UserSearchBar'
import UserSearchResult from '../components/UserSearchResult'
import { searchUsers } from '../services/api'
import styles from './Home.module.css'

export default function UserSearch({ onHomeClick, onMyListClick, onSearchClick, onLoginClick, onLogout, onUserSelect, onRecommendationClick, username }) {
  const [query, setQuery] = useState('')
  const [users, setUsers] = useState([])
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [isLogoutBannerOpen, setIsLogoutBannerOpen] = useState(false)
  const [hasSearched, setHasSearched] = useState(false)
  const [isChatOpen, setIsChatOpen] = useState(false)

  function openRecommendations() {
    setIsChatOpen(true)
  }

  async function handleSearch(event) {
    event.preventDefault()
    setIsLoading(true)
    setError('')
    setHasSearched(true)
    try {
      setUsers(await searchUsers(query.trim()))
    } catch (requestError) {
      setUsers([])
      setError(requestError instanceof Error ? requestError.message : 'Unable to search users.')
    } finally {
      setIsLoading(false)
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
          <p>Find what your friends are watching!.</p>
        </section>
        <UserSearchBar
          value={query}
          onChange={setQuery}
          onSubmit={handleSearch}
          isLoading={isLoading}
        />
        {hasSearched && users.length === 0 && !isLoading && <p>No users found.</p>}
        {users.map((user) => (
          <UserSearchResult key={user.id} user={user} onClick={onUserSelect} />
        ))}
        {error && <ErrorPopup message={error} onClose={() => setError('')} />}
        {isLogoutBannerOpen && (
          <LogoutBanner onCancel={() => setIsLogoutBannerOpen(false)} onConfirm={confirmLogout} />
        )}
        {isChatOpen && <RecommendationChat onClose={() => setIsChatOpen(false)} />}
      </main>
    </div>
  )
}
