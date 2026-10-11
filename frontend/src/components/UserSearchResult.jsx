import React from 'react'
import styles from './UserSearchResult.module.css'

export default function UserSearchResult({ user, onClick }) {
  return (
    <button type="button" className={styles.result} onClick={() => onClick(user)}>
      {user.username}
    </button>
  )
}
