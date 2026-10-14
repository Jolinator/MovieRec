import React from 'react'
import LoginButton from './LoginButton'
import styles from './Sidebar.module.css'

export default function Sidebar({ isLoggedIn, onAuthClick, onHomeClick, onMyListClick, onRecommendationClick, onSearchClick, onNavigate }) {
  function navigate(callback) {
    onNavigate?.()
    callback?.()
  }

  return (
    <aside className={styles.sidebar} aria-label="Services">
      <nav className={styles.services}>
        <div className={styles.navigation}>
          <button type="button" className={styles.item} onClick={() => navigate(onHomeClick)}>
            Home
          </button>
          <button type="button" className={styles.item} onClick={() => navigate(onMyListClick)}>
            My List
          </button>
          {onRecommendationClick && (
            <button type="button" className={styles.item} onClick={() => navigate(onRecommendationClick)}>
              Recommendation Chat
            </button>
          )}
          {onSearchClick && (
            <button type="button" className={styles.item} onClick={() => navigate(onSearchClick)}>
              Search Users
            </button>
          )}
        </div>
        <LoginButton isLoggedIn={isLoggedIn} onClick={onAuthClick} />
      </nav>
    </aside>
  )
}
