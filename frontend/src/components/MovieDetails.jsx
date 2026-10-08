import React, { useEffect } from 'react'
import styles from './MovieDetails.module.css'

const IMAGE_BASE_URL = 'https://image.tmdb.org/t/p/w780'

export default function MovieDetails({ movie, onClose }) {
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
          <p className={styles.synopsis}>{movie.synopsis || 'No synopsis available.'}</p>
        </div>
      </section>
    </div>
  )
}
