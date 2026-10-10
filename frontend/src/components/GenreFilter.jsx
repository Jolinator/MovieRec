import React, { useState } from 'react'
import styles from './GenreFilter.module.css'

export default function GenreFilter({ genres, selectedGenreId, onSelect }) {
  const [isOpen, setIsOpen] = useState(false)
  const selectedGenre = genres.find((genre) => genre.id === selectedGenreId)
  const filterLabel = selectedGenre ? `Filter : ${selectedGenre.name}` : 'Filter'

  function handleSelect(genreId) {
    onSelect(genreId)
    setIsOpen(false)
  }

  return (
    <div className={styles.filter}>
      <button
        type="button"
        className={styles.toggle}
        aria-expanded={isOpen}
        aria-controls="genre-filter-options"
        onClick={() => setIsOpen((isCurrentlyOpen) => !isCurrentlyOpen)}
      >
        <span aria-hidden="true">☰</span>
        {filterLabel}
      </button>
      {isOpen && (
        <div id="genre-filter-options" className={styles.options} aria-label="Filter movies by genre">
          <button
            type="button"
            className={selectedGenreId === null ? styles.active : ''}
            onClick={() => handleSelect(null)}
          >
            All
          </button>
          {genres.map((genre) => (
            <button
              key={genre.id}
              type="button"
              className={selectedGenreId === genre.id ? styles.active : ''}
              onClick={() => handleSelect(genre.id)}
            >
              {genre.name}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
