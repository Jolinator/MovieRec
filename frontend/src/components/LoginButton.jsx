import React from 'react'
import styles from './LoginButton.module.css'

export default function LoginButton({ isLoggedIn, onClick }) {
  return (
    <button type="button" className={styles.button} onClick={onClick}>
      {isLoggedIn ? 'Logout' : 'Login'}
    </button>
  )
}
