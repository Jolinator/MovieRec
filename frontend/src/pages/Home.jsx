import React, { useCallback, useEffect, useRef, useState } from 'react'
import ErrorPopup from '../components/ErrorPopup'
import LoginButton from '../components/LoginButton'
import LogoutBanner from '../components/LogoutBanner'
import MovieGrid from '../components/MovieGrid'
import MovieDetails from '../components/MovieDetails'
import UserBadge from '../components/UserBadge'
import { getHealth, getMovies } from '../services/api'
import styles from './Home.module.css'

export default function Home({ onLoginClick, onLogout, username }) {
  const pageSize = 99
  const [error, setError] = useState('')
  const [movies, setMovies] = useState([])
  const [isLoadingMovies, setIsLoadingMovies] = useState(true)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [hasMoreMovies, setHasMoreMovies] = useState(true)
  const [isLogoutBannerOpen, setIsLogoutBannerOpen] = useState(false)
  const [selectedMovie, setSelectedMovie] = useState(null)
  const loadMoreRef = useRef(null)

  async function handleAuthAction() {
    if (!username) {
      onLoginClick()
      return
    }

    setIsLogoutBannerOpen(true)
  }

  async function confirmLogout() {
    try {
      await onLogout()
      setIsLogoutBannerOpen(false)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Unable to log out.')
    }
  }

  useEffect(() => {
    Promise.all([getHealth(), getMovies({ limit: pageSize })]).then(([, loadedMovies]) => {
      setMovies(loadedMovies)
      setHasMoreMovies(loadedMovies.length === pageSize)
    }).catch((requestError) => {
      setError(requestError instanceof Error ? requestError.message : 'Unable to reach the backend.')
    }).finally(() => {
      setIsLoadingMovies(false)
    })
  }, [])

  const loadMoreMovies = useCallback(async () => {
    if (isLoadingMovies || isLoadingMore || !hasMoreMovies) {
      return
    }

    setIsLoadingMore(true)
    try {
      const loadedMovies = await getMovies({ limit: pageSize, offset: movies.length })
      setMovies((currentMovies) => [...currentMovies, ...loadedMovies])
      setHasMoreMovies(loadedMovies.length === pageSize)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Unable to load more movies.')
    } finally {
      setIsLoadingMore(false)
    }
  }, [hasMoreMovies, isLoadingMore, isLoadingMovies, movies.length])

  useEffect(() => {
    const loadMoreElement = loadMoreRef.current
    if (!loadMoreElement) {
      return undefined
    }

    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) {
        loadMoreMovies()
      }
    }, { rootMargin: '300px' })

    observer.observe(loadMoreElement)
    return () => observer.disconnect()
  }, [loadMoreMovies])

  return (
    <div className={styles.shell}>
      <header className={styles.topbar}>
        <h1 className={styles.title}>MovieRec</h1>
        <UserBadge username={username} />
      </header>

      <aside className={styles.sidebar} aria-label="Services">
        <nav className={styles.services}>
          <LoginButton isLoggedIn={Boolean(username)} onClick={handleAuthAction} />
        </nav>
      </aside>

      <main className={styles.content}>
        <section className={styles.intro}>
          <p className={styles.eyebrow}>Discover your next favourite</p>
          <h2>Popular movies</h2>
        </section>

        {isLoadingMovies ? (
          <p>Loading movies...</p>
        ) : (
          <>
            <MovieGrid movies={movies} onSelect={setSelectedMovie} />
            <div ref={loadMoreRef} aria-hidden="true" />
            {isLoadingMore && <p>Loading more movies...</p>}
          </>
        )}
        {error && <ErrorPopup message={error} onClose={() => setError('')} />}
        {isLogoutBannerOpen && (
          <LogoutBanner
            onCancel={() => setIsLogoutBannerOpen(false)}
            onConfirm={confirmLogout}
          />
        )}
        {selectedMovie && (
          <MovieDetails movie={selectedMovie} onClose={() => setSelectedMovie(null)} />
        )}
      </main>
    </div>
  )
}
