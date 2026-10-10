import React, { useEffect, useState } from 'react'
import ErrorPopup from '../components/ErrorPopup'
import MovieDetails from '../components/MovieDetails'
import MovieGrid from '../components/MovieGrid'
import Sidebar from '../components/Sidebar'
import UserBadge from '../components/UserBadge'
import { addMovieToList, getMyList, removeMovieFromList } from '../services/api'
import styles from './Home.module.css'

export default function MyList({ onHomeClick, onLoginClick, onLogout, username }) {
  const [movies, setMovies] = useState([])
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [isLogoutBannerOpen, setIsLogoutBannerOpen] = useState(false)
  const [selectedMovie, setSelectedMovie] = useState(null)
  const [isAddingToList, setIsAddingToList] = useState(false)

  useEffect(() => {
    let isCurrentRequest = true

    getMyList().then((loadedMovies) => {
      if (isCurrentRequest) {
        setMovies(loadedMovies)
      }
    }).catch((requestError) => {
      if (isCurrentRequest) {
        setError(requestError instanceof Error ? requestError.message : 'Unable to load your list.')
      }
    }).finally(() => {
      if (isCurrentRequest) {
        setIsLoading(false)
      }
    })

    return () => {
      isCurrentRequest = false
    }
  }, [])

  async function confirmLogout() {
    try {
      await onLogout()
      setIsLogoutBannerOpen(false)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Unable to log out.')
    }
  }

  async function handleToggleList() {
    setIsAddingToList(true)
    try {
      if (movies.some((movie) => movie.tmdbId === selectedMovie.tmdbId)) {
        await removeMovieFromList(selectedMovie.tmdbId)
        setMovies((currentMovies) => currentMovies.filter((movie) => movie.tmdbId !== selectedMovie.tmdbId))
        return false
      }

      await addMovieToList(selectedMovie.tmdbId)
      setMovies((currentMovies) => [...currentMovies, selectedMovie])
      return true
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Unable to update your list.')
      throw requestError
    } finally {
      setIsAddingToList(false)
    }
  }

  return (
    <div className={styles.shell}>
      <header className={styles.topbar}>
        <button type="button" className={styles.titleButton} onClick={onHomeClick}>
          MovieRec
        </button>
        <UserBadge username={username} />
      </header>

      <Sidebar
        isLoggedIn={Boolean(username)}
        onHomeClick={onHomeClick}
        onMyListClick={() => {}}
        onAuthClick={() => {
          if (username) {
            setIsLogoutBannerOpen(true)
          } else {
            onLoginClick()
          }
        }}
      />

      <main className={styles.content}>
        <section className={styles.intro}>
          <p className={styles.eyebrow}>Your collection</p>
          <h2>My List</h2>
        </section>

        {isLoading ? <p>Loading your list...</p> : <MovieGrid movies={movies} onSelect={setSelectedMovie} />}
        {error && <ErrorPopup message={error} onClose={() => setError('')} />}
        {selectedMovie && (
          <MovieDetails
            movie={selectedMovie}
            onClose={() => setSelectedMovie(null)}
            onToggleList={handleToggleList}
            isAddingToList={isAddingToList}
            isInList={movies.some((movie) => movie.tmdbId === selectedMovie.tmdbId)}
          />
        )}
      </main>
    </div>
  )
}
