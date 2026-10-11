import React, { useCallback, useEffect, useRef, useState } from 'react'
import ErrorPopup from '../components/ErrorPopup'
import GenreFilter from '../components/GenreFilter'
import LogoutBanner from '../components/LogoutBanner'
import MovieGrid from '../components/MovieGrid'
import MovieDetails from '../components/MovieDetails'
import Sidebar from '../components/Sidebar'
import RecommendationChat from '../components/RecommendationChat'
import UserBadge from '../components/UserBadge'
import { addMovieToList, getGenres, getHealth, getMovies, getMyList, removeMovieFromList } from '../services/api'
import styles from './Home.module.css'

export default function Home({ onLoginClick, onRequireLogin, onLogout, onHomeClick, onMyListClick, onSearchClick, username }) {
  const pageSize = 99
  const [error, setError] = useState('')
  const [movies, setMovies] = useState([])
  const [genres, setGenres] = useState([])
  const [selectedGenreId, setSelectedGenreId] = useState(null)
  const [isLoadingMovies, setIsLoadingMovies] = useState(true)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [hasMoreMovies, setHasMoreMovies] = useState(true)
  const [isLogoutBannerOpen, setIsLogoutBannerOpen] = useState(false)
  const [selectedMovie, setSelectedMovie] = useState(null)
  const [isAddingToList, setIsAddingToList] = useState(false)
  const [myListMovieIds, setMyListMovieIds] = useState(new Set())
  const [isChatOpen, setIsChatOpen] = useState(false)
  const loadMoreRef = useRef(null)

  function openRecommendations() {
    if (!username) {
      onRequireLogin()
      return
    }
    setIsChatOpen(true)
  }

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

  async function handleToggleList() {
    if (!username) {
      onLoginClick()
      return false
    }

    setIsAddingToList(true)
    try {
      if (myListMovieIds.has(selectedMovie.tmdbId)) {
        await removeMovieFromList(selectedMovie.tmdbId)
        setMyListMovieIds((currentIds) => {
          const nextIds = new Set(currentIds)
          nextIds.delete(selectedMovie.tmdbId)
          return nextIds
        })
        return false
      }

      await addMovieToList(selectedMovie.tmdbId)
      setMyListMovieIds((currentIds) => new Set(currentIds).add(selectedMovie.tmdbId))
      return true
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Unable to update your list.')
      throw requestError
    } finally {
      setIsAddingToList(false)
    }
  }

  useEffect(() => {
    if (!username) {
      setMyListMovieIds(new Set())
      return
    }

    getMyList().then((list) => {
      setMyListMovieIds(new Set(list.map((movie) => movie.tmdbId)))
    }).catch((requestError) => {
        setError(requestError instanceof Error ? requestError.message : 'Unable to load your list.')
      })
    }, [username])

    useEffect(() => {
    getGenres().then((loadedGenres) => {
      setGenres(loadedGenres)
    }).catch((requestError) => {
      setError(requestError instanceof Error ? requestError.message : 'Unable to load genres.')
    })
  }, [])

  useEffect(() => {
    let isCurrentRequest = true
    setIsLoadingMovies(true)
    setMovies([])
    setHasMoreMovies(true)

    Promise.all([getHealth(), getMovies({ limit: pageSize, genreId: selectedGenreId })]).then(([, loadedMovies]) => {
      if (!isCurrentRequest) {
        return
      }
      setMovies(loadedMovies)
      setHasMoreMovies(loadedMovies.length === pageSize)
    }).catch((requestError) => {
      if (!isCurrentRequest) {
        return
      }
      setError(requestError instanceof Error ? requestError.message : 'Unable to reach the backend.')
    }).finally(() => {
      if (isCurrentRequest) {
        setIsLoadingMovies(false)
      }
    })

    return () => {
      isCurrentRequest = false
    }
  }, [selectedGenreId])

  const loadMoreMovies = useCallback(async () => {
    if (isLoadingMovies || isLoadingMore || !hasMoreMovies) {
      return
    }

    setIsLoadingMore(true)
    try {
      const loadedMovies = await getMovies({
        limit: pageSize,
        offset: movies.length,
        genreId: selectedGenreId,
      })
      setMovies((currentMovies) => [...currentMovies, ...loadedMovies])
      setHasMoreMovies(loadedMovies.length === pageSize)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Unable to load more movies.')
    } finally {
      setIsLoadingMore(false)
    }
  }, [hasMoreMovies, isLoadingMore, isLoadingMovies, movies.length, selectedGenreId])

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

      <Sidebar
        isLoggedIn={Boolean(username)}
        onHomeClick={onHomeClick}
        onMyListClick={onMyListClick}
        onSearchClick={onSearchClick}
        onAuthClick={handleAuthAction}
        onRecommendationClick={openRecommendations}
      />

      <main className={styles.content}>
        <section className={styles.intro}>
          <p className={styles.eyebrow}>Discover your next favourite</p>
          <h2>Popular movies</h2>
          <GenreFilter
            genres={genres}
            selectedGenreId={selectedGenreId}
            onSelect={setSelectedGenreId}
          />
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
          <MovieDetails
            movie={selectedMovie}
            onClose={() => setSelectedMovie(null)}
            onToggleList={handleToggleList}
            isAddingToList={isAddingToList}
            isInList={myListMovieIds.has(selectedMovie.tmdbId)}
          />
        )}
        {isChatOpen && <RecommendationChat onClose={() => setIsChatOpen(false)} />}
      </main>
    </div>
  )
}
