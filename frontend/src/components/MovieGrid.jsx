import React from 'react'
import MovieDisplay from './MovieDisplay'
import styles from './MovieGrid.module.css'

export default function MovieGrid({ movies, onSelect }) {
  if (movies.length === 0) {
    return <p className={styles.empty}>No movies available.</p>
  }

  return (
    <section className={styles.grid} aria-label="Movies">
      {movies.map((movie) => (
        <MovieDisplay key={movie.tmdbId} movie={movie} onSelect={onSelect} />
      ))}
    </section>
  )
}
