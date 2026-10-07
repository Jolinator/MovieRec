import React, { useEffect, useState } from 'react'
import ErrorPopup from '../components/ErrorPopup'
import LoginButton from '../components/LoginButton'
import MovieGrid from '../components/MovieGrid'
import UserBadge from '../components/UserBadge'
import { getHealth, getMovies } from '../services/api'
import styles from './Home.module.css'

export default function Home({ onLoginClick, username }) {
  const [error, setError] = useState('')
  const [movies, setMovies] = useState([])
  const [isLoadingMovies, setIsLoadingMovies] = useState(true)

  useEffect(() => {
    Promise.all([getHealth(), getMovies({ limit: 20 })]).then(([, loadedMovies]) => {
      setMovies(loadedMovies)
    }).catch((requestError) => {
      setError(requestError instanceof Error ? requestError.message : 'Unable to reach the backend.')
    }).finally(() => {
      setIsLoadingMovies(false)
    })
  }, [])

  return (
    <main className={styles.home}>
      <UserBadge username={username} />
      <h1 className={styles.title}>MOVIES</h1>
      <LoginButton onClick={onLoginClick} />

      {isLoadingMovies ? <p>Loading movies...</p> : <MovieGrid movies={movies} />}
      {error && <ErrorPopup message={error} onClose={() => setError('')} />}
    </main>
  )
}
