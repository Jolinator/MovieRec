import React, { useEffect, useState } from 'react'
import ConfirmationPopup from './ConfirmationPopup'
import styles from './MovieDetails.module.css'

const IMAGE_BASE_URL = 'https://image.tmdb.org/t/p/w780'

export default function MovieDetails({
  movie,
  onClose,
  onToggleList,
  isAddingToList = false,
  isInList = false,
  showListActions = true,
}) {
  const [confirmationType, setConfirmationType] = useState('')

  useEffect(() => {
    setConfirmationType('')
  }, [movie.tmdbId])

  async function handleAddToList() {
    const wasAdded = await onToggleList()
    setConfirmationType(wasAdded ? 'added' : 'removed')
  }

  useEffect(() => {
    function handleKeyDown(event) {
      if (event.key === 'Escape') {
        onClose()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  const backdropUrl = movie.backdropPath
    ? `${IMAGE_BASE_URL}${movie.backdropPath}`
    : null

  return (
    <>
      <div
        className={styles.overlay}
        role="dialog"
        aria-modal="true"
        aria-labelledby="movie-details-title"
        onClick={onClose}
      >
        <section className={styles.dialog} onClick={(event) => event.stopPropagation()}>
          <button type="button" className={styles.close} onClick={onClose} aria-label="Close movie details">
            ×
          </button>
          {backdropUrl && <img className={styles.backdrop} src={backdropUrl} alt="" />}
          <div className={styles.body}>
            <p className={styles.eyebrow}>Movie details</p>
            <h2 id="movie-details-title">{movie.title}</h2>
            <p className={styles.rating}>Rating: {Number(movie.averageRating).toFixed(1)}</p>
            <p className={styles.releaseDate}>
              Release date: {movie.releaseDate || 'Unknown'}
            </p>
            <p className={styles.synopsis}>{movie.synopsis || 'No synopsis available.'}</p>
            {showListActions && onToggleList && (
              <button
                type="button"
                className={styles.addToList}
                onClick={handleAddToList}
                disabled={isAddingToList}
                aria-label={isInList ? `Remove ${movie.title} from My List` : `Add ${movie.title} to My List`}
                title={isInList ? 'Remove from My List' : 'Add to My List'}
              >
                {isInList ? '−' : '+'}
              </button>
            )}
          </div>
        </section>
      </div>
      {confirmationType && (
        <ConfirmationPopup
          key={confirmationType}
          message={confirmationType === 'added' ? 'Added to My List' : 'Removed from My List'}
          onDismiss={() => setConfirmationType('')}
        />
      )}
    </>
  )
}
