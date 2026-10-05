import React, { useEffect, useState } from 'react'
import ErrorPopup from '../components/ErrorPopup'
import LoginButton from '../components/LoginButton'
import { getHealth } from '../services/api'
import styles from './Home.module.css'

export default function Home() {
  const [error, setError] = useState('')

  useEffect(() => {
    getHealth().catch((requestError) => {
      setError(requestError instanceof Error ? requestError.message : 'Unable to reach the backend.')
    })
  }, [])

  return (
    <main className={styles.home}>
      <h1 className={styles.title}>HOME</h1>
      <LoginButton />

      {error && <ErrorPopup message={error} onClose={() => setError('')} />}
    </main>
  )
}
