import React, { useState } from 'react'
import MovieDetails from './MovieDetails'
import { chatRecommendations } from '../services/api'
import styles from './RecommendationChat.module.css'

function cleanReply(reply) {
  return reply
    .replace(/\*/g, '')
    .replace(/\[ID:\d+\]/g, '')
    .replace(/(?:the )?database (?:id|identifier)(?: for this movie)?\s*:\s*\d*/gi, '')
    .replace(/\s+([.,])/g, '$1')
    .replace(/[ \t]+\n/g, '\n')
    .trim()
}

export default function RecommendationChat({
  onClose,
  onToggleList,
  isAddingToList,
  myListMovieIds,
}) {
  const [message, setMessage] = useState('')
  const [reply, setReply] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [movies, setMovies] = useState([])
  const [selectedMovie, setSelectedMovie] = useState(null)
  async function submit(event) {
    event.preventDefault(); if (!message.trim() || loading) return
    setLoading(true); setError(''); setReply(''); setMovies([])
    try {
      const result = await chatRecommendations(message.trim())
      setReply(result.message)
      setMovies((result.movies ?? []).slice(0, 5))
      setMessage('')
    }
    catch (requestError) { setError(requestError instanceof Error ? requestError.message : 'Chat unavailable.') }
    finally { setLoading(false) }
  }
  return <section className={styles.panel} aria-label="Recommendation Chat">
    <div className={styles.heading}><h2>Recommendation Chat</h2><button type="button" onClick={onClose}>×</button></div>
    <p>Tell me what you feel like watching.</p>
    {movies.length > 0 && (
      <div className={styles.movieLinks} aria-label="Recommended movies">
        {movies.map((movie) => (
          <button
            type="button"
            className={styles.movieLink}
            key={movie.tmdbId}
            onClick={() => setSelectedMovie(movie)}
          >
            {movie.title}
          </button>
        ))}
      </div>
    )}
    {reply && <div className={styles.reply}>{cleanReply(reply)}</div>}
    {error && <div className={styles.error}>{error}</div>}
    <form onSubmit={submit}><textarea value={message} onChange={(event) => setMessage(event.target.value)} placeholder="e.g. a clever sci-fi thriller" maxLength={1000} required /><button type="submit" disabled={loading}>{loading ? 'Thinking…' : 'Recommend'}</button></form>
    {selectedMovie && (
      <MovieDetails
        movie={selectedMovie}
        onClose={() => setSelectedMovie(null)}
        onToggleList={() => onToggleList(selectedMovie)}
        isAddingToList={isAddingToList}
        isInList={myListMovieIds.has(selectedMovie.tmdbId)}
      />
    )}
  </section>
}
