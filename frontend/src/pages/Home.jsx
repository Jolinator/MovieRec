import React, { useEffect, useState } from 'react'
import ErrorPopup from '../components/ErrorPopup'
import LoginButton from '../components/LoginButton'
import UserBadge from '../components/UserBadge'
import { getHealth } from '../services/api'
import styles from './Home.module.css'

export default function Home({ onLoginClick, username }) {
  const [error, setError] = useState('')

  useEffect(() => {
    getHealth().catch((requestError) => {
      setError(requestError instanceof Error ? requestError.message : 'Unable to reach the backend.')
    })
  }, [])

  return (
    <main className={styles.home}>
      <UserBadge username={username} />
      <h1 className={styles.title}>HOME</h1>
      <LoginButton onClick={onLoginClick} />

      {error && <ErrorPopup message={error} onClose={() => setError('')} />}
    </main>
  )
}
