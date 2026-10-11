import React from 'react'
import LoginButton from './LoginButton'
import styles from './Sidebar.module.css'

export default function Sidebar({ isLoggedIn, onAuthClick, onHomeClick, onMyListClick, onSearchClick }) {
  return (
    <aside className={styles.sidebar} aria-label="Services">
      <nav className={styles.services}>
        <div className={styles.navigation}>
          <button type="button" className={styles.item} onClick={onHomeClick}>
            Home
          </button>
          <button type="button" className={styles.item} onClick={onMyListClick}>
            My List
          </button>
          <button type="button" className={styles.item} onClick={onSearchClick}>
            Search Users
          </button>
        </div>
        <LoginButton isLoggedIn={isLoggedIn} onClick={onAuthClick} />
      </nav>
    </aside>
  )
}
