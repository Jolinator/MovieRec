import React, { useCallback, useEffect, useRef, useState } from 'react'
import ErrorPopup from '../components/ErrorPopup'
import GenreFilter from '../components/GenreFilter'
import LogoutBanner from '../components/LogoutBanner'
import MovieGrid from '../components/MovieGrid'
import MovieDetails from '../components/MovieDetails'
import Sidebar from '../components/Sidebar'
import RecommendationChat from '../components/RecommendationChat'
import UserBadge from '../components/UserBadge'
import { addMovieToList, getGenres, getMovies, getMyList, getPersonalizedRecommendations, removeMovieFromList } from '../services/api'
import styles from './Home.module.css'

export default function Home({ onLoginClick, onRequireLogin, onLogout, onHomeClick, onMyListClick, onSearchClick, username }) {
  const pageSize = 99
  const [error, setError] = useState('')
  const [movies, setMovies] = useState([])
  const [movieSearch, setMovieSearch] = useState('')
  const [activeMovieSearch, setActiveMovieSearch] = useState('')
  const [genres, setGenres] = useState([])
  const [selectedGenreId, setSelectedGenreId] = useState(null)
  const [isLoadingMovies, setIsLoadingMovies] = useState(true)
  const [moviesLoadFailed, setMoviesLoadFailed] = useState(false)
  const [movieRetryCount, setMovieRetryCount] = useState(0)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [loadMoreFailed, setLoadMoreFailed] = useState(false)
  const [hasMoreMovies, setHasMoreMovies] = useState(true)
  const [isLogoutBannerOpen, setIsLogoutBannerOpen] = useState(false)
  const [selectedMovie, setSelectedMovie] = useState(null)
  const [isAddingToList, setIsAddingToList] = useState(false)
  const [myListMovieIds, setMyListMovieIds] = useState(new Set())
  const [isChatOpen, setIsChatOpen] = useState(false)
  const [personalizedRecommendations, setPersonalizedRecommendations] = useState([])
  const [recommendationStatus, setRecommendationStatus] = useState('idle')
  const [recommendationReloadKey, setRecommendationReloadKey] = useState(0)
  const loadMoreRef = useRef(null)
  const movieQueryGenerationRef = useRef(0)

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

  async function handleToggleList(movie = selectedMovie) {
    if (!username) {
      onLoginClick()
      return false
    }

    if (!movie) {
      return false
    }

    setIsAddingToList(true)
    try {
      if (myListMovieIds.has(movie.tmdbId)) {
        await removeMovieFromList(movie.tmdbId)
        setMyListMovieIds((currentIds) => {
          const nextIds = new Set(currentIds)
          nextIds.delete(movie.tmdbId)
          return nextIds
        })
        setRecommendationReloadKey((key) => key + 1)
        return false
      }

      await addMovieToList(movie.tmdbId)
      setMyListMovieIds((currentIds) => new Set(currentIds).add(movie.tmdbId))
      setRecommendationReloadKey((key) => key + 1)
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
    let isCurrentRequest = true
    let retryTimer

    if (!username) {
      setPersonalizedRecommendations([])
      setRecommendationStatus('idle')
      return () => {
        isCurrentRequest = false
        clearTimeout(retryTimer)
      }
    }

    async function loadRecommendations(isRetry = false) {
      if (!isRetry) {
        setRecommendationStatus('loading')
      }
      try {
        const response = await getPersonalizedRecommendations()
        if (!isCurrentRequest) return
        const status = response.status ?? 'empty'
        setPersonalizedRecommendations(response.movies ?? [])
        setRecommendationStatus(status)
        if (status === 'preparing') {
          retryTimer = setTimeout(() => loadRecommendations(true), 1500)
        }
      } catch (requestError) {
        if (isCurrentRequest) {
          setRecommendationStatus('error')
          setError(requestError instanceof Error ? requestError.message : 'Unable to load recommendations.')
        }
      }
    }

    loadRecommendations()
    return () => {
      isCurrentRequest = false
      clearTimeout(retryTimer)
    }
  }, [username, recommendationReloadKey])

  useEffect(() => {
    let isCurrentRequest = true
    getGenres().then((loadedGenres) => {
      if (isCurrentRequest) setGenres(loadedGenres)
    }).catch(() => {
      // The genre filter is optional; keep movie browsing available without it.
      if (isCurrentRequest) setGenres([])
    })
    return () => {
      isCurrentRequest = false
    }
  }, [])

  useEffect(() => {
    let isCurrentRequest = true
    const requestGeneration = ++movieQueryGenerationRef.current
    setIsLoadingMovies(true)
    setIsLoadingMore(false)
    setMoviesLoadFailed(false)
    setLoadMoreFailed(false)
    setMovies([])
    setHasMoreMovies(true)

    getMovies({
      limit: pageSize,
      genreId: selectedGenreId,
      ...(activeMovieSearch ? { search: activeMovieSearch } : {}),
    }).then((loadedMovies) => {
      if (!isCurrentRequest) {
        return
      }
      setMovies(loadedMovies)
      setHasMoreMovies(loadedMovies.length === pageSize)
    }).catch(() => {
      if (!isCurrentRequest) {
        return
      }
      setMoviesLoadFailed(true)
    }).finally(() => {
      if (isCurrentRequest) {
        setIsLoadingMovies(false)
      }
    })

    return () => {
      isCurrentRequest = false
    }
  }, [activeMovieSearch, movieRetryCount, selectedGenreId])

  const loadMoreMovies = useCallback(async () => {
    if (isLoadingMovies || isLoadingMore || !hasMoreMovies) {
      return
    }

    setIsLoadingMore(true)
    setLoadMoreFailed(false)
    const requestGeneration = movieQueryGenerationRef.current
    try {
      const loadedMovies = await getMovies({
        limit: pageSize,
        offset: movies.length,
        genreId: selectedGenreId,
        ...(activeMovieSearch ? { search: activeMovieSearch } : {}),
      })
      if (requestGeneration !== movieQueryGenerationRef.current) {
        return
      }
      setMovies((currentMovies) => [...currentMovies, ...loadedMovies])
      setHasMoreMovies(loadedMovies.length === pageSize)
    } catch {
      if (requestGeneration === movieQueryGenerationRef.current) {
        setLoadMoreFailed(true)
      }
    } finally {
      if (requestGeneration === movieQueryGenerationRef.current) {
        setIsLoadingMore(false)
      }
    }
  }, [activeMovieSearch, hasMoreMovies, isLoadingMore, isLoadingMovies, movies.length, selectedGenreId])

  function handleMovieSearch(event) {
    event.preventDefault()
    const search = movieSearch.trim()
    setMovieSearch(search)
    setActiveMovieSearch(search)
  }

  function clearMovieSearch() {
    setMovieSearch('')
    setActiveMovieSearch('')
  }

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
        onNavigate={() => setIsChatOpen(false)}
      />

      <main className={styles.content}>
        {username && (
          <section className={styles.intro} aria-label="Recommendations">
            <p className={styles.eyebrow}>For you</p>
            <h2>Recommendations</h2>
            {recommendationStatus === 'loading' || recommendationStatus === 'preparing' ? (
              <p>Recommendations are being prepared...</p>
            ) : recommendationStatus === 'empty' ? (
              <p>Add movies to My List to receive personalized recommendations.</p>
            ) : recommendationStatus === 'ready' ? (
              <MovieGrid movies={personalizedRecommendations} onSelect={setSelectedMovie} />
            ) : recommendationStatus === 'error' ? (
              <p>Recommendations are currently unavailable.</p>
            ) : null}
          </section>
        )}
        <section className={styles.intro}>
          <p className={styles.eyebrow}>Discover your next favourite</p>
          <h2>{activeMovieSearch ? `Results for "${activeMovieSearch}"` : 'Popular movies'}</h2>
          <form className={styles.movieSearch} role="search" onSubmit={handleMovieSearch}>
            <label className={styles.movieSearchLabel} htmlFor="movie-search">Search movies</label>
            <div className={styles.movieSearchControls}>
              <input
                id="movie-search"
                className={styles.movieSearchInput}
                type="search"
                value={movieSearch}
                onChange={(event) => setMovieSearch(event.target.value)}
                placeholder="Search by movie title"
                autoComplete="off"
              />
              <button className={styles.movieSearchButton} type="submit" disabled={!movieSearch.trim()}>
                Search
              </button>
              {(movieSearch || activeMovieSearch) && (
                <button className={styles.clearMovieSearchButton} type="button" onClick={clearMovieSearch}>
                  Clear
                </button>
              )}
            </div>
          </form>
          <GenreFilter
            genres={genres}
            selectedGenreId={selectedGenreId}
            onSelect={setSelectedGenreId}
          />
        </section>

        {isLoadingMovies ? (
          <p>Loading movies...</p>
        ) : moviesLoadFailed ? (
          <div className={styles.movieLoadError} role="status">
            <p>Movies are temporarily unavailable. Check your connection and try again.</p>
            <button
              type="button"
              onClick={() => setMovieRetryCount((count) => count + 1)}
            >
              Try again
            </button>
          </div>
        ) : (
          <>
            {movies.length === 0 && activeMovieSearch ? (
              <p className={styles.noMoviesFound}>No movies found for "{activeMovieSearch}".</p>
            ) : (
              <MovieGrid movies={movies} onSelect={setSelectedMovie} />
            )}
            <div ref={loadMoreRef} className={styles.loadMoreSentinel} aria-hidden="true" />
            {isLoadingMore && <p>Loading more movies...</p>}
            {loadMoreFailed && (
              <p className={styles.movieLoadError} role="status">
                Could not load more movies. <button type="button" onClick={loadMoreMovies}>Try again</button>
              </p>
            )}
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
        {isChatOpen && (
          <RecommendationChat
            onClose={() => setIsChatOpen(false)}
            onToggleList={(movie) => handleToggleList(movie)}
            isAddingToList={isAddingToList}
            myListMovieIds={myListMovieIds}
          />
        )}
      </main>
    </div>
  )
}
