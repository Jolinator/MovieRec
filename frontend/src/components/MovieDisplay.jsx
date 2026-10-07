import React from 'react'
import styles from './MovieDisplay.module.css'

const IMAGE_BASE_URL = 'https://image.tmdb.org/t/p/w500'

export default function MovieDisplay({ movie }) {
  const posterUrl = movie.posterPath ? `${IMAGE_BASE_URL}${movie.posterPath}` : null

  return (
    <article className={styles.card}>
      {posterUrl ? (
        <img className={styles.poster} src={posterUrl} alt={`${movie.title} poster`} />
      ) : (
        <div className={styles.posterPlaceholder}>No poster</div>
      )}
      <div className={styles.content}>
        <h2>{movie.title}</h2>
        <p className={styles.rating}>Rating: {Number(movie.averageRating).toFixed(1)}</p>
        <p className={styles.synopsis}>{movie.synopsis || 'No synopsis available.'}</p>
      </div>
    </article>
  )
}
