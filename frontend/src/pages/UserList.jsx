import React, { useEffect, useState } from 'react'
import ErrorPopup from '../components/ErrorPopup'
import LogoutBanner from '../components/LogoutBanner'
import MovieDetails from '../components/MovieDetails'
import MovieGrid from '../components/MovieGrid'
import Sidebar from '../components/Sidebar'
import UserBadge from '../components/UserBadge'
import {
  addMovieToList,
  getMyList,
  getUserList,
  removeMovieFromList,
} from '../services/api'
import styles from './Home.module.css'

export default function UserList({ user, onHomeClick, onMyListClick, onSearchClick, onBackToSearch, onLoginClick, onLogout, username }) {
  const [movies, setMovies] = useState([])
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [isLogoutBannerOpen, setIsLogoutBannerOpen] = useState(false)
  const [selectedMovie, setSelectedMovie] = useState(null)
  const [myListMovieIds, setMyListMovieIds] = useState(() => new Set())
  const [isUpdatingList, setIsUpdatingList] = useState(false)

  useEffect(() => {
    let isCurrentRequest = true
    Promise.all([getUserList(user.id), getMyList()])
      .then(([loadedMovies, myList]) => {
        if (!isCurrentRequest) return
        setMovies(loadedMovies)
        setMyListMovieIds(new Set(myList.map((movie) => movie.tmdbId)))
      })
      .catch((requestError) => {
        if (isCurrentRequest) {
          setError(requestError instanceof Error ? requestError.message : 'Unable to load this list.')
        }
      })
      .finally(() => {
        if (isCurrentRequest) setIsLoading(false)
      })
    return () => {
      isCurrentRequest = false
    }
  }, [user.id])

  async function handleToggleList() {
    if (!selectedMovie) return false

    const tmdbId = selectedMovie.tmdbId
    const isInMyList = myListMovieIds.has(tmdbId)
    setIsUpdatingList(true)
    try {
      if (isInMyList) {
        await removeMovieFromList(tmdbId)
        setMyListMovieIds((currentIds) => {
          const nextIds = new Set(currentIds)
          nextIds.delete(tmdbId)
          return nextIds
        })
        return false
      }

      await addMovieToList(tmdbId)
      setMyListMovieIds((currentIds) => new Set(currentIds).add(tmdbId))
      return true
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Unable to update your list.')
      throw requestError
    } finally {
      setIsUpdatingList(false)
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
        onAuthClick={() => username ? setIsLogoutBannerOpen(true) : onLoginClick()}
      />
      <main className={styles.content}>
        <section className={styles.intro}>
          <button type="button" className={styles.titleButton} onClick={onBackToSearch}>Back to search</button>
          <p className={styles.eyebrow}>Public list</p>
          <h2>{user.username}'s List</h2>
        </section>
        {isLoading ? <p>Loading list...</p> : <MovieGrid movies={movies} onSelect={setSelectedMovie} />}
        {error && <ErrorPopup message={error} onClose={() => setError('')} />}
        {isLogoutBannerOpen && (
          <LogoutBanner onCancel={() => setIsLogoutBannerOpen(false)} onConfirm={confirmLogout} />
        )}
        {selectedMovie && (
          <MovieDetails
            movie={selectedMovie}
            onClose={() => setSelectedMovie(null)}
            onToggleList={handleToggleList}
            isAddingToList={isUpdatingList}
            isInList={myListMovieIds.has(selectedMovie.tmdbId)}
            showListActions={Boolean(username)}
          />
        )}
      </main>
    </div>
  )
}
